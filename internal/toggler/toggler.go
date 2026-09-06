// Package toggler implements SPEC.md §4.3: enabling/disabling a mod by
// moving its tracked files between the live game folder and
// "Disabled mods/<mod-name>/", preserving relative paths.
package toggler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/caiojohnston/gta-mod-manager/internal/model"
	"github.com/caiojohnston/gta-mod-manager/internal/procguard"
)

const disabledFolderName = "Disabled mods"

// Disable moves every file belonging to mod out of the live game folder and
// into "Disabled mods/<mod.Name>/", mirroring relative paths. Returns an
// *procguard.ErrGameRunning if the game is currently running — callers must
// check errors.As for that case to show a specific message.
func Disable(gameDir, gameExeName string, mod *model.Mod) error {
	if running, err := procguard.IsGameRunning(gameExeName); err != nil {
		return err
	} else if running {
		return &procguard.ErrGameRunning{ExeName: gameExeName}
	}
	if !mod.Enabled {
		return nil // already disabled, no-op
	}
	disabledRoot := filepath.Join(gameDir, disabledFolderName, mod.Name)
	for _, f := range mod.Files {
		src := filepath.Join(gameDir, filepath.FromSlash(f.RelPath))
		dst := filepath.Join(disabledRoot, filepath.FromSlash(f.RelPath))
		if err := moveFile(src, dst); err != nil {
			return fmt.Errorf("toggler: disabling %q, moving %q: %w", mod.Name, f.RelPath, err)
		}
		pruneEmptyDirs(gameDir, filepath.Dir(src))
	}
	mod.Enabled = false
	return nil
}

// Enable is the reverse of Disable: moves files back from
// "Disabled mods/<mod.Name>/" into their live locations.
func Enable(gameDir, gameExeName string, mod *model.Mod) error {
	if running, err := procguard.IsGameRunning(gameExeName); err != nil {
		return err
	} else if running {
		return &procguard.ErrGameRunning{ExeName: gameExeName}
	}
	if mod.Enabled {
		return nil // already enabled, no-op
	}
	disabledRoot := filepath.Join(gameDir, disabledFolderName, mod.Name)
	for _, f := range mod.Files {
		src := filepath.Join(disabledRoot, filepath.FromSlash(f.RelPath))
		dst := filepath.Join(gameDir, filepath.FromSlash(f.RelPath))
		if err := moveFile(src, dst); err != nil {
			return fmt.Errorf("toggler: enabling %q, moving %q: %w", mod.Name, f.RelPath, err)
		}
		pruneEmptyDirs(disabledRoot, filepath.Dir(src))
	}
	mod.Enabled = true
	// Best-effort cleanup of the now-empty disabled folder for this mod.
	_ = os.Remove(disabledRoot)
	return nil
}

// Uninstall permanently deletes every file belonging to mod, whether the mod
// is currently enabled (files live in gameDir) or disabled (files live in
// "Disabled mods/<mod.Name>/"). This is the one operation SPEC.md allows to
// delete rather than move — the caller must have confirmed with the user, and
// is responsible for dropping the mod from the registry afterwards.
//
// keepRelPaths are game-relative paths that another tracked mod also claims:
// they are left on disk (so uninstalling one of two mods that share a file
// doesn't break the other) but the caller still drops this mod's registry
// entry.
func Uninstall(gameDir, gameExeName string, mod *model.Mod, keepRelPaths map[string]bool) error {
	if running, err := procguard.IsGameRunning(gameExeName); err != nil {
		return err
	} else if running {
		return &procguard.ErrGameRunning{ExeName: gameExeName}
	}

	base := gameDir
	if !mod.Enabled {
		base = filepath.Join(gameDir, disabledFolderName, mod.Name)
	}
	for _, f := range mod.Files {
		if keepRelPaths[filepath.ToSlash(f.RelPath)] {
			continue // still owned by another mod
		}
		p := filepath.Join(base, filepath.FromSlash(f.RelPath))
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("toggler: uninstalling %q, removing %q: %w", mod.Name, f.RelPath, err)
		}
		pruneEmptyDirs(base, filepath.Dir(p))
	}
	if !mod.Enabled {
		_ = os.Remove(base) // best-effort: drop the now-empty per-mod disabled folder
	}
	return nil
}

// pruneEmptyDirs walks up from dir removing directories that are now empty,
// stopping before it reaches (and never touching) stopAt. Used after moving or
// deleting a mod's files so empty "scripts/Foo/" style folders don't pile up.
func pruneEmptyDirs(stopAt, dir string) {
	stopAt = filepath.Clean(stopAt)
	for {
		dir = filepath.Clean(dir)
		if dir == stopAt || !strings.HasPrefix(dir, stopAt+string(filepath.Separator)) {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			return
		}
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

// moveFile moves src to dst, creating dst's parent directories as needed.
// Uses os.Rename first (cheap, same-volume case) and falls back to a
// copy+remove if the move crosses a filesystem boundary.
func moveFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	// Fallback: cross-device rename isn't allowed by the OS; copy then remove.
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := out.ReadFrom(in); err != nil {
		return err
	}
	return os.Remove(src)
}
