package ui

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/caiojohnston/gta-mod-manager/internal/config"
	"github.com/caiojohnston/gta-mod-manager/internal/gamedetect"
	"github.com/caiojohnston/gta-mod-manager/internal/model"
	"github.com/caiojohnston/gta-mod-manager/internal/procguard"
	"github.com/caiojohnston/gta-mod-manager/internal/scanner"
	"github.com/caiojohnston/gta-mod-manager/internal/toggler"
)

// persist saves the config and surfaces any failure. Every handler that
// mutates a.Cfg calls this instead of config.Save directly.
func (a *App) persist() {
	if err := config.Save(a.Cfg); err != nil {
		dialog.ShowError(fmt.Errorf("couldn't save your settings: %w", err), a.Win)
	}
}

// modPtrs returns pointers into a.Cfg.Mods, the form the toggler/profiles
// packages expect (they flip Mod.Enabled in place).
func (a *App) modPtrs() []*model.Mod {
	out := make([]*model.Mod, len(a.Cfg.Mods))
	for i := range a.Cfg.Mods {
		out[i] = &a.Cfg.Mods[i]
	}
	return out
}

// setGameDir records a newly chosen install folder, then scans it for
// pre-existing manual installs so nothing already there is invisible to the
// app (SPEC.md §4.1).
func (a *App) setGameDir(path string) {
	a.Cfg.GameDir = path
	a.persist()
	a.refreshStatus()
	a.updateButtonStates()
	a.scanAndImport(false)
}

// QueueFirstRun is called from main once the UI is built: detect the game
// folder on a fresh install, or rescan an already-configured one.
func (a *App) QueueFirstRun() {
	if a.Cfg.GameDir == "" {
		a.detectGameDir()
		return
	}
	a.scanAndImport(false)
}

// detectGameDir runs the automatic probe (registry / Steam / Epic / common
// paths) and either sets the folder outright, asks the user to pick between
// several, or falls back to the manual picker.
func (a *App) detectGameDir() {
	candidates := gamedetect.Detect()

	switch len(candidates) {
	case 0:
		dialog.ShowInformation("No install found automatically",
			"Couldn't find GTA V automatically. Use \"Set folder manually...\" and point at the folder that contains GTA5_Enhanced.exe.",
			a.Win)
	case 1:
		c := candidates[0]
		dialog.ShowConfirm("Use this folder?",
			fmt.Sprintf("Found via %s:\n\n%s\n(%s)\n\nUse it?", c.Source, c.Path, c.Exe),
			func(ok bool) {
				if ok {
					a.setGameDir(c.Path)
				}
			}, a.Win)
	default:
		a.showCandidatePicker(candidates)
	}
}

func (a *App) showCandidatePicker(candidates []gamedetect.Candidate) {
	labels := make([]string, len(candidates))
	for i, c := range candidates {
		labels[i] = fmt.Sprintf("%s  —  %s (%s)", c.Path, c.Source, c.Exe)
	}
	radio := widget.NewRadioGroup(labels, nil)
	radio.SetSelected(labels[0])

	dialog.ShowCustomConfirm("Pick your GTA V folder", "Use this one", "Cancel",
		radio, func(ok bool) {
			if !ok || radio.Selected == "" {
				return
			}
			for i, l := range labels {
				if l == radio.Selected {
					a.setGameDir(candidates[i].Path)
					return
				}
			}
		}, a.Win)
}

