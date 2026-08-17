package abr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContiguousSegmentsStopsAtHole(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	vs := enabledVariants(variants)
	writeSegs(t, root, vs, "seg_00000.ts", "seg_00001.ts")
	// 00003 exists but 00002 does not — prefix must stop after 00001.
	writeSegs(t, root, vs, "seg_00003.ts")

	got := contiguousSegments(root, vs)
	want := []string{"seg_00000.ts", "seg_00001.ts"}
	if len(got) != len(want) {
		t.Fatalf("contiguousSegments = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("contiguousSegments = %v, want %v", got, want)
		}
	}
}

func TestContiguousSegmentsEmptyWhenZeroMissing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	vs := enabledVariants(variants)
	writeSegs(t, root, vs, "seg_00001.ts")

	if got := contiguousSegments(root, vs); len(got) != 0 {
		t.Fatalf("contiguousSegments = %v, want empty", got)
	}
}

func TestMediaPlaylistHeaderAndOrder(t *testing.T) {
	t.Parallel()

	body := mediaPlaylist([]indexEntry{
		{Name: "seg_00000.ts", Duration: 4.732},
		{Name: "seg_00001.ts", Duration: 3.0},
	}, false)

	for _, want := range []string{
		"#EXTM3U\n",
		"#EXT-X-VERSION:3\n",
		"#EXT-X-TARGETDURATION:5\n",
		"#EXT-X-MEDIA-SEQUENCE:0\n",
		"#EXTINF:4.732,\nseg_00000.ts\n",
		"#EXTINF:3.000,\nseg_00001.ts\n",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("playlist missing %q\n%s", want, body)
		}
	}
	if strings.Contains(body, "#EXT-X-ENDLIST") {
		t.Fatal("live playlist must not include ENDLIST")
	}
}

func TestUpdateIndexesWritesEachVariantAndSkipsHole(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	vs := enabledVariants(variants)
	writeSegs(t, root, vs, "seg_00000.ts", "seg_00001.ts")
	writeSegs(t, root, vs[:1], "seg_00002.ts") // only first rung — not contiguous

	h := NewHandler(nil)
	h.durationOf = func(string) (float64, error) { return 3.001, nil }

	if err := h.UpdateIndexes(root, "video"); err != nil {
		t.Fatal(err)
	}

	for _, v := range vs {
		body, err := os.ReadFile(filepath.Join(root, v.Label, "index.m3u8"))
		if err != nil {
			t.Fatalf("read %s/index.m3u8: %v", v.Label, err)
		}
		s := string(body)
		if !strings.Contains(s, "seg_00000.ts") || !strings.Contains(s, "seg_00001.ts") {
			t.Fatalf("%s/index.m3u8 missing expected segments:\n%s", v.Label, s)
		}
		if strings.Contains(s, "seg_00002.ts") {
			t.Fatalf("%s/index.m3u8 listed a hole:\n%s", v.Label, s)
		}
	}
}

func TestEnsureMasterCreatesMissingDir(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "segment-test")
	h := NewHandler(nil)
	if err := h.EnsureMaster(root); err != nil {
		t.Fatal(err)
	}

	body, err := os.ReadFile(filepath.Join(root, "master.m3u8"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, want := range []string{
		"#EXTM3U\n",
		"#EXT-X-INDEPENDENT-SEGMENTS\n",
		"360p/index.m3u8\n",
		"1080p/index.m3u8\n",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("master missing %q\n%s", want, s)
		}
	}
	if strings.Contains(s, "\t") {
		t.Fatalf("master has indented tags (invalid for some players):\n%s", s)
	}

	if err := h.EnsureMaster(root); err != nil {
		t.Fatalf("second call: %v", err)
	}
}

func writeSegs(t *testing.T, root string, vs []Variant, names ...string) {
	t.Helper()
	for _, v := range vs {
		dir := filepath.Join(root, v.Label)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, name := range names {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
