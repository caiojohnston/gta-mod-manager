// Package scanner implements SPEC.md §4.1: on first pointing the app at a
// game folder, detect files that look like a pre-existing manual mod install
// (same heuristic patterns as the reference tool the user found) and offer
// to import them as tracked mods, so nothing already installed is lost or
// duplicated on the next install/toggle.
package scanner

import (
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/user/gta-mod-manager/internal/config"
	"github.com/user/gta-mod-manager/internal/installer"
	"github.com/user/gta-mod-manager/internal/model"
)

// knownRootFiles are exact filenames in the game root that are never part of
// the vanilla install and should always be treated as a mod if present.
var knownRootFiles = map[string]bool{
	"dinput8.dll":     true,
	"scripthookv.dll": true,
}

// FindUnmanaged scans gameDir's top level for mod-like entries that aren't
// already referenced by any mod in existingMods, and returns one proposed
// Mod per top-level item (a single loose file, or a whole folder like
// scripts/ or mods/ as one unit — matching how most mods are distributed).
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

		if e.IsDir() && (lower == "scripts" || lower == "mods") {
			mod, err := importFolderAsMod(gameDir, name, rules, tracked)
			if err != nil {
				return nil, err
			}
			if len(mod.Files) > 0 {
				found = append(found, mod)
			}
			continue
		}
		if e.IsDir() {
			continue // some other folder (saves, etc.) — not a mod pattern, leave alone
		}
		if tracked[lower] {
			continue
		}
		isKnown := knownRootFiles[filepathLower(name)]
		isAsi := filepath.Ext(name) == ".asi"
		if isKnown || isAsi {
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

func importFolderAsMod(gameDir, folderName string, rules []config.RulePattern, tracked map[string]bool) (model.Mod, error) {
	mod := model.Mod{
		ID:      uuid.NewString(),
		Name:    folderName,
		Enabled: true,
		Source:  "scanned",
	}
	root := filepath.Join(gameDir, folderName)
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
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
		kind := installer.Classify(relSlash, rules)
		mod.Files = append(mod.Files, model.ModFile{RelPath: relSlash, Kind: kind})
		return nil
	})
	return mod, err
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
