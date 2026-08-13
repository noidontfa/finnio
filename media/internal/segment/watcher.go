package segment

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches a folder for new .ts segment files.
// handled ensures each path is processed at most once (Watch + CatchUp share it).
type Watcher struct {
	fs      *fsnotify.Watcher
	mu      sync.Mutex
	handled map[string]bool

	inFlight sync.WaitGroup
	stop     chan struct{}
	stopOnce sync.Once
}

func NewWatcher() *Watcher {
	fs, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	return &Watcher{
		fs:      fs,
		handled: make(map[string]bool),
		stop:    make(chan struct{}),
	}
}

// Stop asks Watch to shut down gracefully: it stops accepting new events but
// lets segments already being processed finish. Safe to call more than once.
func (w *Watcher) Stop() {
	fmt.Println("here stop")
	w.stopOnce.Do(func() { close(w.stop) })
}

// watchResult reports the outcome of processing one segment file.
type watchResult struct {
	path string
	err  error
}

// Watch calls onReady once per new .ts file. It blocks until ctx is cancelled
// or Stop is called; Stop drains segments that are still being processed.
func (w *Watcher) Watch(ctx context.Context, folder string, onReady func(path string) error) error {
	if err := w.fs.Add(folder); err != nil {
		return err
	}
	defer w.fs.Close()

	results := make(chan watchResult)

	ctxCancel, ctxCancelFunc := context.WithCancel(ctx)
	defer ctxCancelFunc()

	for {
		select {
		case <-ctxCancel.Done():
			fmt.Println("here ctx cancel done")
			w.drain(results, onReady)
			return ctx.Err()

		case <-w.stop:
			fmt.Println("here stop drain Watch")
			w.drain(results, onReady)
			return nil

		case event, ok := <-w.fs.Events:
			if !ok {
				return errors.New("watcher closed")
			}
			w.handleEvent(ctxCancel, event, results)

		case result := <-results:
			if result.err != nil {
				return result.err
			}

			if onReady != nil {
				err := onReady(result.path)
				if err != nil {
					return err
				}
			}
			report(result)

		case err, ok := <-w.fs.Errors:
			if !ok {
				return errors.New("watcher errors closed")
			}
			if err != nil {
				return err
			}

		}
	}
}

func (w *Watcher) handleEvent(ctx context.Context, event fsnotify.Event, results chan<- watchResult) {
	path := event.Name
	// ffmpeg writes the playlist via *.tmp then renames; only final .ts segments matter.
	if strings.HasSuffix(path, ".tmp") || !strings.HasSuffix(path, ".ts") {
		return
	}

	if !w.claim(path) {
		return
	}

	w.inFlight.Add(1)
	go func(path string) {
		defer w.inFlight.Done()

		err := waitUntilWritten(ctx, path)
		// Keep the claim on success so later events (and CatchUp) skip this
		// path; release it on failure so it can be retried.
		if err != nil {
			w.unclaim(path)
		}
		results <- watchResult{path: path, err: err}
	}(path)
}

// drain keeps receiving results until every in-flight segment goroutine has
// finished. Workers send on an unbuffered channel, so once they are all done
// there is nothing left to receive.
func (w *Watcher) drain(results <-chan watchResult, onReady func(path string) error) {
	done := make(chan struct{})
	go func() {
		w.inFlight.Wait()
		close(done)
	}()

	for {
		select {
		case result := <-results:
			if onReady != nil && result.err == nil {
				if err := onReady(result.path); err != nil {
					return
				}
			}
			report(result)
		case <-done:
			return
		}
	}
}

func report(result watchResult) {
	if result.err != nil {
		log.Printf("failed processing %s: %v", result.path, result.err)
		return
	}
	// log.Println("segment ready:", result.path)
}

// CatchUp calls onReady for .ts files on disk that Watch never saw
// (ffmpeg can finish before fsnotify delivers Create events).
func (w *Watcher) CatchUp(folder string, onReady func(path string) error) error {
	matches, err := filepath.Glob(filepath.Join(folder, "*.ts"))
	if err != nil {
		return err
	}
	for _, path := range matches {
		if !w.claim(path) {
			continue
		}
		if onReady == nil {
			continue
		}
		if err := onReady(path); err != nil {
			return err
		}
	}
	return nil
}

const handledCap = 2

// claim marks path as handled. Returns false if it was already claimed.
// When the map reaches handledCap entries, it is cleared so long-lived
// RTMP sessions do not retain every segment path forever.
func (w *Watcher) claim(path string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.handled[path] {
		return false
	}
	if len(w.handled) >= handledCap {
		w.handled = make(map[string]bool, handledCap)
	}
	w.handled[path] = true
	return true
}

func (w *Watcher) unclaim(path string) {
	w.mu.Lock()
	delete(w.handled, path)
	w.mu.Unlock()
}

func waitUntilWritten(ctx context.Context, file string) error {
	const timeout = 500 * time.Millisecond
	const poll = 50 * time.Millisecond

	deadline := time.Now().Add(timeout)
	var lastSize int64 = -1

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for file %s to finish writing", file)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		info, err := os.Stat(file)
		if err != nil {
			return err
		}
		if info.Size() == lastSize {
			return nil
		}
		lastSize = info.Size()

		timer := time.NewTimer(poll)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
