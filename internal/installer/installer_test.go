package installer

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/caiojohnston/gta-mod-manager/internal/config"
	"github.com/caiojohnston/gta-mod-manager/internal/model"
)

func TestClassify(t *testing.T) {
	rules := config.DefaultRules()
	cases := map[string]model.FileKind{
		"OpenIV.asi":             model.KindRoot,
		"dinput8.dll":            model.KindRoot,
		"ScriptHookV.dll":        model.KindRoot,
		"scripts/Trainer.dll":    model.KindScripts,
		"pkg/scripts/config.ini": model.KindScripts,
		"MyMod.dll":              model.KindScripts,
		"menu.lua":               model.KindScripts,
		"mods/update/update.rpf": model.KindMods,
		"readme.txt":             model.KindOther,
		"preview/screenshot.png": model.KindOther,
	}
	for path, want := range cases {
		if got := Classify(path, rules); got != want {
			t.Errorf("Classify(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestDestinationPath(t *testing.T) {
	cases := []struct {
		path   string
		kind   model.FileKind
		want   string
		wantOK bool
	}{
		{"pack/OpenIV.asi", model.KindRoot, "OpenIV.asi", true},
		{"pack/scripts/T.dll", model.KindScripts, "scripts/T.dll", true},
		{"loose.dll", model.KindScripts, "scripts/loose.dll", true},
		{"x/mods/y.rpf", model.KindMods, "mods/y.rpf", true},
		{"readme.txt", model.KindOther, "", false},

		// content-mod path preservation
		{"NVE v1/onigiri/common/data/decals.dat", model.KindMods, "mods/common/data/decals.dat", true},
		{"Cool Mod/some.rpf", model.KindMods, "mods/some.rpf", true},
		{"bare/physicstasks.ymt", model.KindMods, "", false}, // no anchor: user must place it
		{"anything", model.KindCustom, "", false},            // custom is set directly, not derived

		// add-on dlcpacks always normalise to mods/update/x64/dlcpacks/<pack>/dlc.rpf
		{"vremastered/dlc.rpf", model.KindMods, "mods/update/x64/dlcpacks/vremastered/dlc.rpf", true},
		{"a80/dlc.rpf", model.KindMods, "mods/update/x64/dlcpacks/a80/dlc.rpf", true},
		{"Add-on/x64/dlcpacks/lc200/dlc.rpf", model.KindMods, "mods/update/x64/dlcpacks/lc200/dlc.rpf", true},
		{"dlc.rpf", model.KindMods, "", false}, // bare, no pack name
	}
	for _, c := range cases {
		got, ok := DestinationPath(c.path, c.kind)
		if got != c.want || ok != c.wantOK {
			t.Errorf("DestinationPath(%q,%q) = (%q,%v), want (%q,%v)", c.path, c.kind, got, ok, c.want, c.wantOK)
		}
	}
}

func TestAutoApprove(t *testing.T) {
	cases := []struct {
		path string
		kind model.FileKind
		want bool
	}{
		{"OpenIV.asi", model.KindRoot, true},
		{"scripts/x.dll", model.KindScripts, true},
		{"mods/update/x.rpf", model.KindMods, true},          // real mods/ folder in the archive
		{"pack/mods/update/x.rpf", model.KindMods, true},     // ditto, nested
		{"pack/onigiri/common/x.dat", model.KindMods, false}, // heuristic placement: needs review
		{"pack/some.rpf", model.KindMods, false},
		{"whatever", model.KindCustom, false},
		{"readme.txt", model.KindOther, false},
	}
	for _, c := range cases {
		if got := AutoApprove(c.path, c.kind); got != c.want {
			t.Errorf("AutoApprove(%q,%q) = %v, want %v", c.path, c.kind, got, c.want)
		}
	}
}

func TestClassifyContentMods(t *testing.T) {
	rules := config.DefaultRules()
	cases := map[string]model.FileKind{
		"pack/some.rpf":                       model.KindMods,
		"NVE/onigiri/common/data/decals.dat":  model.KindMods,
		"NVE/onigiri/platform/textures/x.ytd": model.KindMods,
		"mod/dlcpacks/foo/dlc.rpf":            model.KindMods,
	}
	for p, want := range cases {
		if got := Classify(p, rules); got != want {
			t.Errorf("Classify(%q) = %q, want %q", p, got, want)
		}
	}
}

func TestCommitRejectsEscapingCustomPath(t *testing.T) {
	tmp := t.TempDir()
	gameDir := filepath.Join(tmp, "game")
	src := filepath.Join(tmp, "src.txt")
	os.MkdirAll(gameDir, 0o755)
	os.WriteFile(src, []byte("x"), 0o644)

	files := []ProposedFile{
		{SourcePath: src, RelInArchive: "src.txt", Kind: model.KindCustom, Dest: "../../escape.txt", Approved: true},
	}
	if _, err := Commit(gameDir, "Evil", "src", files); err == nil {
		t.Fatal("Commit should reject a custom Dest that escapes the game folder")
	}
	if _, err := os.Stat(filepath.Join(tmp, "escape.txt")); !os.IsNotExist(err) {
		t.Error("escape file should not have been written")
	}
}

func writeZip(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, body := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestInspectAndCommitZip(t *testing.T) {
	tmp := t.TempDir()
	zipPath := filepath.Join(tmp, "CoolMod.zip")
	writeZip(t, zipPath, map[string]string{
		"CoolMod.asi":         "asi-bytes",
		"scripts/CoolMod.dll": "dll-bytes",
		"readme.txt":          "ignore me",
	})

	staging, files, cleanup, err := Inspect(zipPath, config.DefaultRules())
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	defer cleanup()
	if staging == "" {
		t.Error("expected a staging dir for a zip")
	}
	if len(files) != 3 {
		t.Fatalf("got %d proposed files, want 3", len(files))
	}

	var approvedReadme bool
	for _, f := range files {
		if f.RelInArchive == "readme.txt" {
			if f.Approved {
				approvedReadme = true
			}
			if f.Dest != "" {
				t.Errorf("readme.txt got a destination %q, want none", f.Dest)
			}
		}
	}
	if approvedReadme {
		t.Error("readme.txt should not be auto-approved")
	}

	gameDir := filepath.Join(tmp, "game")
	if err := os.MkdirAll(gameDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mod, err := Commit(gameDir, "CoolMod", zipPath, files)
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if len(mod.Files) != 2 {
		t.Fatalf("mod tracked %d files, want 2 (asi + scripts dll)", len(mod.Files))
	}
	if _, err := os.Stat(filepath.Join(gameDir, "CoolMod.asi")); err != nil {
		t.Errorf("CoolMod.asi not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(gameDir, "scripts", "CoolMod.dll")); err != nil {
		t.Errorf("scripts/CoolMod.dll not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(gameDir, "readme.txt")); !os.IsNotExist(err) {
		t.Error("readme.txt should not have been copied")
	}
	if mod.ID == "" {
		t.Error("Commit should assign a mod ID")
	}
}

func TestInspectFolder(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "extracted")
	if err := os.MkdirAll(filepath.Join(src, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(src, "Menu.asi"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(src, "scripts", "Menu.dll"), []byte("x"), 0o644)

	staging, files, cleanup, err := Inspect(src, config.DefaultRules())
	if err != nil {
		t.Fatalf("Inspect folder: %v", err)
	}
	defer cleanup()
	if staging != src {
		t.Errorf("folder staging = %q, want %q (no extraction for folders)", staging, src)
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2", len(files))
	}
}

func TestExtractZipRejectsZipSlip(t *testing.T) {
	tmp := t.TempDir()
	zipPath := filepath.Join(tmp, "evil.zip")
	writeZip(t, zipPath, map[string]string{
		"../escape.txt": "pwned",
	})
	_, _, cleanup, err := Inspect(zipPath, config.DefaultRules())
	if cleanup != nil {
		cleanup()
	}
	if err == nil {
		t.Fatal("expected Inspect to reject a zip entry that escapes the staging dir")
	}
}
