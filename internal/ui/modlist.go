// Package ui wires the internal packages into Fyne screens. modlist.go is
// the main screen: the mod list with on/off switches (SPEC.md §4.3).
package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/user/gta-mod-manager/internal/config"
	"github.com/user/gta-mod-manager/internal/model"
	"github.com/user/gta-mod-manager/internal/procguard"
	"github.com/user/gta-mod-manager/internal/toggler"
)

// App bundles the loaded config with the Fyne window so handlers can read
// and persist state without a pile of parameters everywhere.
type App struct {
	Win    fyne.Window
	Cfg    *config.Config
	modsBy map[string]*model.Mod
	list   *widget.List
}

// NewApp builds the main window content. Call Win.SetContent(app.Build()).
func NewApp(win fyne.Window, cfg *config.Config) *App {
	a := &App{Win: win, Cfg: cfg}
	a.reindex()
	return a
}

func (a *App) reindex() {
	a.modsBy = make(map[string]*model.Mod, len(a.Cfg.Mods))
	for i := range a.Cfg.Mods {
		a.modsBy[a.Cfg.Mods[i].ID] = &a.Cfg.Mods[i]
	}
}

// Build returns the root content for the main window.
func (a *App) Build() fyne.CanvasObject {
	a.list = widget.NewList(
		func() int { return len(a.Cfg.Mods) },
		func() fyne.CanvasObject {
			return container.NewBorder(nil, nil, nil,
				widget.NewCheck("", func(bool) {}),
				widget.NewLabel("mod name"),
			)
		},
		func(i widget.ListItemID, obj fyne.CanvasObject) {
			mod := &a.Cfg.Mods[i]
			row := obj.(*fyne.Container)
			label := row.Objects[0].(*widget.Label)
			check := row.Objects[1].(*widget.Check)

			label.SetText(mod.Name)
			check.SetChecked(mod.Enabled)
			check.OnChanged = func(want bool) {
				a.handleToggle(mod, want, check)
			}
		},
	)

	installBtn := widget.NewButton("Install mod...", a.handleInstall)
	gameDirBtn := widget.NewButton("Set game folder...", a.handleSetGameDir)
	profilesBtn := widget.NewButton("Profiles...", func() { ShowProfilesPanel(a) })

	toolbar := container.NewHBox(installBtn, gameDirBtn, profilesBtn)
	return container.NewBorder(toolbar, nil, nil, nil, a.list)
}

func (a *App) handleToggle(mod *model.Mod, want bool, check *widget.Check) {
	var err error
	if want {
		err = toggler.Enable(a.Cfg.GameDir, a.Cfg.GameExeName, mod)
	} else {
		err = toggler.Disable(a.Cfg.GameDir, a.Cfg.GameExeName, mod)
	}
	if err != nil {
		check.SetChecked(mod.Enabled) // revert the visual state
		if _, ok := err.(*procguard.ErrGameRunning); ok {
			dialog.ShowError(err, a.Win)
			return
		}
		dialog.ShowError(fmt.Errorf("couldn't toggle %q: %w", mod.Name, err), a.Win)
		return
	}
	if err := config.Save(a.Cfg); err != nil {
		dialog.ShowError(fmt.Errorf("toggled, but failed to save config: %w", err), a.Win)
	}
}

func (a *App) handleInstall() {
	// Wired up in install_wizard.go; kept separate so this file stays focused
	// on the list screen per PLAN.md's package layout.
	ShowInstallWizard(a)
}

func (a *App) handleSetGameDir() {
	dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil || uri == nil {
			return
		}
		a.Cfg.GameDir = uri.Path()
		if err := config.Save(a.Cfg); err != nil {
			dialog.ShowError(err, a.Win)
		}
	}, a.Win)
}

// Refresh re-renders the list after mods are added/removed elsewhere
// (e.g. after the install wizard commits a new mod).
func (a *App) Refresh() {
	a.reindex()
	if a.list != nil {
		a.list.Refresh()
	}
}
