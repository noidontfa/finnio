package abr

import (
	"strings"
	"testing"
)

func TestEncodeArgsKeepsSourceTimeline(t *testing.T) {
	t.Parallel()

	vs := enabledVariants(variants)
	args := encodeArgs("/src/seg_00007.ts", "/out", vs, true)
	joined := strings.Join(args, " ")

	if !strings.Contains(joined, "-copyts") {
		t.Fatalf("encode must not rebase timestamps to zero: %v", args)
	}
	if got := strings.Count(joined, "-muxdelay 0 -muxpreload 0"); got != len(vs) {
		t.Fatalf("mux offset cleared for %d of %d variants: %v", got, len(vs), args)
	}
}

func TestEncodeArgsWritesTempFilesPerVariant(t *testing.T) {
	t.Parallel()

	vs := enabledVariants(variants)
	args := encodeArgs("/src/seg_00007.ts", "/out", vs, false)
	joined := strings.Join(args, " ")

	for _, v := range vs {
		want := variantTmpPath("/out", v, "seg_00007.ts")
		if !strings.Contains(joined, want) {
			t.Fatalf("%s output missing %q: %v", v.Label, want, args)
		}
	}
	if strings.Contains(joined, "seg_00007.ts "+encodeTmpSuffix) {
		t.Fatalf("malformed temp path: %v", args)
	}
}
