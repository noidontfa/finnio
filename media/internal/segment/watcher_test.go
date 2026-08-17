package segment

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestWatchDoesNotPublishUntilNextSegmentStarts(t *testing.T) {
	folder := t.TempDir()

	var mu sync.Mutex
	seen := map[string]int{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w := NewWatcher()
	done := make(chan error, 1)
	go func() {
		done <- w.Watch(ctx, folder, func(path string) error {
			mu.Lock()
			seen[filepath.Base(path)]++
			mu.Unlock()
			return nil
		})
	}()

	time.Sleep(100 * time.Millisecond)

	if err := os.WriteFile(filepath.Join(folder, "seg_00000.ts"), []byte("open"), 0o644); err != nil {
		t.Fatalf("write 00000: %v", err)
	}
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	if len(seen) != 0 {
		mu.Unlock()
		t.Fatalf("open segment was published before the next one started: %v", seen)
	}
	mu.Unlock()

	if err := os.WriteFile(filepath.Join(folder, "seg_00001.ts"), []byte("open"), 0o644); err != nil {
		t.Fatalf("write 00001: %v", err)
	}
	waitSeen(t, &mu, seen, 1)

	mu.Lock()
	defer mu.Unlock()
	if seen["seg_00000.ts"] != 1 {
		t.Errorf("seg_00000.ts handled %d times, want 1", seen["seg_00000.ts"])
	}
	if _, ok := seen["seg_00001.ts"]; ok {
		t.Error("seg_00001.ts is still open and must not be published yet")
	}
}

func TestWatchCallsOnReadyOncePerSegment(t *testing.T) {
	folder := t.TempDir()

	var mu sync.Mutex
	seen := map[string]int{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w := NewWatcher()
	done := make(chan error, 1)
	go func() {
		done <- w.Watch(ctx, folder, func(path string) error {
			mu.Lock()
			seen[filepath.Base(path)]++
			mu.Unlock()
			return nil
		})
	}()

	// Give fsnotify time to register the folder before writing.
	time.Sleep(100 * time.Millisecond)

	for _, name := range []string{"seg_00000.ts", "seg_00001.ts"} {
		if err := os.WriteFile(filepath.Join(folder, name), []byte("payload"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	// Non-segment files must be ignored.
	if err := os.WriteFile(filepath.Join(folder, "playlist.m3u8.tmp"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write tmp: %v", err)
	}

	waitSeen(t, &mu, seen, 1)

	// Extra writes to an already-handled segment must not re-trigger onReady.
	if err := os.WriteFile(filepath.Join(folder, "seg_00000.ts"), []byte("payload-more"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	time.Sleep(300 * time.Millisecond)

	w.Stop()
	if err := <-done; err != nil {
		t.Fatalf("Watch: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if seen["seg_00000.ts"] != 1 {
		t.Errorf("seg_00000.ts handled %d times, want 1", seen["seg_00000.ts"])
	}
	if seen["seg_00001.ts"] != 1 {
		t.Errorf("seg_00001.ts handled %d times, want 1 (published on Stop)", seen["seg_00001.ts"])
	}
	if _, ok := seen["playlist.m3u8.tmp"]; ok {
		t.Error("tmp file should not be handled")
	}
}

func TestStopPublishesLastOpenSegment(t *testing.T) {
	folder := t.TempDir()

	var mu sync.Mutex
	seen := map[string]int{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	entered := make(chan struct{})
	release := make(chan struct{})

	w := NewWatcher()
	done := make(chan error, 1)
	go func() {
		done <- w.Watch(ctx, folder, func(path string) error {
			mu.Lock()
			seen[filepath.Base(path)]++
			mu.Unlock()
			close(entered)
			<-release
			return nil
		})
	}()

	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(folder, "seg_00000.ts"), []byte("payload"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	w.Stop()

	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("onReady was never called for the last segment")
	}

	select {
	case <-done:
		t.Fatal("Watch returned before in-flight segment finished")
	case <-time.After(200 * time.Millisecond):
	}

	close(release)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Watch returned %v, want nil after Stop", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Watch did not return after draining")
	}

	mu.Lock()
	defer mu.Unlock()
	if seen["seg_00000.ts"] != 1 {
		t.Errorf("seg_00000.ts handled %d times, want 1", seen["seg_00000.ts"])
	}
}

func TestCatchUpSkipsSegmentsWatchAlreadyHandled(t *testing.T) {
	folder := t.TempDir()

	var mu sync.Mutex
	seen := map[string]int{}
	onReady := func(path string) error {
		mu.Lock()
		seen[filepath.Base(path)]++
		mu.Unlock()
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w := NewWatcher()
	done := make(chan error, 1)
	go func() { done <- w.Watch(ctx, folder, onReady) }()

	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(folder, "seg_00000.ts"), []byte("payload"), 0o644); err != nil {
		t.Fatalf("write watched: %v", err)
	}
	time.Sleep(200 * time.Millisecond)

	// A segment that fsnotify never reported, mimicking one written after ffmpeg exits.
	w.Stop()
	if err := <-done; err != nil {
		t.Fatalf("Watch: %v", err)
	}
	if err := os.WriteFile(filepath.Join(folder, "seg_00001.ts"), []byte("payload"), 0o644); err != nil {
		t.Fatalf("write missed: %v", err)
	}

	if err := w.CatchUp(folder, onReady); err != nil {
		t.Fatalf("CatchUp: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if seen["seg_00000.ts"] != 1 {
		t.Errorf("seg_00000.ts handled %d times, want 1 (CatchUp should skip it)", seen["seg_00000.ts"])
	}
	if seen["seg_00001.ts"] != 1 {
		t.Errorf("seg_00001.ts handled %d times, want 1 (CatchUp should pick it up)", seen["seg_00001.ts"])
	}
}

func TestClaimEvictsOldestAtCap(t *testing.T) {
	t.Parallel()

	w := NewWatcher()
	defer w.fs.Close()

	for i := 0; i < handledCap; i++ {
		path := filepath.Join("/tmp", "seg_"+strconv.Itoa(i)+".ts")
		if !w.claim(path) {
			t.Fatalf("claim %s: want true for first insert", path)
		}
	}
	if got := len(w.handled); got != handledCap {
		t.Fatalf("len(handled)=%d, want %d before overflow", got, handledCap)
	}

	overflow := filepath.Join("/tmp", "seg_overflow.ts")
	if !w.claim(overflow) {
		t.Fatal("claim overflow: want true after evicting oldest")
	}
	if got := len(w.handled); got != handledCap {
		t.Fatalf("len(handled)=%d, want %d after evicting oldest", got, handledCap)
	}
	kept := filepath.Join("/tmp", "seg_1.ts")
	if w.claim(kept) {
		t.Fatal("seg_1.ts should still be claimed after evicting only the oldest")
	}
	old := filepath.Join("/tmp", "seg_0.ts")
	if !w.claim(old) {
		t.Fatal("re-claim of evicted oldest path should succeed")
	}
}

func waitSeen(t *testing.T, mu *sync.Mutex, seen map[string]int, want int) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		mu.Lock()
		n := 0
		for _, c := range seen {
			n += c
		}
		got := fmt.Sprintf("%v", seen)
		mu.Unlock()
		if n == want {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for %d onReady calls, got %s", want, got)
		case <-time.After(20 * time.Millisecond):
		}
	}
}
