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

// DestinationPath returns where relPath should land under the game root for
// a given classification. KindOther has no destination — callers must not
// install it without an explicit user override.
func DestinationPath(relPath string, kind model.FileKind) (string, bool) {
	base := path.Base(relPath)
	switch kind {
	case model.KindRoot:
		return base, true
	case model.KindScripts:
		// Preserve any sub-path the archive already had under scripts/,
		// otherwise just drop the loose file straight into scripts/.
		if idx := strings.Index(strings.ToLower(relPath), "scripts/"); idx >= 0 {
			return relPath[idx:], true
		}
		return path.Join("scripts", base), true
	case model.KindMods:
		if idx := strings.Index(strings.ToLower(relPath), "mods/"); idx >= 0 {
			return relPath[idx:], true
		}
		return path.Join("mods", base), true
	default:
		return "", false
	}
}
