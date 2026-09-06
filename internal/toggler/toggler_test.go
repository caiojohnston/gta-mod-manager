package toggler

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/caiojohnston/gta-mod-manager/internal/model"
	"github.com/caiojohnston/gta-mod-manager/internal/procguard"
)

const fakeExe = "GTA5_Enhanced.exe"

// gameNotRunning makes procguard report the game as closed for the duration of
// a test.
func gameNotRunning(t *testing.T) {
	t.Helper()
	restore := procguard.SetProcessLister(func() ([]string, error) { return []string{"explorer.exe"}, nil })
	t.Cleanup(restore)
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected %s to exist: %v", path, err)
	}
}

func mustNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected %s to be gone, err=%v", path, err)
	}
}

func sampleMod(gameDir string) *model.Mod {
	return &model.Mod{
		ID:      "m1",
		Name:    "Sample Mod",
		Enabled: true,
		Files: []model.ModFile{
			{RelPath: "Sample.asi", Kind: model.KindRoot},
			{RelPath: "scripts/Sample.dll", Kind: model.KindScripts},
			{RelPath: "scripts/sub/data.ini", Kind: model.KindScripts},
		},
	}
}

func TestDisableEnableRoundtrip(t *testing.T) {
	gameNotRunning(t)
	gameDir := t.TempDir()
	mod := sampleMod(gameDir)
	for _, f := range mod.Files {
		writeFile(t, filepath.Join(gameDir, filepath.FromSlash(f.RelPath)), "data")
	}

	if err := Disable(gameDir, fakeExe, mod); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if mod.Enabled {
		t.Error("mod should be marked disabled")
	}
	mustNotExist(t, filepath.Join(gameDir, "Sample.asi"))
	mustExist(t, filepath.Join(gameDir, disabledFolderName, "Sample Mod", "Sample.asi"))
	mustExist(t, filepath.Join(gameDir, disabledFolderName, "Sample Mod", "scripts", "sub", "data.ini"))
	// The live scripts/sub tree should have been pruned once emptied.
	mustNotExist(t, filepath.Join(gameDir, "scripts", "sub"))

	if err := Enable(gameDir, fakeExe, mod); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if !mod.Enabled {
		t.Error("mod should be marked enabled")
	}
	mustExist(t, filepath.Join(gameDir, "Sample.asi"))
	mustExist(t, filepath.Join(gameDir, "scripts", "sub", "data.ini"))
	mustNotExist(t, filepath.Join(gameDir, disabledFolderName, "Sample Mod"))
}

func TestDisableIsNoOpWhenAlreadyDisabled(t *testing.T) {
	gameNotRunning(t)
	gameDir := t.TempDir()
	mod := sampleMod(gameDir)
	mod.Enabled = false
	if err := Disable(gameDir, fakeExe, mod); err != nil {
		t.Fatalf("Disable no-op: %v", err)
	}
}

func TestUninstallEnabled(t *testing.T) {
	gameNotRunning(t)
	gameDir := t.TempDir()
	mod := sampleMod(gameDir)
	for _, f := range mod.Files {
		writeFile(t, filepath.Join(gameDir, filepath.FromSlash(f.RelPath)), "data")
	}

	if err := Uninstall(gameDir, fakeExe, mod); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	mustNotExist(t, filepath.Join(gameDir, "Sample.asi"))
	mustNotExist(t, filepath.Join(gameDir, "scripts", "Sample.dll"))
	mustNotExist(t, filepath.Join(gameDir, "scripts", "sub"))
}

func TestUninstallDisabled(t *testing.T) {
	gameNotRunning(t)
	gameDir := t.TempDir()
	mod := sampleMod(gameDir)
	for _, f := range mod.Files {
		writeFile(t, filepath.Join(gameDir, filepath.FromSlash(f.RelPath)), "data")
	}
	if err := Disable(gameDir, fakeExe, mod); err != nil {
		t.Fatal(err)
	}
	if err := Uninstall(gameDir, fakeExe, mod); err != nil {
		t.Fatalf("Uninstall (disabled): %v", err)
	}
	mustNotExist(t, filepath.Join(gameDir, disabledFolderName, "Sample Mod"))
}

func TestGuardBlocksWhenGameRunning(t *testing.T) {
	restore := procguard.SetProcessLister(func() ([]string, error) {
		return []string{"GTA5_Enhanced.exe"}, nil
	})
	defer restore()

	gameDir := t.TempDir()
	mod := sampleMod(gameDir)
	for _, f := range mod.Files {
		writeFile(t, filepath.Join(gameDir, filepath.FromSlash(f.RelPath)), "data")
	}

	err := Disable(gameDir, fakeExe, mod)
	var running *procguard.ErrGameRunning
	if !errors.As(err, &running) {
		t.Fatalf("Disable while game running: got %v, want ErrGameRunning", err)
	}
	mustExist(t, filepath.Join(gameDir, "Sample.asi")) // nothing moved
}
