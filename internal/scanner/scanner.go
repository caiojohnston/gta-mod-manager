// Package scanner implements SPEC.md §4.1: detect files that look like a
// pre-existing manual mod install (whether dropped in by hand or by OpenIV's
// package installer) and import them as tracked mods, so nothing already
// installed is invisible to the toggle/profile features.
package scanner

import (
	"os"
	"path/filepath"

	"github.com/caiojohnston/gta-mod-manager/internal/config"
	"github.com/caiojohnston/gta-mod-manager/internal/installer"
	"github.com/caiojohnston/gta-mod-manager/internal/model"
	"github.com/google/uuid"
)

// knownRootFiles are exact filenames in the game root that are never part of
// the vanilla install and should always be treated as a mod if present.
var knownRootFiles = map[string]bool{
	"dinput8.dll":     true,
	"scripthookv.dll": true,
	"dsound.dll":      true,
}

// FindUnmanaged scans gameDir for mod-like content not already referenced by
// existingMods: loose root files/DLLs, the scripts/ folder as one unit, and —
// so an OpenIV "install to mods folder" is manageable here — one entry per
// immediate child of mods/ (mods/update, mods/x64, mods/common.rpf, …).
func FindUnmanaged(gameDir string, rules []config.RulePattern, existingMods []model.Mod) ([]model.Mod, error) {
	tracked := make(map[string]bool)
	for _, m := range existingMods {
		for _, f := range m.Files {
			tracked[filepath.ToSlash(f.RelPath)] = true
		}
	}

	entries, err := os.ReadDir(gameDir)
	if err != nil {
		return nil, err
	}

	var found []model.Mod
	for _, e := range entries {
		name := e.Name()
		lower := filepath.ToSlash(name)

		if e.IsDir() && lower == "mods" {
			children, err := importModsChildren(gameDir, rules, tracked)
			if err != nil {
				return nil, err
			}
			found = append(found, children...)
			continue
		}
		if e.IsDir() && lower == "scripts" {
			mod, err := importFolderAsMod(gameDir, name, "scanned", rules, tracked)
			if err != nil {
				return nil, err
			}
			if len(mod.Files) > 0 {
				found = append(found, mod)
			}
			continue
		}
		if e.IsDir() {
			continue // saves, update, x64, … — not a mod pattern, leave alone
		}
		if tracked[lower] {
			continue
		}
		if knownRootFiles[filepathLower(name)] || filepath.Ext(name) == ".asi" {
			found = append(found, model.Mod{
				ID:      uuid.NewString(),
				Name:    name,
				Enabled: true,
				Source:  "scanned",
				Files:   []model.ModFile{{RelPath: name, Kind: model.KindRoot}},
			})
		}
	}
	return found, nil
}

// importModsChildren makes one Mod per immediate child of mods/ so a merged
// mods/ tree (OpenIV packages + hand installs) can still be toggled in useful
// chunks rather than all-or-nothing.
func importModsChildren(gameDir string, rules []config.RulePattern, tracked map[string]bool) ([]model.Mod, error) {
	modsRoot := filepath.Join(gameDir, "mods")
	kids, err := os.ReadDir(modsRoot)
	if err != nil {
		return nil, err
	}
	var out []model.Mod
	for _, k := range kids {
		rel := "mods/" + filepath.ToSlash(k.Name())
		mod := model.Mod{
			ID:      uuid.NewString(),
			Name:    rel,
			Enabled: true,
			Source:  "scanned (mods/)",
		}
		if k.IsDir() {
			if err := walkInto(gameDir, filepath.Join(modsRoot, k.Name()), rules, tracked, &mod); err != nil {
				return nil, err
			}
		} else if !tracked[rel] {
			mod.Files = append(mod.Files, model.ModFile{
				RelPath: rel, Kind: installer.Classify(rel, rules),
			})
		}
		if len(mod.Files) > 0 {
			out = append(out, mod)
		}
	}
	return out, nil
}

func importFolderAsMod(gameDir, folderName, source string, rules []config.RulePattern, tracked map[string]bool) (model.Mod, error) {
	mod := model.Mod{
		ID:      uuid.NewString(),
		Name:    folderName,
		Enabled: true,
		Source:  source,
	}
	err := walkInto(gameDir, filepath.Join(gameDir, folderName), rules, tracked, &mod)
	return mod, err
}

// walkInto adds every untracked file under root to mod.Files, path relative to
// gameDir.
func walkInto(gameDir, root string, rules []config.RulePattern, tracked map[string]bool, mod *model.Mod) error {
	return filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(gameDir, p)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)
		if tracked[relSlash] {
			return nil
		}
		mod.Files = append(mod.Files, model.ModFile{
			RelPath: relSlash, Kind: installer.Classify(relSlash, rules),
		})
		return nil
	})
}

func filepathLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}
