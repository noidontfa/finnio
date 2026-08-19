package segmentv2

import (
	"bytes"
	"context"
	"fmt"
	"media/internal/ffmpeg"
	"os"
	"path/filepath"
	"shared/helper"
	"strings"
	"time"
)

type SegmenterReadyHook func(folder string) error
type MasterUpdatedHook func(masterFile string) error

type Options struct {
	InputFile    string
	OutputFolder string
	HlsTime      string
	// Live adds HLS segment-list flags suited to an ongoing ingest (e.g. RTMP).
	Live bool
}

type Hooks struct {
	OnSegmentCreated func(segmentFile string) error
	OnSegmenterReady SegmenterReadyHook
	OnSegmenterDone  func(outputFolder string) error
	OnMasterUpdated  MasterUpdatedHook
}

type Segmenter struct {
	ffm *ffmpeg.Runner
}

func NewSegmenter(ffm *ffmpeg.Runner) *Segmenter {
	return &Segmenter{ffm: ffm}
}

func (s *Segmenter) Segment(input string, output string) error {
	return nil
}

func (s *Segmenter) Source(options Options, hooks Hooks) error {
	if err := os.MkdirAll(options.OutputFolder, 0o755); err != nil {
		return err
	}

	done, stop := helper.Done()
	defer stop()

	fns := helper.Parallel(
		done,
		func() error {
			defer stop()
			return s.ffm.Run(segmentArgs(options)...)
		},
		func() error {
			defer stop()
			wat, err := NewWatcher()
			if err != nil {
				return err
			}
			return wat.Watch(done, options.OutputFolder, hooks)
		},
	)

	if hooks.OnSegmenterReady != nil {
		if err := hooks.OnSegmenterReady(options.OutputFolder); err != nil {
			return err
		}
	}

	for _, fn := range fns {
		if err := <-fn; err != nil {
			return err
		}
	}

	if hooks.OnSegmenterDone != nil {
		if err := hooks.OnSegmenterDone(options.OutputFolder); err != nil {
			return err
		}
	}

	return nil
}

// Timestamps are deliberately not reset per segment: the ABR stage encodes each
// segment separately, and players need one continuous timeline across the whole
// media playlist. Resetting makes every segment restart near zero, which stalls
// playback after the first segment.
func segmentArgs(options Options) []string {
	// ffmpeg -hide_banner -re \
	// -i tmp/sample/file_example_MP4_1920_18MG.mp4 \
	// -c copy \
	// -f hls \
	// -hls_time 2 \
	// -hls_list_size 0 \
	// -hls_flags temp_file \
	// -hls_segment_filename deployments/tmp/seg-tmp-test/seg_%05d.ts \
	// deployments/tmp/seg-tmp-test/playlist.m3u8

	args := []string{
		"-hide_banner", "-re",
		"-i", options.InputFile,
		"-c", "copy",
		"-f", "hls",
		"-hls_time", options.HlsTime,
		"-hls_list_size", "0",
		"-hls_flags", "temp_file",
		"-hls_segment_filename", filepath.Join(options.OutputFolder, "seg_%05d.ts"),
		filepath.Join(options.OutputFolder, "playlist.m3u8"),
	}

	return args
}

func (s *Segmenter) hasVideoStream(ctx context.Context, url string) (bool, error) {
	out, err := s.ffm.RunFFProbeContext(ctx, probeArgs(url)...)
	if err != nil {
		return false, fmt.Errorf("ffprobe failed: %w\n%s", err, out)
	}
	return strings.TrimSpace(string(bytes.TrimSpace(out))) == "video", nil
}

func probeArgs(url string) []string {
	return []string{
		"-v", "error",
		// Default analyze is 5s; RTMP often needs that full window, which is
		// longer than a tight probe timeout and makes WaitForVideo miss a live path.
		"-analyzeduration", "500000",
		"-probesize", "65536",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_type",
		"-of", "csv=p=0",
		url,
	}
}

// WaitForVideo polls ffprobe until the URL has a video stream or timeout.
func (s *Segmenter) WaitForVideo(url string, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	interval := 250 * time.Millisecond
	for {
		ok, err := s.hasVideoStream(timeoutCtx, url)
		if err == nil && ok {
			return nil
		}
		timer := time.NewTimer(interval)
		select {
		case <-timeoutCtx.Done():
			timer.Stop()
			if timeoutCtx.Err() == context.DeadlineExceeded {
				return fmt.Errorf("timed out waiting for video on %s", url)
			}
			return fmt.Errorf("waiting for video on %s: %w", url, timeoutCtx.Err())
		case <-timer.C:
		}
	}
}
