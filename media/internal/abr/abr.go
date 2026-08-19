package abr

import (
	"context"
	"fmt"
	"media/internal/ffmpeg"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

type Variant struct {
	Label   string `json:"label"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	MaxRate int    `json:"max_rate"`
	Bitrate int    `json:"bitrate"`
	Enabled bool   `json:"enabled"`
}

type ABR struct {
	js  nats.JetStreamContext
	nc  *nats.Conn
	ffm *ffmpeg.Runner
}

type Request struct {
	SourceID    string    `json:"source_id"`
	SourceType  string    `json:"source_type"`
	SegmentFile string    `json:"segment_file"`
	Timestamp   time.Time `json:"timestamp"`
}

var (
	ARB_STREAM           = "ABR"
	ARB_SUBJECT          = "abr.*"
	ARB_WORKER           = "abr-worker"
	ARB_SUBJECT_REQUESTS = "abr.requests"

	variants = []Variant{
		{Label: "360p", Width: 480, Height: 360, MaxRate: 600000, Bitrate: 500000, Enabled: true},
		{Label: "480p", Width: 640, Height: 480, MaxRate: 1500000, Bitrate: 1000000, Enabled: true},
		{Label: "720p", Width: 1280, Height: 720, MaxRate: 3000000, Bitrate: 2000000, Enabled: true},
		{Label: "1080p", Width: 1920, Height: 1080, MaxRate: 5000000, Bitrate: 2000000, Enabled: true},
	}

	TS_FILE    = "ts_file"
	INDEX_FILE = "index_file"
)

func usableAudio(probeOut []byte) bool {
	for _, line := range strings.Split(string(probeOut), "\n") {
		parts := strings.Split(strings.TrimSpace(line), ",")
		if len(parts) != 2 {
			continue
		}
		rate, errRate := strconv.Atoi(parts[0])
		ch, errCh := strconv.Atoi(parts[1])
		if errRate != nil || errCh != nil {
			continue
		}
		return rate > 0 && ch > 0
	}
	return false
}

func enabledVariants(vs []Variant) []Variant {
	out := make([]Variant, 0, len(vs))
	for _, v := range vs {
		if v.Enabled {
			out = append(out, v)
		}
	}
	return out
}

// encodeTmpSuffix keeps in-progress encodes out of contiguousSegments, which
// only matches the final seg_%05d.ts names.
const encodeTmpSuffix = ".tmp"

func variantPath(outputFolder string, v Variant, name string) string {
	return filepath.Join(outputFolder, v.Label, name)
}

func variantTmpPath(outputFolder string, v Variant, name string) string {
	return variantPath(outputFolder, v, name) + encodeTmpSuffix
}

func encodeArgs(inputFile, outputFolder string, vs []Variant, hasAudio bool) []string {
	name := filepath.Base(inputFile)
	// -copyts keeps the source segment's timestamps instead of rebasing to zero,
	// so consecutive segments form one continuous timeline. Without it every
	// segment starts at the mpegts muxer's default 1.4s offset and players stall
	// after the first one.
	args := []string{
		"-hide_banner", "-y",
		"-copyts",
		"-i", inputFile,
		"-filter_complex", filterComplex(vs),
	}
	for _, v := range vs {
		args = append(args, "-map", fmt.Sprintf("[s%s]", v.Label))
		if hasAudio {
			args = append(args, "-map", "0:a")
		}
		args = append(args,
			"-c:v", "libx264", "-crf", "22", "-preset", "fast",
			"-maxrate", kb(v.MaxRate),
			"-bufsize", kb(v.MaxRate*2),
			"-x264-params", "keyint=90:min-keyint=90:scenecut=0",
		)
		if hasAudio {
			args = append(args,
				"-c:a", "aac", "-ar", "44100",
				"-b:a", kb(v.Bitrate),
			)
		} else {
			args = append(args, "-an")
		}
		args = append(args,
			"-f", "mpegts",
			"-muxdelay", "0", "-muxpreload", "0",
			variantTmpPath(outputFolder, v, name),
		)
	}
	return args
}

func filterComplex(vs []Variant) string {
	if len(vs) == 1 {
		v := vs[0]
		return fmt.Sprintf("[0:v]scale=%d:%d[s%s]", v.Width, v.Height, v.Label)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[0:v]split=%d", len(vs))
	for _, v := range vs {
		fmt.Fprintf(&b, "[v%s]", v.Label)
	}
	for _, v := range vs {
		fmt.Fprintf(&b, ";[v%s]scale=%d:%d[s%s]", v.Label, v.Width, v.Height, v.Label)
	}
	return b.String()
}

func kb(bps int) string {
	return fmt.Sprintf("%dk", bps/1000)
}

func retryOperation(ctx context.Context, maxRetries int, fn func() error) error {
	for i := 0; i < maxRetries; i++ {
		err := fn()
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		time.Sleep(time.Duration(i+1) * 50 * time.Millisecond)
	}
	return fmt.Errorf("operation failed after %d retries", maxRetries)
}
