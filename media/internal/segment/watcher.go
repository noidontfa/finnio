package segment

import (
	"context"
	"errors"
	"log"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches a folder for new .ts segment files.
// handled ensures each path is processed at most once (Watch + CatchUp share it).
//
// ffmpeg's segment muxer creates the next file when it closes the current one,
// so a path is only published after the following segment appears (or on Stop
// for the last open file).
type Watcher struct {
	fs      *fsnotify.Watcher
	mu      sync.Mutex
	handled map[string]bool
	order   []string
	current string

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
	w.stopOnce.Do(func() { close(w.stop) })
}

// watchResult reports the outcome of processing one segment file.
type watchResult struct {
	path string
	err  error
}

// Watch calls onReady once per completed .ts file. It blocks until ctx is
// cancelled or Stop is called; Stop publishes the last open segment, then
// drains in-flight callbacks.
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
			w.drain(results, onReady)
			return ctx.Err()

		case <-w.stop:
			w.drain(results, onReady)
			return nil

		case event, ok := <-w.fs.Events:
			if !ok {
				return errors.New("watcher closed")
			}
			w.handleEvent(event, results)

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

func (w *Watcher) handleEvent(event fsnotify.Event, results chan<- watchResult) {
	path := event.Name
	// ffmpeg writes the playlist via *.tmp then renames; only final .ts segments matter.
	if strings.HasSuffix(path, ".tmp") || !strings.HasSuffix(path, ".ts") {
		return
	}

	if !w.claim(path) {
		return
	}

	completed := w.rotate(path)
	if completed == "" {
		return
	}
	w.emit(completed, results)
}

func (w *Watcher) rotate(path string) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	prev := w.current
	w.current = path
	return prev
}

func (w *Watcher) takeCurrent() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	path := w.current
	w.current = ""
	return path
}

func (w *Watcher) emit(path string, results chan<- watchResult) {
	w.inFlight.Add(1)
	go func() {
		defer w.inFlight.Done()
		results <- watchResult{path: path}
	}()
}

// drain keeps receiving results until every in-flight segment goroutine has
// finished, then publishes the last open segment (ffmpeg has closed it).
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
			last := w.takeCurrent()
			if last != "" && onReady != nil {
				_ = onReady(last)
			}
			return
		}
	}
}

func report(result watchResult) {
	if result.err != nil {
		log.Printf("failed processing %s: %v", result.path, result.err)
		return
	}
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

const handledCap = 64

// claim marks path as handled. Returns false if it was already claimed.
// When the map reaches handledCap entries, the oldest path is evicted so
// long-lived RTMP sessions do not retain every segment path forever.
func (w *Watcher) claim(path string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.handled[path] {
		return false
	}
	if len(w.order) >= handledCap {
		old := w.order[0]
		w.order = w.order[1:]
		delete(w.handled, old)
	}
	w.handled[path] = true
	w.order = append(w.order, path)
	return true
}