// scanAndImport looks for mod-like files in the game folder that aren't tracked
// yet and imports them. showToastIfEmpty controls whether a "nothing new"
// message appears (wanted after a manual rescan, not on every startup).
func (a *App) scanAndImport(showToastIfEmpty bool) {
	if a.Cfg.GameDir == "" {
		return
	}
	found, err := scanner.FindUnmanaged(a.Cfg.GameDir, a.Cfg.Rules, a.Cfg.Mods)
	if err != nil {
		dialog.ShowError(fmt.Errorf("couldn't scan the game folder: %w", err), a.Win)
		return
	}
	if len(found) == 0 {
		if showToastIfEmpty {
			dialog.ShowInformation("Scan complete", "No new mods found — everything in the folder is already tracked.", a.Win)
		}
		return
	}
	a.Cfg.Mods = append(a.Cfg.Mods, found...)
	a.persist()
	a.Refresh()

	names := make([]string, len(found))
	for i, m := range found {
		names[i] = "• " + m.Name
	}
	dialog.ShowInformation("Imported existing mods",
		fmt.Sprintf("Found and started tracking %d item(s):\n\n%s", len(found), strings.Join(names, "\n")),
		a.Win)
}

// toggleMod enables or disables a single mod and persists the result.
func (a *App) toggleMod(mod *model.Mod, want bool) {
	if err := a.guardGameDir(); err != nil {
		a.Refresh()
		return
	}
	var err error
	if want {
		err = toggler.Enable(a.Cfg.GameDir, a.Cfg.GameExeName, mod)
	} else {
		err = toggler.Disable(a.Cfg.GameDir, a.Cfg.GameExeName, mod)
	}
	if err != nil {
		a.reportToggleErr(err, mod.Name)
		a.Refresh() // snap the checkbox back to the real state
		return
	}
	a.persist()
	a.Refresh()
}

// enableAll / disableAll apply the toggler across every mod, stopping at the
// first error (typically the game being open) rather than spamming dialogs.
func (a *App) enableAll()  { a.bulkToggle(true) }
func (a *App) disableAll() { a.bulkToggle(false) }

func (a *App) bulkToggle(enable bool) {
	if err := a.guardGameDir(); err != nil {
		return
	}
	for _, m := range a.modPtrs() {
		var err error
		if enable {
			err = toggler.Enable(a.Cfg.GameDir, a.Cfg.GameExeName, m)
		} else {
			err = toggler.Disable(a.Cfg.GameDir, a.Cfg.GameExeName, m)
		}
		if err != nil {
			a.reportToggleErr(err, m.Name)
			break
		}
	}
	a.persist()
	a.Refresh()
}

// launchClean disables every currently-enabled mod for this play session,
// remembering which ones to bring back (SPEC.md §4.3). The remembered set is
// persisted so closing the app mid-clean-run doesn't lose it.
func (a *App) launchClean() {
	if err := a.guardGameDir(); err != nil {
		return
	}
	if len(a.Cfg.PreCleanModIDs) > 0 {
		return // already in a clean run
	}
	var remembered []string
	for _, m := range a.modPtrs() {
		if !m.Enabled {
			continue
		}
		if err := toggler.Disable(a.Cfg.GameDir, a.Cfg.GameExeName, m); err != nil {
			a.reportToggleErr(err, m.Name)
			// roll back whatever we already disabled so state stays coherent
			a.restoreSpecific(remembered)
			a.persist()
			a.Refresh()
			return
		}
		remembered = append(remembered, m.ID)
	}
	a.Cfg.PreCleanModIDs = remembered
	a.persist()
	a.Refresh()
	dialog.ShowInformation("Clean run ready",
		fmt.Sprintf("Disabled %d mod(s) for this session. Launch the game now.\nHit \"Restore mods\" when you're done to turn them back on.", len(remembered)),
		a.Win)
}

func (a *App) restoreMods() {
	if err := a.guardGameDir(); err != nil {
		return
	}
	a.restoreSpecific(a.Cfg.PreCleanModIDs)
	a.Cfg.PreCleanModIDs = nil
	a.persist()
	a.Refresh()
}

func (a *App) restoreSpecific(ids []string) {
	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	for _, m := range a.modPtrs() {
		if want[m.ID] && !m.Enabled {
			if err := toggler.Enable(a.Cfg.GameDir, a.Cfg.GameExeName, m); err != nil {
				a.reportToggleErr(err, m.Name)
				return
			}
		}
	}
}

