// Package dlclist edits a loose dlclist.xml so add-on dlcpacks the app just
// copied into mods/update/x64/dlcpacks/<name>/ actually get loaded.
//
// dlclist.xml normally lives inside update.rpf, but with OpenRPF a loose copy
// at mods/update/update.rpf/common/data/dlclist.xml overrides it, and it is
// plain text the game parses directly — so a line insert is all that's needed.
// This package never creates the file from nothing (its vanilla contents live
// in the RPF); it only amends an existing loose copy.
package dlclist

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// RelPath is where the loose dlclist.xml is expected, relative to the game dir.
const RelPath = "mods/update/update.rpf/common/data/dlclist.xml"

// PackNames returns the distinct dlcpack folder names referenced by relPaths
// (forward-slash, game-relative), e.g. "vremastered" from
// "mods/update/x64/dlcpacks/vremastered/dlc.rpf".
func PackNames(relPaths []string) []string {
	seen := map[string]bool{}
	var out []string
	re := regexp.MustCompile(`(?i)dlcpacks/([^/]+)/`)
	for _, p := range relPaths {
		if m := re.FindStringSubmatch(p); m != nil {
			name := m[1]
			if !seen[name] {
				seen[name] = true
				out = append(out, name)
			}
		}
	}
	return out
}

// Item is the line a dlcpack needs in dlclist.xml.
func Item(pack string) string {
	return fmt.Sprintf("<Item>dlcpacks:/%s/</Item>", pack)
}

// LoosePath returns the absolute path of the loose dlclist.xml for gameDir.
func LoosePath(gameDir string) string {
	return filepath.Join(gameDir, filepath.FromSlash(RelPath))
}

// Status describes what EnsureItems can do for a game folder.
type Status int

const (
	// StatusMissing: no mods/update/update.rpf at all — nothing to amend yet.
	StatusMissing Status = iota
	// StatusEditable: a loose dlclist.xml exists and can be amended in place.
	StatusEditable
	// StatusPackedRPF: mods/update/update.rpf is a real packed RPF (OpenIV
	// style), so dlclist.xml must be edited inside it with OpenIV/CodeWalker.
	StatusPackedRPF
)

// Check reports how the dlclist.xml for gameDir can be reached.
func Check(gameDir string) Status {
	if _, err := os.Stat(LoosePath(gameDir)); err == nil {
		return StatusEditable
	}
	// mods/update/update.rpf present but not as a directory tree => packed.
	rpf := filepath.Join(gameDir, "mods", "update", "update.rpf")
	if st, err := os.Stat(rpf); err == nil && !st.IsDir() {
		return StatusPackedRPF
	}
	return StatusMissing
}

// EnsureItems inserts an <Item> line for each pack (that isn't already listed)
// immediately before the closing </Paths> tag. Returns the packs it added.
// Errors if the loose dlclist.xml is missing or has no </Paths>.
func EnsureItems(gameDir string, packs []string) ([]string, error) {
	path := LoosePath(gameDir)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("dlclist: %w (extract %s from update.rpf once with CodeWalker/OpenIV)", err, RelPath)
	}
	text := string(data)

	idx := strings.LastIndex(text, "</Paths>")
	if idx < 0 {
		return nil, fmt.Errorf("dlclist: %s has no </Paths> — is it a real dlclist.xml?", RelPath)
	}

	// Indentation of the </Paths> line, reused for the inserted items.
	lineStart := strings.LastIndex(text[:idx], "\n") + 1
	indent := text[lineStart:idx]

	var added []string
	var insert strings.Builder
	for _, p := range packs {
		item := Item(p)
		if strings.Contains(text, item) || strings.Contains(text, fmt.Sprintf("dlcpacks:/%s/", p)) {
			continue
		}
		insert.WriteString(indent)
		insert.WriteString("  ")
		insert.WriteString(item)
		insert.WriteString("\n")
		added = append(added, p)
	}
	if len(added) == 0 {
		return nil, nil
	}

	newText := text[:lineStart] + insert.String() + text[lineStart:]
	if err := os.WriteFile(path, []byte(newText), 0o644); err != nil {
		return nil, err
	}
	return added, nil
}
