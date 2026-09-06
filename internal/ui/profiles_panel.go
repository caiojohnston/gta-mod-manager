package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/user/gta-mod-manager/internal/config"
	"github.com/user/gta-mod-manager/internal/model"
	"github.com/user/gta-mod-manager/internal/profiles"
)

// ShowProfilesPanel implements SPEC.md §4.4: save the current enabled set as
// a named profile, or switch to a previously saved one.
func ShowProfilesPanel(a *App) {
	modPtrs := make([]*model.Mod, len(a.Cfg.Mods))
	for i := range a.Cfg.Mods {
		modPtrs[i] = &a.Cfg.Mods[i]
	}

	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Profile name")
	saveBtn := widget.NewButton("Save current as profile", func() {
		if nameEntry.Text == "" {
			return
		}
		p := profiles.CurrentAsProfile(nameEntry.Text, modPtrs)
		a.Cfg.Profiles = append(a.Cfg.Profiles, p)
		_ = config.Save(a.Cfg)
		a.Refresh()
	})

	var profileList *widget.List
	profileList = widget.NewList(
		func() int { return len(a.Cfg.Profiles) },
		func() fyne.CanvasObject { return widget.NewButton("profile", func() {}) },
		func(i widget.ListItemID, obj fyne.CanvasObject) {
			p := a.Cfg.Profiles[i]
			btn := obj.(*widget.Button)
			btn.SetText(p.Name)
			btn.OnTapped = func() {
				if err := profiles.Switch(a.Cfg.GameDir, a.Cfg.GameExeName, modPtrs, p); err != nil {
					dialog.ShowError(err, a.Win)
					return
				}
				_ = config.Save(a.Cfg)
				a.Refresh()
			}
		},
	)

	content := container.NewBorder(container.NewHBox(nameEntry, saveBtn), nil, nil, nil, profileList)
	d := dialog.NewCustom("Profiles", "Close", content, a.Win)
	d.Resize(fyne.NewSize(400, 400))
	d.Show()
}
