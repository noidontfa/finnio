package abr

import (
	"context"
	"fmt"
	"media/internal/ffmpeg"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Handler struct {
	ffm        *ffmpeg.Runner
	indexMu    sync.Mutex
	durationOf func(path string) (float64, error)

	sourcesMu sync.Mutex
	sources   map[string]*sourceLock
}

type sourceLock struct {
	mu   sync.Mutex
	refs int
}

func NewHandler(ffm *ffmpeg.Runner) *Handler {
	return &Handler{ffm: ffm}
}

// lockSource serializes work for one source so a playlist rebuild never probes
// a segment another worker is still encoding. Different sources stay parallel.
// The returned func releases the lock and drops the entry once nobody holds it.
func (h *Handler) lockSource(sourceID string) func() {
	h.sourcesMu.Lock()
	if h.sources == nil {
		h.sources = make(map[string]*sourceLock)
	}
	lock := h.sources[sourceID]
	if lock == nil {
		lock = &sourceLock{}
		h.sources[sourceID] = lock
	}
	lock.refs++
	h.sourcesMu.Unlock()

	lock.mu.Lock()

	return func() {
		lock.mu.Unlock()
		h.sourcesMu.Lock()
		lock.refs--
		if lock.refs == 0 {
			delete(h.sources, sourceID)
		}
		h.sourcesMu.Unlock()
	}
}

func (h *Handler) Handle(request Request, outputFolder string) error {
	unlock := h.lockSource(request.SourceID)
	defer unlock()

	if request.SourceType != "video_done" {
		err := h.Encode(request.SegmentFile, outputFolder)
		if err != nil {
			return err
		}
	}

	err := h.UpdateIndexes(outputFolder, request.SourceType)
	if err != nil {
		return err
	}
	err = h.EnsureMaster(outputFolder)
	if err != nil {
		return err
	}
	return nil
}

func (h *Handler) Encode(inputFile, outputFolder string) error {
	vs := enabledVariants(variants)
	if len(vs) == 0 {
		return fmt.Errorf("no enabled variants")
	}
	for _, v := range vs {
		if err := os.MkdirAll(filepath.Join(outputFolder, v.Label), 0o755); err != nil {
			return err
		}
	}
	hasAudio := h.hasUsableAudio(inputFile)

	name := filepath.Base(inputFile)
	if err := h.ffm.Run(encodeArgs(inputFile, outputFolder, vs, hasAudio)...); err != nil {
		for _, v := range vs {
			os.Remove(variantTmpPath(outputFolder, v, name))
		}
		return err
	}

	for _, v := range vs {
		if err := os.Rename(variantTmpPath(outputFolder, v, name), variantPath(outputFolder, v, name)); err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) hasUsableAudio(inputFile string) bool {
	if h.ffm == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := h.ffm.RunFFProbeContext(ctx,
		"-v", "error",
		"-select_streams", "a:0",
		"-show_entries", "stream=sample_rate,channels",
		"-of", "csv=p=0",
		inputFile,
	)
	if err != nil {
		return false
	}
	return usableAudio(out)
}

func (h *Handler) EnsureMaster(outputFolder string) error {
	if err := os.MkdirAll(outputFolder, 0o755); err != nil {
		return err
	}

	masterFile := filepath.Join(outputFolder, "master.m3u8")
	if _, err := os.Stat(masterFile); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	vs := enabledVariants(variants)
	if len(vs) == 0 {
		return fmt.Errorf("no enabled variants")
	}
	return writeAtomic(masterFile, masterPlaylist(vs))
}

func (h *Handler) UpdateIndexes(outputFolder string, sourceType string) error {
	h.indexMu.Lock()
	defer h.indexMu.Unlock()

	vs := enabledVariants(variants)
	if len(vs) == 0 {
		return fmt.Errorf("no enabled variants")
	}

	names := contiguousSegments(outputFolder, vs)
	if len(names) == 0 {
		return nil
	}

	entries := make([]indexEntry, 0, len(names))
	for _, name := range names {
		dur, err := h.segmentDuration(filepath.Join(outputFolder, vs[0].Label, name))
		if err != nil {
			return err
		}
		entries = append(entries, indexEntry{Name: name, Duration: dur})
	}

	body := mediaPlaylist(entries, sourceType == "video_done")
	for _, v := range vs {
		if err := writeAtomic(filepath.Join(outputFolder, v.Label, "index.m3u8"), body); err != nil {
			return err
		}
	}
	return nil
}
