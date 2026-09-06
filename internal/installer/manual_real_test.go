package installer

import (
	"os"
	"strings"
	"testing"

	"github.com/caiojohnston/gta-mod-manager/internal/config"
)

// TestManualInspectRealMods runs Inspect against real folders on disk so the
// classifier's output can be eyeballed. Opt-in via INSTALLER_MANUAL_DIR
// (";"-separated list of folders); skipped otherwise.
func TestManualInspectRealMods(t *testing.T) {
	spec := os.Getenv("INSTALLER_MANUAL_DIR")
	if spec == "" {
		t.Skip("set INSTALLER_MANUAL_DIR=<folder>[;<folder>] to inspect real mods")
	}
	for _, d := range strings.Split(spec, ";") {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		_, files, cleanup, err := Inspect(d, config.DefaultRules())
		if err != nil {
			t.Errorf("Inspect(%q): %v", d, err)
			continue
		}
		t.Logf("=== %s : %d files", d, len(files))
		for _, f := range files {
			mark := " "
			if f.Approved {
				mark = "x"
			}
			t.Logf("  [%s] %-9s %-45s -> %s", mark, f.Kind, f.RelInArchive, f.Dest)
		}
		cleanup()
	}
}
