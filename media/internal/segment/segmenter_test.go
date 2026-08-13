package segment

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSegmentArgsLiveAddsListFlags(t *testing.T) {
	t.Parallel()

	live := segmentArgs(SourceOptions{
		InputFile:    "rtmp://127.0.0.1:1935/key",
		OutputFolder: "/tmp/out",
		HlsTime:      "2",
		Live:         true,
	})
	joined := strings.Join(live, " ")
	if !strings.Contains(joined, "-segment_list_flags +live") {
		t.Fatalf("live args missing +live flags: %v", live)
	}
	wantOut := filepath.Join("/tmp/out", "seg_%05d.ts")
	if live[len(live)-1] != wantOut {
		t.Fatalf("output pattern = %q, want %q", live[len(live)-1], wantOut)
	}

	file := segmentArgs(SourceOptions{
		InputFile:    "in.mp4",
		OutputFolder: "/tmp/out",
		HlsTime:      "2",
		Live:         false,
	})
	joined = strings.Join(file, " ")
	if strings.Contains(joined, "-segment_list_flags") {
		t.Fatalf("file mode should not set segment_list_flags: %v", file)
	}
}
