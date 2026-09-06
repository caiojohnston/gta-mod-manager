// Package toggler implements SPEC.md §4.3: enabling/disabling a mod by
// moving its tracked files between the live game folder and
// "Disabled mods/<mod-name>/", preserving relative paths.
package toggler

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/user/gta-mod-manager/internal/model"
	"github.com/user/gta-mod-manager/internal/procguard"
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
	}
	mod.Enabled = true
	// Best-effort cleanup of the now-empty disabled folder for this mod.
	_ = os.Remove(disabledRoot)
	return nil
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
