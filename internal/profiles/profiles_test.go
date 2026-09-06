package profiles

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/caiojohnston/gta-mod-manager/internal/model"
	"github.com/caiojohnston/gta-mod-manager/internal/procguard"
)

func TestCurrentAsProfile(t *testing.T) {
	mods := []*model.Mod{
		{ID: "a", Enabled: true},
		{ID: "b", Enabled: false},
		{ID: "c", Enabled: true},
	}
	p := CurrentAsProfile("snap", mods)
	if p.Name != "snap" {
		t.Errorf("name = %q", p.Name)
	}
	if len(p.EnabledModIDs) != 2 || p.EnabledModIDs[0] != "a" || p.EnabledModIDs[1] != "c" {
		t.Errorf("EnabledModIDs = %v, want [a c]", p.EnabledModIDs)
	}
}

func TestSwitchMovesOnlyDelta(t *testing.T) {
	restore := procguard.SetProcessLister(func() ([]string, error) { return nil, nil })
	defer restore()

	gameDir := t.TempDir()
	mk := func(id string, enabled bool) *model.Mod {
		rel := id + ".asi"
		m := &model.Mod{ID: id, Name: id, Enabled: enabled, Files: []model.ModFile{{RelPath: rel, Kind: model.KindRoot}}}
		base := gameDir
		if !enabled {
			base = filepath.Join(gameDir, "Disabled mods", id)
		}
		path := filepath.Join(base, rel)
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, []byte("x"), 0o644)
		return m
	}

	// a: on->stays on, b: on->off, c: off->on
	mods := []*model.Mod{mk("a", true), mk("b", true), mk("c", false)}
	target := model.Profile{Name: "graphics", EnabledModIDs: []string{"a", "c"}}

	if err := Switch(gameDir, "GTA5_Enhanced.exe", mods, target); err != nil {
		t.Fatalf("Switch: %v", err)
	}

	if !mods[0].Enabled || mods[1].Enabled || !mods[2].Enabled {
		t.Errorf("enabled states = [%v %v %v], want [true false true]",
			mods[0].Enabled, mods[1].Enabled, mods[2].Enabled)
	}
	if _, err := os.Stat(filepath.Join(gameDir, "a.asi")); err != nil {
		t.Errorf("a.asi should still be live: %v", err)
	}
	if _, err := os.Stat(filepath.Join(gameDir, "b.asi")); !os.IsNotExist(err) {
		t.Error("b.asi should have been disabled")
	}
	if _, err := os.Stat(filepath.Join(gameDir, "c.asi")); err != nil {
		t.Errorf("c.asi should have been enabled: %v", err)
	}
}
