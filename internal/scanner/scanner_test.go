package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/caiojohnston/gta-mod-manager/internal/config"
	"github.com/caiojohnston/gta-mod-manager/internal/model"
)

func TestFindUnmanaged(t *testing.T) {
	gameDir := t.TempDir()
	write := func(rel string) {
		p := filepath.Join(gameDir, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte("x"), 0o644)
	}
	write("dinput8.dll")
	write("OpenIV.asi")
	write("scripts/Trainer.dll")
	write("scripts/Trainer.ini")
	write("mods/update.rpf")
	write("PlayGTAV.exe")       // not a mod pattern
	write("Profiles/save1.sav") // unrelated folder, must be ignored

	found, err := FindUnmanaged(gameDir, config.DefaultRules(), nil)
	if err != nil {
		t.Fatalf("FindUnmanaged: %v", err)
	}

	names := map[string]int{} // name -> file count
	for _, m := range found {
		names[m.Name] = len(m.Files)
		if m.Source != "scanned" {
			t.Errorf("%s source = %q, want scanned", m.Name, m.Source)
		}
	}

	if _, ok := names["dinput8.dll"]; !ok {
		t.Error("dinput8.dll not detected")
	}
	if _, ok := names["OpenIV.asi"]; !ok {
		t.Error("OpenIV.asi not detected")
	}
	if names["scripts"] != 2 {
		t.Errorf("scripts folder mod = %d files, want 2", names["scripts"])
	}
	if names["mods"] != 1 {
		t.Errorf("mods folder mod = %d files, want 1", names["mods"])
	}
	if _, ok := names["PlayGTAV.exe"]; ok {
		t.Error("PlayGTAV.exe should not be treated as a mod")
	}
	if _, ok := names["Profiles"]; ok {
		t.Error("Profiles/ should be ignored")
	}
}

func TestFindUnmanagedSkipsTrackedFiles(t *testing.T) {
	gameDir := t.TempDir()
	p := filepath.Join(gameDir, "OpenIV.asi")
	os.WriteFile(p, []byte("x"), 0o644)

	existing := []model.Mod{
		{ID: "x", Name: "OpenIV", Files: []model.ModFile{{RelPath: "OpenIV.asi", Kind: model.KindRoot}}},
	}
	found, err := FindUnmanaged(gameDir, config.DefaultRules(), existing)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 0 {
		t.Errorf("expected nothing new (file already tracked), got %d", len(found))
	}
}
