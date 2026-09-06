package ui

import (
	"errors"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/caiojohnston/gta-mod-manager/internal/procguard"
	"github.com/caiojohnston/gta-mod-manager/internal/profiles"
)

// ShowProfilesPanel implements SPEC.md §4.4: save the current enabled set as a
// named profile, switch to a saved one (diff-based, only the delta moves), or
// delete one.
func ShowProfilesPanel(a *App) {
	if a.guardGameDir() != nil {
		return
	}

	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("New profile name")

	var d dialog.Dialog
	var profileList *widget.List

	saveBtn := widget.NewButton("Save current mods as profile", func() {
		name := nameEntry.Text
		if name == "" {
			dialog.ShowInformation("Name needed", "Type a name for the profile first.", a.Win)
			return
		}
		p := profiles.CurrentAsProfile(name, a.modPtrs())
		// Replace an existing profile of the same name rather than duplicating.
		replaced := false
		for i := range a.Cfg.Profiles {
			if a.Cfg.Profiles[i].Name == name {
				a.Cfg.Profiles[i] = p
				replaced = true
				break
			}
		}
		if !replaced {
			a.Cfg.Profiles = append(a.Cfg.Profiles, p)
		}
		nameEntry.SetText("")
		a.persist()
		profileList.Refresh()
	})

	profileList = a.newProfileList(func() { profileList.Refresh() })

	hint := widget.NewLabelWithStyle(
		"A profile is a saved on/off set. Applying one moves only what differs.",
		fyne.TextAlignLeading, fyne.TextStyle{Italic: true})

	content := container.NewBorder(
		container.NewVBox(container.NewBorder(nil, nil, nil, saveBtn, nameEntry), hint, widget.NewSeparator()),
		nil, nil, nil, profileList)
	d = dialog.NewCustom("Profiles", "Close", content, a.Win)
	d.Resize(fyne.NewSize(460, 440))
	d.Show()
}

// newProfileList builds the saved-profiles list (apply / delete per row).
// Standalone so a headless test can render a row. afterMutate is called after a
// delete so the caller can refresh the list widget.
func (a *App) newProfileList(afterMutate func()) *widget.List {
	return widget.NewList(
		func() int { return len(a.Cfg.Profiles) },
		func() fyne.CanvasObject {
			apply := widget.NewButtonWithIcon("apply", theme.ConfirmIcon(), nil)
			del := widget.NewButtonWithIcon("", theme.DeleteIcon(), nil)
			del.Importance = widget.LowImportance
			name := widget.NewLabel("profile")
			// NewBorder puts the center object first in .Objects, then each
			// non-nil border in top/bottom/left/right order — so this row is
			// [name, buttonBox]. Keep that in sync with the update func below.
			return container.NewBorder(nil, nil, nil, container.NewHBox(apply, del), name)
		},
		func(i widget.ListItemID, obj fyne.CanvasObject) {
			p := a.Cfg.Profiles[i]
			row := obj.(*fyne.Container)
			name := row.Objects[0].(*widget.Label)
			btns := row.Objects[1].(*fyne.Container)
			apply := btns.Objects[0].(*widget.Button)
			del := btns.Objects[1].(*widget.Button)

			name.SetText(fmt.Sprintf("%s  (%d mod(s))", p.Name, len(p.EnabledModIDs)))

			idx := i
			apply.OnTapped = func() {
				prof := a.Cfg.Profiles[idx]
				if err := profiles.Switch(a.Cfg.GameDir, a.Cfg.GameExeName, a.modPtrs(), prof); err != nil {
					var running *procguard.ErrGameRunning
					if errors.As(err, &running) {
						dialog.ShowError(err, a.Win)
					} else {
						dialog.ShowError(fmt.Errorf("couldn't switch to %q: %w", prof.Name, err), a.Win)
					}
					a.Refresh()
					return
				}
				a.Cfg.PreCleanModIDs = nil // an explicit profile switch ends any clean run
				a.persist()
				a.Refresh()
				dialog.ShowInformation("Profile applied", "Now running: "+prof.Name, a.Win)
			}
			del.OnTapped = func() {
				prof := a.Cfg.Profiles[idx]
				dialog.ShowConfirm("Delete profile?", "Delete "+prof.Name+"? (Mods and files are untouched.)",
					func(ok bool) {
						if !ok {
							return
						}
						a.Cfg.Profiles = append(a.Cfg.Profiles[:idx], a.Cfg.Profiles[idx+1:]...)
						a.persist()
						if afterMutate != nil {
							afterMutate()
						}
					}, a.Win)
			}
		},
	)
}
