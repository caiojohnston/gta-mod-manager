package ui

import (
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/caiojohnston/gta-mod-manager/internal/config"
	"github.com/caiojohnston/gta-mod-manager/internal/model"
	"github.com/caiojohnston/gta-mod-manager/internal/procguard"
)

// newTestApp wires an App around a temp game folder with two installed mods
// (real files on disk), a headless Fyne window, config persistence redirected
// into the temp tree, and the process guard forced to "game closed".
func newTestApp(t *testing.T) (*App, string) {
	t.Helper()

	if os.PathSeparator == '\\' {
		t.Setenv("AppData", t.TempDir())
	} else {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	}
	restore := procguard.SetProcessLister(func() ([]string, error) { return nil, nil })
	t.Cleanup(restore)

	gameDir := t.TempDir()
	write := func(rel string) {
		p := filepath.Join(gameDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("Alpha.asi")
	write("scripts/Beta.dll")

	cfg := &config.Config{
		SchemaVersion: config.CurrentSchemaVersion,
		GameDir:       gameDir,
		GameExeName:   "GTA5_Enhanced.exe",
		Rules:         config.DefaultRules(),
		Mods: []model.Mod{
			{ID: "a", Name: "Alpha", Enabled: true, Files: []model.ModFile{{RelPath: "Alpha.asi", Kind: model.KindRoot}}},
			{ID: "b", Name: "Beta", Enabled: true, Files: []model.ModFile{{RelPath: "scripts/Beta.dll", Kind: model.KindScripts}}},
		},
	}

	win := test.NewWindow(nil)
	t.Cleanup(win.Close)
	a := NewApp(win, cfg)
	win.SetContent(a.Build())
	return a, gameDir
}

func TestBuildRendersRows(t *testing.T) {
	a, _ := newTestApp(t)
	if got := a.list.Length(); got != 2 {
		t.Fatalf("list length = %d, want 2", got)
	}
	if a.emptyHint == nil || a.emptyHint.Visible() {
		t.Error("empty hint should be hidden when mods exist")
	}
}

func TestToggleMovesFilesAndPersists(t *testing.T) {
	a, gameDir := newTestApp(t)

	a.toggleMod(a.modsBy["a"], false)

	if a.modsBy["a"].Enabled {
		t.Error("Alpha should be disabled")
	}
	if _, err := os.Stat(filepath.Join(gameDir, "Alpha.asi")); !os.IsNotExist(err) {
		t.Error("Alpha.asi should have moved out of the game root")
	}
	if _, err := os.Stat(filepath.Join(gameDir, "Disabled mods", "Alpha", "Alpha.asi")); err != nil {
		t.Errorf("Alpha.asi should be in Disabled mods/: %v", err)
	}

	// persisted?
	reloaded, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, m := range reloaded.Mods {
		if m.ID == "a" {
			found = true
			if m.Enabled {
				t.Error("persisted config still shows Alpha enabled")
			}
		}
	}
	if !found {
		t.Error("Alpha missing from reloaded config")
	}

	// toggle back
	a.toggleMod(a.modsBy["a"], true)
	if _, err := os.Stat(filepath.Join(gameDir, "Alpha.asi")); err != nil {
		t.Errorf("Alpha.asi should be back in the game root: %v", err)
	}
}

func TestBulkAndLaunchClean(t *testing.T) {
	a, gameDir := newTestApp(t)

	a.disableAll()
	for _, m := range a.Cfg.Mods {
		if m.Enabled {
			t.Fatalf("%s still enabled after disableAll", m.Name)
		}
	}

	a.enableAll()
	for _, m := range a.Cfg.Mods {
		if !m.Enabled {
			t.Fatalf("%s still disabled after enableAll", m.Name)
		}
	}

	a.launchClean()
	if len(a.Cfg.PreCleanModIDs) != 2 {
		t.Fatalf("PreCleanModIDs = %v, want 2 entries", a.Cfg.PreCleanModIDs)
	}
	if _, err := os.Stat(filepath.Join(gameDir, "Alpha.asi")); !os.IsNotExist(err) {
		t.Error("Alpha.asi should be disabled during a clean run")
	}

	a.restoreMods()
	if len(a.Cfg.PreCleanModIDs) != 0 {
		t.Errorf("PreCleanModIDs should be cleared after restore, got %v", a.Cfg.PreCleanModIDs)
	}
	if _, err := os.Stat(filepath.Join(gameDir, "Alpha.asi")); err != nil {
		t.Errorf("Alpha.asi should be restored: %v", err)
	}
}

func TestSortedModsViewOrder(t *testing.T) {
	mods := []model.Mod{
		{Name: "zeta", Enabled: true},
		{Name: "alpha", Enabled: false},
		{Name: "beta", Enabled: true},
	}
	order := sortedModsView(mods)
	// enabled first (zeta, beta by name), then disabled (alpha)
	got := []string{mods[order[0]].Name, mods[order[1]].Name, mods[order[2]].Name}
	want := []string{"beta", "zeta", "alpha"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}
