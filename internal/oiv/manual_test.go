package oiv

import (
	"os"
	"strings"
	"testing"
)

// TestManualRealOIV reads real .oiv files listed in OIV_MANUAL (";"-separated)
// and prints the install plan. Opt-in; skipped otherwise.
func TestManualRealOIV(t *testing.T) {
	spec := os.Getenv("OIV_MANUAL")
	if spec == "" {
		t.Skip("set OIV_MANUAL=<a.oiv>[;<b.oiv>] to inspect real packages")
	}
	for _, p := range strings.Split(spec, ";") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		pkg, err := Read(p)
		if err != nil {
			t.Errorf("Read(%s): %v", p, err)
			continue
		}
		t.Logf("=== %s  (%q v%s)  %d add(s), %d unsupported", p, pkg.Name, pkg.Version, len(pkg.Adds), len(pkg.Unsupported))
		for _, a := range pkg.Adds {
			t.Logf("  ADD %-45s -> %s", a.SourceName, a.ModsRelPath)
		}
		for _, u := range pkg.Unsupported {
			t.Logf("  SKIP %s", u)
		}
	}
}
