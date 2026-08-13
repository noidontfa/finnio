package segment

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"
)

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

	deadline := time.After(3 * time.Second)
	for {
		mu.Lock()
		count := len(seen)
		mu.Unlock()
		if count == 2 {
			break
		}
		select {
		case <-deadline:
			mu.Lock()
			t.Fatalf("timed out waiting for onReady, got %v", seen)
			mu.Unlock()
		case <-time.After(50 * time.Millisecond):
		}
	}

	// Extra writes to an already-handled segment must not re-trigger onReady.
	if err := os.WriteFile(filepath.Join(folder, "seg_00000.ts"), []byte("payload-more"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if seen["seg_00000.ts"] != 1 {
		t.Errorf("seg_00000.ts handled %d times, want 1", seen["seg_00000.ts"])
	}
	if seen["seg_00001.ts"] != 1 {
		t.Errorf("seg_00001.ts handled %d times, want 1", seen["seg_00001.ts"])
	}
	if _, ok := seen["playlist.m3u8.tmp"]; ok {
		t.Error("tmp file should not be handled")
	}
}

func TestStopDrainsInFlightSegments(t *testing.T) {
	folder := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	entered := make(chan struct{})
	release := make(chan struct{})

	w := NewWatcher()
	done := make(chan error, 1)
	go func() {
		done <- w.Watch(ctx, folder, func(path string) error {
			close(entered)
			<-release
			return nil
		})
	}()

	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(folder, "seg_00000.ts"), []byte("payload"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("onReady was never called")
	}

	w.Stop()

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
	time.Sleep(500 * time.Millisecond)

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

func TestClaimClearsHandledAtCap(t *testing.T) {
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
		t.Fatal("claim overflow: want true after clear")
	}
	if got := len(w.handled); got != 1 {
		t.Fatalf("len(handled)=%d, want 1 after clearing at cap", got)
	}
	// Previously capped paths were dropped; claiming one again should succeed.
	old := filepath.Join("/tmp", "seg_0.ts")
	if !w.claim(old) {
		t.Fatal("re-claim of cleared path should succeed")
	}
}
