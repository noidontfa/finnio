package abr

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type indexEntry struct {
	Name     string
	Duration float64
}

func contiguousSegments(root string, vs []Variant) []string {
	var names []string
	for i := 0; ; i++ {
		name := fmt.Sprintf("seg_%05d.ts", i)
		for _, v := range vs {
			if _, err := os.Stat(filepath.Join(root, v.Label, name)); err != nil {
				return names
			}
		}
		names = append(names, name)
	}
}

func mediaPlaylist(entries []indexEntry, isDone bool) string {
	var max float64
	for _, e := range entries {
		if e.Duration > max {
			max = e.Duration
		}
	}
	td := int(math.Ceil(max))
	if td < 1 {
		td = 1
	}

	var b strings.Builder
	fmt.Fprintf(&b, "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:%d\n#EXT-X-MEDIA-SEQUENCE:0\n", td)
	for _, e := range entries {
		fmt.Fprintf(&b, "#EXTINF:%.3f,\n%s\n", e.Duration, e.Name)
	}
	if isDone {
		fmt.Fprintf(&b, "#EXT-X-ENDLIST")
	}
	return b.String()
}

func masterPlaylist(vs []Variant) string {
	var b strings.Builder
	b.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-INDEPENDENT-SEGMENTS\n")
	for _, v := range vs {
		fmt.Fprintf(&b, "#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d\n%s/index.m3u8\n",
			v.MaxRate+v.Bitrate, v.Width, v.Height, v.Label)
	}
	return b.String()
}

func writeAtomic(path, content string) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (h *Handler) segmentDuration(path string) (float64, error) {
	if h.durationOf != nil {
		return h.durationOf(path)
	}
	if h.ffm == nil {
		return 0, fmt.Errorf("ffprobe runner is nil")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := h.ffm.RunFFProbeContext(ctx,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "csv=p=0",
		path,
	)
	if err != nil {
		return 0, err
	}
	d, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0, fmt.Errorf("parse duration %q: %w", out, err)
	}
	return d, nil
}
