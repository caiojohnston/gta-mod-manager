package oiv

import (
	"archive/zip"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/caiojohnston/gta-mod-manager/internal/installer"
	"github.com/caiojohnston/gta-mod-manager/internal/model"
)

func makeOIV(t *testing.T, assembly string, files map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "test.oiv")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	add := func(name, body string) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(body))
	}
	add("assembly.xml", assembly)
	for name, body := range files {
		add(name, body)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

const assemblyAddsOnly = `<?xml version="1.0" encoding="UTF-8"?>
<package version="2.0" target="Five">
<metadata><name>Test Mod</name><version><major>4</major><minor>1</minor></version></metadata>
<content>
  <archive path="update\update.rpf" createIfNotExist="False" type="RPF7">
    <add source="metas\weapons.meta">\common\data\ai\weapons.meta</add>
    <archive path="dlc_patch\mpbusiness\dlc.rpf" type="RPF7">
      <add source="metas\heavypistol.meta">\common\data\ai\heavypistol.meta</add>
    </archive>
  </archive>
  <archive path="x64\audio\sfx\WEAPONS_PLAYER.rpf" createIfNotExist="True" type="RPF7">
    <add source="audio\ptl_pistol.awc">ptl_pistol.awc</add>
  </archive>
</content>
</package>`

func TestReadAddsAndPaths(t *testing.T) {
	oivPath := makeOIV(t, assemblyAddsOnly, map[string]string{
		"content/metas/weapons.meta":     "bin1",
		"content/metas/heavypistol.meta": "bin2",
		"content/audio/ptl_pistol.awc":   "bin3",
	})
	pkg, err := Read(oivPath)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if pkg.Name != "Test Mod" || pkg.Version != "4.1" {
		t.Errorf("meta = %q %q", pkg.Name, pkg.Version)
	}
	if len(pkg.Unsupported) != 0 {
		t.Errorf("Unsupported = %v, want none", pkg.Unsupported)
	}

	got := map[string]string{}
	for _, a := range pkg.Adds {
		got[a.SourceName] = a.ModsRelPath
	}
	want := map[string]string{
		"metas/weapons.meta":     "mods/update/update.rpf/common/data/ai/weapons.meta",
		"metas/heavypistol.meta": "mods/update/update.rpf/dlc_patch/mpbusiness/dlc.rpf/common/data/ai/heavypistol.meta",
		"audio/ptl_pistol.awc":   "mods/x64/audio/sfx/WEAPONS_PLAYER.rpf/ptl_pistol.awc",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s -> %q, want %q", k, got[k], v)
		}
	}

	if err := pkg.Extract(); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	defer pkg.Close()
	for _, a := range pkg.Adds {
		if a.ExtractedPath == "" {
			t.Errorf("%s not extracted", a.SourceName)
			continue
		}
		if _, err := os.Stat(a.ExtractedPath); err != nil {
			t.Errorf("extracted file missing: %v", err)
		}
	}
}

func TestReadReportsUnsupported(t *testing.T) {
	asm := `<?xml version="1.0"?>
<package><metadata><name>X</name></metadata><content>
  <archive path="update\update.rpf" type="RPF7">
    <add source="a.meta">\common\a.meta</add>
    <delete>\common\old.meta</delete>
    <xml path="\common\data\dlclist.xml"><add>x</add></xml>
  </archive>
</content></package>`
	pkg, err := Read(makeOIV(t, asm, map[string]string{"content/a.meta": "x"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(pkg.Adds) != 1 {
		t.Fatalf("Adds = %d, want 1", len(pkg.Adds))
	}
	sort.Strings(pkg.Unsupported)
	if len(pkg.Unsupported) != 2 {
		t.Fatalf("Unsupported = %v, want 2 (delete + xml)", pkg.Unsupported)
	}
}

// TestOIVToLooseInstall is the end-to-end the UI performs: read a .oiv, turn
// its adds into installer.ProposedFile, Commit them under the game folder.
func TestOIVToLooseInstall(t *testing.T) {
	oivPath := makeOIV(t, assemblyAddsOnly, map[string]string{
		"content/metas/weapons.meta":     "META-BYTES",
		"content/metas/heavypistol.meta": "bin2",
		"content/audio/ptl_pistol.awc":   "AUDIO",
	})
	pkg, err := Read(oivPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := pkg.Extract(); err != nil {
		t.Fatal(err)
	}
	defer pkg.Close()

	files := make([]installer.ProposedFile, 0, len(pkg.Adds))
	for _, a := range pkg.Adds {
		files = append(files, installer.ProposedFile{
			SourcePath: a.ExtractedPath, RelInArchive: a.SourceName,
			Kind: model.KindCustom, Dest: a.ModsRelPath, Approved: true,
		})
	}

	gameDir := t.TempDir()
	mod, err := installer.Commit(gameDir, pkg.Name, oivPath, files)
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if len(mod.Files) != 3 {
		t.Fatalf("tracked %d files, want 3", len(mod.Files))
	}
	want := filepath.Join(gameDir, "mods", "update", "update.rpf", "common", "data", "ai", "weapons.meta")
	b, err := os.ReadFile(want)
	if err != nil || string(b) != "META-BYTES" {
		t.Errorf("weapons.meta not written correctly: %v / %q", err, b)
	}
	nested := filepath.Join(gameDir, "mods", "x64", "audio", "sfx", "WEAPONS_PLAYER.rpf", "ptl_pistol.awc")
	if _, err := os.Stat(nested); err != nil {
		t.Errorf("nested audio file missing: %v", err)
	}
}

func TestReadRejectsNonOIV(t *testing.T) {
	p := filepath.Join(t.TempDir(), "plain.zip")
	f, _ := os.Create(p)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("readme.txt")
	w.Write([]byte("hi"))
	zw.Close()
	f.Close()

	if _, err := Read(p); err == nil {
		t.Error("Read should reject a zip with no assembly.xml")
	}
}
