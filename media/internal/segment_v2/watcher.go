package segmentv2

import (
	"errors"
	"strings"

	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	fs *fsnotify.Watcher
}

func NewWatcher() (*Watcher, error) {
	fs, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &Watcher{fs: fs}, nil
}

func (w *Watcher) Watch(done <-chan bool, folder string, hooks Hooks) error {
	err := w.fs.Add(folder)
	if err != nil {
		return err
	}
	defer w.fs.Close()
	for {
		select {
		case <-done:
			return nil
		case event, ok := <-w.fs.Events:
			if !ok {
				return errors.New("watcher closed")
			}
			err := w.handleEvent(event, hooks)
			if err != nil {
				return err
			}
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

func (w *Watcher) handleEvent(event fsnotify.Event, hooks Hooks) error {
	path := event.Name

	switch event.Op {
	case fsnotify.Create:
		// fmt.Println("create", path)
		if !strings.HasSuffix(path, ".ts") && !strings.HasSuffix(path, ".m3u8") {
			return nil
		}
		if strings.HasSuffix(path, ".ts") && hooks.OnSegmentCreated != nil {
			return hooks.OnSegmentCreated(path)
		}
		if strings.HasSuffix(path, ".m3u8") && hooks.OnMasterUpdated != nil {
			return hooks.OnMasterUpdated(path)
		}
		return nil
	case fsnotify.Remove:
		// fmt.Println("remove", path)
		return nil
	case fsnotify.Write:
		// fmt.Println("write", path)
		return nil
	case fsnotify.Rename:
		// fmt.Println("rename", path)
		// if !strings.HasSuffix(path, ".tmp") {
		// 	return nil
		// }
		// return callback(strings.TrimSuffix(path, ".tmp"))
		return nil
	}
	return nil
}
