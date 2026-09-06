package gamedetect

import (
	"os"
	"testing"
)

// TestManualDetectThisMachine runs the real probe against the host it's built
// on. It's opt-in (set GAMEDETECT_MANUAL=1) so CI on a machine without GTA V
// doesn't see a confusing skip in normal runs.
func TestManualDetectThisMachine(t *testing.T) {
	if os.Getenv("GAMEDETECT_MANUAL") == "" {
		t.Skip("set GAMEDETECT_MANUAL=1 to run the real-machine probe")
	}
	got := Detect()
	t.Logf("Detect() -> %d candidate(s)", len(got))
	for i, c := range got {
		t.Logf("  [%d] %s | source=%s exe=%s enhanced=%v", i, c.Path, c.Source, c.Exe, c.Enhanced)
	}
	if len(got) == 0 {
		t.Log("no candidates (expected if GTA V isn't installed here)")
	}
}
