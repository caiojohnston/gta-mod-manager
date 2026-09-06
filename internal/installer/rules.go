// Package installer implements SPEC.md §4.2: turning a zip archive or folder
// into a reviewable, classified list of files, then copying the ones the
// user approves into the game folder.
package installer

import (
	"path"
	"strings"

	"github.com/caiojohnston/gta-mod-manager/internal/config"
	"github.com/caiojohnston/gta-mod-manager/internal/model"
)

// Classify applies the rule table to a single relative path (forward-slash
// separated, as produced by zip archives) and returns the matched kind.
// Falls back to model.KindOther when nothing matches — per SPEC.md G6, an
// unmatched file must be surfaced for manual placement, never guessed.
func Classify(relPath string, rules []config.RulePattern) model.FileKind {
	base := path.Base(relPath)
	lower := strings.ToLower(relPath)

	for _, r := range rules {
		switch {
		case strings.HasSuffix(r.Match, "/"):
			// Path-prefix rule, e.g. "scripts/": matches if the path already
			// lives under that folder in the archive.
			if strings.Contains(lower, "/"+strings.ToLower(r.Match)) ||
				strings.HasPrefix(lower, strings.ToLower(r.Match)) {
				return r.Destination
			}
		case strings.HasPrefix(r.Match, "*."):
			// Extension glob, e.g. "*.asi".
			ext := strings.ToLower(r.Match[1:]) // ".asi"
			if strings.HasSuffix(strings.ToLower(base), ext) {
				return r.Destination
			}
		default:
			// Exact filename match, e.g. "dinput8.dll".
			if strings.EqualFold(base, r.Match) {
				return r.Destination
			}
		}
	}
	return model.KindOther
}

// modAnchors are the folder names that mark where the OpenRPF/OpenIV "mods/"
// mirror of the game's virtual filesystem begins inside a content-mod archive.
// The path from the first anchor onward is preserved under the game's mods/.
var modAnchors = []string{"mods/", "update/", "dlcpacks/", "common/", "platform/", "x64/"}

// DestinationPath returns where relPath should land under the game root for
// a given classification, plus whether it could be resolved at all. KindOther
// and an unanchored KindMods return ("", false) — the caller must get an
// explicit destination from the user (SPEC.md §G6), e.g. via KindCustom.
func DestinationPath(relPath string, kind model.FileKind) (string, bool) {
	base := path.Base(relPath)
	lower := strings.ToLower(relPath)

	switch kind {
	case model.KindRoot:
		return base, true

	case model.KindScripts:
		// Preserve any sub-path the archive already had under scripts/,
		// otherwise just drop the loose file straight into scripts/.
		if idx := strings.Index(lower, "scripts/"); idx >= 0 {
			return relPath[idx:], true
		}
		return path.Join("scripts", base), true

	case model.KindMods:
		// Already under a literal mods/ folder: keep the path as-is.
		if idx := strings.Index(lower, "mods/"); idx >= 0 {
			return relPath[idx:], true
		}
		// Content that mirrors the game's virtual FS (common/, dlcpacks/, ...):
		// preserve everything from the first anchor onward, under mods/. Checked
		// before the bare-.rpf case so a dlcpacks/foo/dlc.rpf keeps its folder.
		if a := firstAnchorIndex(lower); a >= 0 {
			return path.Join("mods", relPath[a:]), true
		}
		// A .rpf sitting loose in the archive with no anchor: mirror its path
		// under mods/, dropping just the wrapper folder.
		if strings.HasSuffix(lower, ".rpf") {
			return path.Join("mods", stripWrapperDir(relPath)), true
		}
		// No anchor — we can't know where a bare .ymt/.meta belongs. The user
		// has to say (Custom path in the wizard).
		return "", false

	case model.KindCustom:
		// Destination was set directly by the user elsewhere; nothing to derive.
		return "", false

	default:
		return "", false
	}
}

// AutoApprove reports whether a classified file is safe to pre-tick in the
// review screen. Loose files with a firm home (root, scripts/) and content
// already laid out under a real mods/ folder are; anything placed by the
// content-mod heuristics is shown unchecked so the user confirms the path.
func AutoApprove(relPath string, kind model.FileKind) bool {
	switch kind {
	case model.KindRoot, model.KindScripts:
		return true
	case model.KindMods:
		lower := strings.ToLower(relPath)
		return strings.HasPrefix(lower, "mods/") || strings.Contains(lower, "/mods/")
	default:
		return false
	}
}

func firstAnchorIndex(lower string) int {
	best := -1
	for _, a := range modAnchors {
		idx := -1
		if strings.HasPrefix(lower, a) {
			idx = 0
		} else if i := strings.Index(lower, "/"+a); i >= 0 {
			idx = i + 1
		}
		if idx >= 0 && (best < 0 || idx < best) {
			best = idx
		}
	}
	return best
}

// stripWrapperDir drops a single leading directory ("Cool Mod v1/foo/bar" ->
// "foo/bar") so a mod that ships everything inside one wrapper folder doesn't
// carry that folder into the game.
func stripWrapperDir(relPath string) string {
	if i := strings.IndexByte(relPath, '/'); i >= 0 && i < len(relPath)-1 {
		return relPath[i+1:]
	}
	return relPath
}