// uninstallMod deletes a mod's files (after confirmation) and drops it from
// the registry. This is the only path that removes files rather than moving
// them (SPEC.md §G5 / §4.3).
func (a *App) uninstallMod(mod *model.Mod) {
	dialog.ShowConfirm("Uninstall "+mod.Name+"?",
		fmt.Sprintf("This permanently deletes %d file(s) this mod placed. It can't be undone.", len(mod.Files)),
		func(ok bool) {
			if !ok {
				return
			}
			if err := a.guardGameDir(); err != nil {
				return
			}
			if err := toggler.Uninstall(a.Cfg.GameDir, a.Cfg.GameExeName, mod); err != nil {
				a.reportToggleErr(err, mod.Name)
				return
			}
			a.Cfg.Mods = removeModByID(a.Cfg.Mods, mod.ID)
			a.Cfg.PreCleanModIDs = removeString(a.Cfg.PreCleanModIDs, mod.ID)
			a.persist()
			a.Refresh()
		}, a.Win)
}

// guardGameDir returns a non-nil error (after showing a dialog) when no game
// folder is set, so callers can bail before touching the filesystem.
func (a *App) guardGameDir() error {
	if a.Cfg.GameDir == "" {
		dialog.ShowInformation("No game folder", "Set your GTA V folder first.", a.Win)
		return errors.New("no game dir")
	}
	return nil
}

func (a *App) reportToggleErr(err error, modName string) {
	var running *procguard.ErrGameRunning
	if errors.As(err, &running) {
		dialog.ShowError(err, a.Win)
		return
	}
	dialog.ShowError(fmt.Errorf("couldn't update %q: %w", modName, err), a.Win)
}

func (a *App) refreshStatus() {
	if a.statusBar == nil {
		return
	}
	if a.Cfg.GameDir == "" {
		a.statusBar.SetText("Game folder: not set")
	} else {
		a.statusBar.SetText("Game folder: " + a.Cfg.GameDir)
	}
}

// updateButtonStates greys out actions that need a game folder until one is set,
// and flips the clean/restore button between its two modes.
func (a *App) updateButtonStates() {
	hasDir := a.Cfg.GameDir != ""
	for _, b := range a.needGameDirButtons {
		if b == nil {
			continue
		}
		if hasDir {
			b.Enable()
		} else {
			b.Disable()
		}
	}
	if a.cleanBtn != nil {
		if len(a.Cfg.PreCleanModIDs) > 0 {
			a.cleanBtn.SetText("Restore mods")
			a.cleanBtn.OnTapped = a.restoreMods
		} else {
			a.cleanBtn.SetText("Launch Clean")
			a.cleanBtn.OnTapped = a.launchClean
		}
		if hasDir {
			a.cleanBtn.Enable()
		} else {
			a.cleanBtn.Disable()
		}
	}
}

// sortedModsView returns the mods in a stable display order: enabled first,
// then alphabetical. Purely cosmetic; the underlying slice keeps insertion
// order so indexes stay valid elsewhere.
func sortedModsView(mods []model.Mod) []int {
	idx := make([]int, len(mods))
	for i := range mods {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		ma, mb := mods[idx[a]], mods[idx[b]]
		if ma.Enabled != mb.Enabled {
			return ma.Enabled
		}
		return strings.ToLower(ma.Name) < strings.ToLower(mb.Name)
	})
	return idx
}

func removeModByID(mods []model.Mod, id string) []model.Mod {
	out := mods[:0]
	for _, m := range mods {
		if m.ID != id {
			out = append(out, m)
		}
	}
	return out
}

func removeString(s []string, v string) []string {
	out := s[:0]
	for _, x := range s {
		if x != v {
			out = append(out, x)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// fyneURIToPath is a tiny helper so the two folder-open callbacks don't repeat
// the nil/err dance.
func fyneURIToPath(uri fyne.ListableURI, err error) (string, bool) {
	if err != nil || uri == nil {
		return "", false
	}
	return uri.Path(), true
}
