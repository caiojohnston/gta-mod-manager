package rpf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestManualExtractRealRPF extracts real .rpf files listed in RPF_MANUAL
// (";"-separated <src.rpf>=<destdir> pairs, or just <src.rpf> to dump under a
// temp dir) and reports the tree. Opt-in.
func TestManualExtractRealRPF(t *testing.T) {
	spec := os.Getenv("RPF_MANUAL")
	if spec == "" {
		t.Skip("set RPF_MANUAL=<a.rpf>=<destdir>[;<b.rpf>=<destdir>]")
	}
	for _, pair := range strings.Split(spec, ";") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		src, dest, ok := strings.Cut(pair, "=")
		if !ok {
			dest = filepath.Join(t.TempDir(), "out")
		}
		n, err := ExtractTo(src, dest)
		if err != nil {
			t.Errorf("ExtractTo(%s): %v", src, err)
			continue
		}
		t.Logf("=== %s -> %s : %d file(s)", src, dest, n)
		filepath.Walk(dest, func(p string, info os.FileInfo, e error) error {
			if e != nil || info.IsDir() {
				return e
			}
			rel, _ := filepath.Rel(dest, p)
			t.Logf("   %8d  %s", info.Size(), rel)
			return nil
		})
	}
}
