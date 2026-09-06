package dlclist

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackNames(t *testing.T) {
	rels := []string{
		"mods/update/x64/dlcpacks/vremastered/dlc.rpf",
		"mods/update/x64/dlcpacks/a80/dlc.rpf",
		"mods/update/x64/dlcpacks/a80/dlc.rpf", // dup
		"mods/update/update.rpf/common/data/ai/weapons.meta",
	}
	got := PackNames(rels)
	if len(got) != 2 || got[0] != "vremastered" || got[1] != "a80" {
		t.Fatalf("PackNames = %v, want [vremastered a80]", got)
	}
}

func writeDlclist(t *testing.T, gameDir, body string) {
	t.Helper()
	p := LoosePath(gameDir)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

const sample = `<?xml version="1.0" encoding="UTF-8"?>
<SMandatoryPacksData>
  <Paths>
    <Item>dlcpacks:/mpbeach/</Item>
    <Item>dlcpacks:/patchday1ng/</Item>
  </Paths>
</SMandatoryPacksData>`

func TestEnsureItemsAddsBeforeClosingTag(t *testing.T) {
	dir := t.TempDir()
	writeDlclist(t, dir, sample)

	added, err := EnsureItems(dir, []string{"vremastered", "mpbeach", "a80"})
	if err != nil {
		t.Fatalf("EnsureItems: %v", err)
	}
	if len(added) != 2 { // mpbeach already present
		t.Fatalf("added = %v, want [vremastered a80]", added)
	}

	out, _ := os.ReadFile(LoosePath(dir))
	s := string(out)
	if !strings.Contains(s, "<Item>dlcpacks:/vremastered/</Item>") ||
		!strings.Contains(s, "<Item>dlcpacks:/a80/</Item>") {
		t.Errorf("new items missing:\n%s", s)
	}
	// order: inserted lines come before </Paths>
	if strings.Index(s, "vremastered") > strings.Index(s, "</Paths>") {
		t.Error("new item was inserted after </Paths>")
	}
	// idempotent
	added2, err := EnsureItems(dir, []string{"vremastered", "a80"})
	if err != nil || len(added2) != 0 {
		t.Errorf("second run should add nothing, got %v (%v)", added2, err)
	}
}

func TestEnsureItemsMissingFile(t *testing.T) {
	if _, err := EnsureItems(t.TempDir(), []string{"a80"}); err == nil {
		t.Error("EnsureItems should error when dlclist.xml is absent")
	}
}

func TestCheck(t *testing.T) {
	dir := t.TempDir()
	if Check(dir) != StatusMissing {
		t.Error("want StatusMissing on empty dir")
	}

	// A packed RPF file at mods/update/update.rpf.
	packed := filepath.Join(dir, "mods", "update", "update.rpf")
	os.MkdirAll(filepath.Dir(packed), 0o755)
	os.WriteFile(packed, []byte("RPF8 packed bytes"), 0o644)
	if Check(dir) != StatusPackedRPF {
		t.Error("want StatusPackedRPF when update.rpf is a file")
	}

	// Replace it with the loose tree.
	os.Remove(packed)
	writeDlclist(t, dir, sample)
	if Check(dir) != StatusEditable {
		t.Error("want StatusEditable once the loose file exists")
	}
}
