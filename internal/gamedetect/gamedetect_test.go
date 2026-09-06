package gamedetect

import (
	"os"
	"path/filepath"
	"testing"
)

func withProbe(t *testing.T, raws []rawCandidate) {
	t.Helper()
	prev := probeFn
	probeFn = func() []rawCandidate { return raws }
	t.Cleanup(func() { probeFn = prev })
}

func mkDir(t *testing.T, parent, name string, exe string) string {
	t.Helper()
	d := filepath.Join(parent, name)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if exe != "" {
		if err := os.WriteFile(filepath.Join(d, exe), []byte("MZ"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

func TestDetectFiltersAndSorts(t *testing.T) {
	tmp := t.TempDir()
	enhanced := mkDir(t, tmp, "GTAV Enhanced", "GTA5_Enhanced.exe")
	legacy := mkDir(t, tmp, "GTAV Legacy", "GTA5.exe")
	empty := mkDir(t, tmp, "NotAGame", "")

	withProbe(t, []rawCandidate{
		{path: legacy, src: SourceSteam},
		{path: empty, src: SourceCommonPath},
		{path: enhanced, src: SourceCommonPath},
		{path: enhanced, src: SourceRockstar}, // duplicate path, should be deduped
		{path: filepath.Join(tmp, "does-not-exist"), src: SourceEpic},
	})

	got := Detect()
	if len(got) != 2 {
		t.Fatalf("Detect returned %d candidates, want 2: %+v", len(got), got)
	}
	if got[0].Path != filepath.Clean(enhanced) || !got[0].Enhanced {
		t.Errorf("first candidate = %+v, want the Enhanced folder first", got[0])
	}
	if got[1].Path != filepath.Clean(legacy) || got[1].Enhanced {
		t.Errorf("second candidate = %+v, want the Legacy folder", got[1])
	}
	if got[0].Exe != "GTA5_Enhanced.exe" {
		t.Errorf("Exe = %q", got[0].Exe)
	}
}

func TestDetectEmptyWhenNothingValid(t *testing.T) {
	withProbe(t, []rawCandidate{{path: filepath.Join(t.TempDir(), "nope"), src: SourceSteam}})
	if got := Detect(); len(got) != 0 {
		t.Errorf("want no candidates, got %+v", got)
	}
}
