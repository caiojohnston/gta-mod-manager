package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/caiojohnston/gta-mod-manager/internal/model"
)

// redirectConfigDir points config.Dir() at a temp folder for the test by
// overriding the platform's config-dir env var.
func redirectConfigDir(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	switch {
	case os.Getenv("AppData") != "" || isWindows():
		t.Setenv("AppData", tmp)
	default:
		t.Setenv("XDG_CONFIG_HOME", tmp)
	}
	return tmp
}

func isWindows() bool { return os.PathSeparator == '\\' }

func TestSaveLoadRoundtrip(t *testing.T) {
	redirectConfigDir(t)

	cfg, err := Load() // first run: writes a default file
	if err != nil {
		t.Fatalf("Load (first run): %v", err)
	}
	if cfg.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("SchemaVersion = %d", cfg.SchemaVersion)
	}
	if cfg.GameExeName == "" || len(cfg.Rules) == 0 {
		t.Error("default config should have GameExeName and Rules populated")
	}

	cfg.GameDir = `C:\Games\GTAV Enhanced`
	cfg.Mods = append(cfg.Mods, model.Mod{ID: "m1", Name: "Test", Enabled: true})
	cfg.PreCleanModIDs = []string{"m1"}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load (second): %v", err)
	}
	if got.GameDir != cfg.GameDir {
		t.Errorf("GameDir = %q, want %q", got.GameDir, cfg.GameDir)
	}
	if len(got.Mods) != 1 || got.Mods[0].ID != "m1" {
		t.Errorf("Mods = %+v", got.Mods)
	}
	if len(got.PreCleanModIDs) != 1 || got.PreCleanModIDs[0] != "m1" {
		t.Errorf("PreCleanModIDs = %v", got.PreCleanModIDs)
	}
}

func TestBackfillDefaults(t *testing.T) {
	dir := redirectConfigDir(t)

	// Write a sparse config missing GameExeName and Rules.
	sparse := map[string]any{"gameDir": `D:\GTAV`}
	data, _ := json.Marshal(sparse)
	cfgDir := filepath.Join(dir, "GTAVGoModManager")
	os.MkdirAll(cfgDir, 0o755)
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.GameExeName == "" {
		t.Error("GameExeName should be backfilled")
	}
	if len(cfg.Rules) == 0 {
		t.Error("Rules should be backfilled")
	}
	if cfg.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("SchemaVersion should be backfilled, got %d", cfg.SchemaVersion)
	}
	if cfg.Mods == nil || cfg.Profiles == nil {
		t.Error("Mods/Profiles should be non-nil after backfill")
	}
}
