package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/caiojohnston/gta-mod-manager/internal/config"
	"github.com/caiojohnston/gta-mod-manager/internal/installer"
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

// renderRow drives a widget.List's template + update funcs the way Fyne's
// renderer does, so a template/index mismatch panics here instead of at
// runtime when the user opens the screen.
func renderRow(t *testing.T, l *widget.List, id int) fyne.CanvasObject {
	t.Helper()
	obj := l.CreateItem()
	l.UpdateItem(id, obj)
	return obj
}

func TestModListRowRenders(t *testing.T) {
	a, _ := newTestApp(t)
	row := renderRow(t, a.list, 0).(*fyne.Container)
	check := row.Objects[1].(*widget.Check)
	if !check.Checked {
		t.Error("row 0 (Alpha, enabled) should render a checked box")
	}
	btns := row.Objects[2].(*fyne.Container)
	if len(btns.Objects) != 2 {
		t.Fatalf("row should have forget + uninstall buttons, got %d", len(btns.Objects))
	}
	_ = btns.Objects[0].(*widget.Button)
	_ = btns.Objects[1].(*widget.Button)
}

func TestRemoveModByIDKeepsOthers(t *testing.T) {
	mods := []model.Mod{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	got := removeModByID(mods, "b")
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "c" {
		t.Fatalf("removeModByID = %+v, want [a c]", got)
	}
}

func TestReviewListRowRenders(t *testing.T) {
	a, _ := newTestApp(t)
	files := []installer.ProposedFile{
		{RelInArchive: "ScriptHookV.dll", Kind: model.KindRoot, Dest: "ScriptHookV.dll", Approved: true},
		{RelInArchive: "readme.txt", Kind: model.KindOther},
		{RelInArchive: "Mod v1/onigiri/common/data/effects/decals.dat", Kind: model.KindMods, Dest: "mods/common/data/effects/decals.dat"},
	}
	rc := &review{
		a: a, rules: config.DefaultRules(), files: files,
		selection: make([]string, len(files)), custom: make([]string, len(files)),
		wrapper: commonWrapperDir(files),
	}
	rc.list = rc.newList()
	for i := range files {
		row := renderRow(t, rc.list, i).(*fyne.Container)
		_ = row.Objects[0].(*widget.Label)
		_ = row.Objects[1].(*widget.Check)
		_ = row.Objects[2].(*widget.Select)
	}
}

func TestReviewBasePathAppliesToAll(t *testing.T) {
	a, _ := newTestApp(t)
	files := []installer.ProposedFile{
		{RelInArchive: "Mod v1/physicstasks.ymt"},
		{RelInArchive: "Mod v1/ReadMe.txt"},
	}
	rc := &review{
		a: a, rules: config.DefaultRules(), files: files,
		selection: make([]string, len(files)), custom: make([]string, len(files)),
		wrapper: commonWrapperDir(files),
	}
	rc.list = rc.newList()

	// Simulate the "Set base path for all" dialog callback body.
	prefix := "mods/update/update.rpf/x64/data/tune"
	for i := range rc.files {
		rel := strings.TrimPrefix(rc.files[i].RelInArchive, rc.wrapper)
		rc.files[i].Dest = prefix + "/" + rel
	}
	if files[0].Dest != "mods/update/update.rpf/x64/data/tune/physicstasks.ymt" {
		t.Fatalf("ymt dest = %q", files[0].Dest)
	}
	if rc.wrapper != "Mod v1/" {
		t.Fatalf("wrapper = %q, want \"Mod v1/\"", rc.wrapper)
	}
}

func TestProfileListRowRenders(t *testing.T) {
	a, _ := newTestApp(t)
	a.Cfg.Profiles = []model.Profile{{Name: "Graphics", EnabledModIDs: []string{"a"}}}
	l := a.newProfileList(nil)
	row := renderRow(t, l, 0).(*fyne.Container)
	name := row.Objects[0].(*widget.Label)
	if name.Text == "" {
		t.Error("profile row should show a name")
	}
	_ = row.Objects[1].(*fyne.Container) // the apply/delete button box
}

// TestBorderObjectsOrder pins the container.NewBorder(.Objects) ordering the
// list update funcs rely on: center object(s) first, then each non-nil border
// in top, bottom, left, right order.
func TestBorderObjectsOrder(t *testing.T) {
	center := widget.NewLabel("C")
	left := widget.NewCheck("", nil)
	right := widget.NewSelect(nil, nil)
	c := container.NewBorder(nil, nil, left, right, center)
	if len(c.Objects) != 3 || c.Objects[0] != center || c.Objects[1] != left || c.Objects[2] != right {
		t.Fatalf("NewBorder .Objects order changed: got %#v", c.Objects)
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
