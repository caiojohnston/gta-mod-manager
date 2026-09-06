// Package model holds the domain types shared across the installer, toggler,
// profiles and UI packages. Keeping these in one place means every package
// agrees on what a "Mod" is without importing each other.
package model

import "time"

// FileKind records why a file ended up at a given destination, so the UI can
// explain itself and rules.go's table stays inspectable/debuggable.
type FileKind string

const (
	KindRoot    FileKind = "root"    // .asi, dinput8.dll, ScriptHookV.dll -> game root
	KindScripts FileKind = "scripts" // .dll / .lua -> scripts/
	KindMods    FileKind = "mods"    // content for the OpenRPF/OpenIV mods/ tree, path preserved
	KindCustom  FileKind = "custom"  // user typed an explicit destination path (SPEC G6)
	KindOther   FileKind = "other"   // matched no rule; needs manual placement
)

// ModFile is one file that belongs to a Mod, recorded at install time so that
// toggling later is purely mechanical (see PLAN.md decision 1).
type ModFile struct {
	// RelPath is the path relative to the game install root, e.g. "scripts/MyMod.dll".
	RelPath string   `json:"relPath"`
	Kind    FileKind `json:"kind"`
}

// Mod is one tracked, toggleable unit.
type Mod struct {
	ID          string    `json:"id"` // stable, generated at install time
	Name        string    `json:"name"`
	Files       []ModFile `json:"files"`
	Enabled     bool      `json:"enabled"`
	InstalledAt time.Time `json:"installedAt"`
	// Source records where this came from, purely informational (archive path,
	// folder path, or "scanned" for mods imported from a pre-existing install).
	Source string `json:"source"`
}

// Profile is a named snapshot of which mod IDs should be enabled.
type Profile struct {
	Name          string   `json:"name"`
	EnabledModIDs []string `json:"enabledModIds"`
}
