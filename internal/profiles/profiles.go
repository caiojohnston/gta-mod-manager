// Package profiles implements SPEC.md §4.4: named sets of "which mods are
// enabled", switchable with a single diff-based move rather than a full
// disable-everything-then-enable-everything pass.
package profiles

import (
	"fmt"

	"github.com/user/gta-mod-manager/internal/model"
	"github.com/user/gta-mod-manager/internal/toggler"
)

// Switch moves the mod set in mods from its current enabled state to match
// profile's EnabledModIDs, touching only the mods whose state actually
// changes. gameExeName is passed through to the toggler's process guard.
func Switch(gameDir, gameExeName string, mods []*model.Mod, profile model.Profile) error {
	want := make(map[string]bool, len(profile.EnabledModIDs))
	for _, id := range profile.EnabledModIDs {
		want[id] = true
	}

	for _, m := range mods {
		shouldBeEnabled := want[m.ID]
		switch {
		case shouldBeEnabled && !m.Enabled:
			if err := toggler.Enable(gameDir, gameExeName, m); err != nil {
				return fmt.Errorf("profiles: enabling %q for profile %q: %w", m.Name, profile.Name, err)
			}
		case !shouldBeEnabled && m.Enabled:
			if err := toggler.Disable(gameDir, gameExeName, m); err != nil {
				return fmt.Errorf("profiles: disabling %q for profile %q: %w", m.Name, profile.Name, err)
			}
		}
	}
	return nil
}

// CurrentAsProfile snapshots which mod IDs are presently enabled, for the
// "save current state as profile" UI action.
func CurrentAsProfile(name string, mods []*model.Mod) model.Profile {
	p := model.Profile{Name: name}
	for _, m := range mods {
		if m.Enabled {
			p.EnabledModIDs = append(p.EnabledModIDs, m.ID)
		}
	}
	return p
}
