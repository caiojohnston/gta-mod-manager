// Package config handles loading and saving the app's persisted state:
// the game folder path, the mod registry, profiles, and the (user-editable)
// installer rule table. See SPEC.md §4.5.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/caiojohnston/gta-mod-manager/internal/model"
)

// CurrentSchemaVersion must be bumped whenever the on-disk shape changes, and
// a migration added in Load. Never break existing users' saved state silently.
const CurrentSchemaVersion = 1

type RulePattern struct {
	// Match is either an exact filename (e.g. "dinput8.dll"), a glob
	// (e.g. "*.asi"), or a path-prefix check (e.g. "scripts/") — Installer
	// decides which based on the presence of "*" or a trailing "/".
	Match       string         `json:"match"`
	Destination model.FileKind `json:"destination"`
}

// Config is the whole on-disk state.
type Config struct {
	SchemaVersion int             `json:"schemaVersion"`
	GameDir       string          `json:"gameDir"`
	GameExeName   string          `json:"gameExeName"`
	Mods          []model.Mod     `json:"mods"`
	Profiles      []model.Profile `json:"profiles"`
	Rules         []RulePattern   `json:"rules"`
	// PreCleanModIDs holds the mod IDs that were enabled before the user hit
	// "Launch Clean" (SPEC.md §4.3). Non-empty means a clean run is active and
	// the UI should offer "Restore mods"; it survives a restart so a crash
	// mid-session can't strand the user with everything disabled and no memory
	// of what to turn back on.
	PreCleanModIDs []string `json:"preCleanModIds,omitempty"`
}

// DefaultRules mirrors SPEC.md §4.2's default table. Stored in config (not
// hard-coded in installer.go) so users can add patterns without recompiling.
func DefaultRules() []RulePattern {
	return []RulePattern{
		{Match: "dinput8.dll", Destination: model.KindRoot},
		{Match: "ScriptHookV.dll", Destination: model.KindRoot},
		{Match: "OpenIV.asi", Destination: model.KindRoot},
		{Match: "*.asi", Destination: model.KindRoot},
		{Match: "scripts/", Destination: model.KindScripts},
		{Match: "*.lua", Destination: model.KindScripts},
		{Match: "*.dll", Destination: model.KindScripts},
		// Content mods for the OpenRPF/OpenIV mods/ tree. These route to
		// model.KindMods; installer.DestinationPath preserves the sub-path and
		// installer.AutoApprove leaves the heuristic ones unchecked for review.
		{Match: "mods/", Destination: model.KindMods},
		{Match: "*.rpf", Destination: model.KindMods},
		{Match: "dlcpacks/", Destination: model.KindMods},
		{Match: "update/", Destination: model.KindMods},
		{Match: "common/", Destination: model.KindMods},
		{Match: "platform/", Destination: model.KindMods},
	}
}

func defaultConfig() *Config {
	return &Config{
		SchemaVersion: CurrentSchemaVersion,
		GameExeName:   "GTA5_Enhanced.exe",
		Mods:          []model.Mod{},
		Profiles:      []model.Profile{},
		Rules:         DefaultRules(),
	}
}

// Dir returns the directory the config file lives in (%AppData%\GTAVGoModManager
// on Windows; os.UserConfigDir()'s equivalent elsewhere so the app is still
// runnable on a dev machine for iteration).
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "GTAVGoModManager"), nil
}

func filePath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load reads config.json, creating a default one on first run.
func Load() (*Config, error) {
	path, err := filePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		cfg := defaultConfig()
		return cfg, Save(cfg)
	}
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	// Schema migrations would go here, keyed on cfg.SchemaVersion, before
	// returning. v1 has nothing to migrate from yet.
	backfillDefaults(&cfg)
	return &cfg, nil
}

// backfillDefaults fills in fields that a hand-edited or partially-written
// config might be missing, so the rest of the app never has to nil-check them.
func backfillDefaults(cfg *Config) {
	if cfg.SchemaVersion == 0 {
		cfg.SchemaVersion = CurrentSchemaVersion
	}
	if cfg.GameExeName == "" {
		cfg.GameExeName = defaultConfig().GameExeName
	}
	if len(cfg.Rules) == 0 {
		cfg.Rules = DefaultRules()
	}
	if cfg.Mods == nil {
		cfg.Mods = []model.Mod{}
	}
	if cfg.Profiles == nil {
		cfg.Profiles = []model.Profile{}
	}
}

// Save writes the config atomically-ish (write temp, rename) so a crash
// mid-write can't corrupt the registry the toggler depends on.
func Save(cfg *Config) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "config.json")
	tmp := path + ".tmp"
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
