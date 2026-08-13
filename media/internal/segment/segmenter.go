package segment

import (
	"bytes"
	"context"
	"fmt"
	"media/internal/ffmpeg"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type SourceOptions struct {
	InputFile    string
	OutputFolder string
	HlsTime      string
	// Live adds HLS segment-list flags suited to an ongoing ingest (e.g. RTMP).
	Live bool
}

type Hooks struct {
	OnSegmentCreated func(segmentFile string) error
	OnSegmenterReady func(outputFolder string) error
}

type Segmenter struct {
	runner *ffmpeg.Runner
}

func NewSegmenter(runner *ffmpeg.Runner) *Segmenter {
	return &Segmenter{runner: runner}
}

func (s *Segmenter) Source(inputFile string, outputFolder string, hlsTime int) error {
	if err := os.MkdirAll(outputFolder, 0o755); err != nil {
		return err
	}
	return s.runner.Run(segmentArgs(SourceOptions{
		InputFile:    inputFile,
		OutputFolder: outputFolder,
		HlsTime:      strconv.Itoa(hlsTime),
	})...)
}

// SourceContext runs ffmpeg while watching for new segments.
//
// Flow:
//  1. Tell hooks the output folder is ready
//  2. Watch for new .ts files (calls OnSegmentCreated)
//  3. Run ffmpeg
//  4. When ffmpeg finishes, stop the watcher and CatchUp any missed files
func (s *Segmenter) SourceContext(ctx context.Context, options SourceOptions, hooks Hooks) error {
	if err := os.MkdirAll(options.OutputFolder, 0o755); err != nil {
		return err
	}

	watcher := NewWatcher()

	// 1) watch for segments
	watchDone := make(chan error, 1)
	go func() {
		// runnerCtx, runnerCancel := context.WithTimeout(ctx, 12*time.Second)
		// defer runnerCancel()
		// watchDone <- watcher.Watch(runnerCtx, options.OutputFolder, hooks.OnSegmentCreated)

		watchDone <- watcher.Watch(ctx, options.OutputFolder, hooks.OnSegmentCreated)
	}()

	// 2) run ffmpeg to completion (or until live input ends / ctx cancel)
	cmdErr := make(chan error, 1)
	go func() {
		// runnerCtx, runnerCancel := context.WithTimeout(ctx, 10*time.Second)
		// defer runnerCancel()
		// cmdErr <- s.runner.RunContext(runnerCtx, segmentArgs(options)...)

		cmdErr <- s.runner.RunContext(ctx, segmentArgs(options)...)

	}()

	if hooks.OnSegmenterReady != nil {
		if err := hooks.OnSegmenterReady(options.OutputFolder); err != nil {
			return err
		}
	}

	for {
		select {
		case err := <-cmdErr:
			if err != nil {
				return err
			}
			watcher.Stop()
		case err := <-watchDone:
			return err
		}
	}

}

func segmentArgs(options SourceOptions) []string {
	args := []string{
		"-i", options.InputFile,
		"-c", "copy",
		"-f", "segment",
		"-segment_time", options.HlsTime,
		"-segment_list", filepath.Join(options.OutputFolder, "playlist.m3u8"),
		"-segment_list_type", "m3u8",
	}
	if options.Live {
		args = append(args, "-segment_list_flags", "+live")
	}
	args = append(args,
		"-reset_timestamps", "1",
		filepath.Join(options.OutputFolder, "seg_%05d.ts"),
	)
	return args
}

func (s *Segmenter) hasVideoStream(ctx context.Context, url string) (bool, error) {
	out, err := s.runner.RunFFProbeContext(ctx, probeArgs(url)...)
	if err != nil {
		return false, fmt.Errorf("ffprobe failed: %w\n%s", err, out)
	}
	return strings.TrimSpace(string(bytes.TrimSpace(out))) == "video", nil
}

func probeArgs(url string) []string {
	return []string{
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_type",
		"-of", "csv=p=0",
		url,
	}
}

func (s *Segmenter) WaitForVideo(ctx context.Context, url string, interval time.Duration) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if interval <= 0 {
		interval = 250 * time.Millisecond
	}
	for {
		ok, err := s.hasVideoStream(ctx, url)
		if err == nil && ok {
			return nil
		}
		timer := time.NewTimer(interval)
		select {
		case <-timeoutCtx.Done():
			timer.Stop()
			return fmt.Errorf("timed out waiting for video on %s: %w", url, ctx.Err())
		case <-timer.C:
		}
	}
}
