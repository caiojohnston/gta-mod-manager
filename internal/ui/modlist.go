// Package ui wires the internal packages into Fyne screens. modlist.go is the
// main screen: the mod list with on/off switches plus the toolbar and status
// bar (SPEC.md §4.3).
package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/caiojohnston/gta-mod-manager/internal/config"
	"github.com/caiojohnston/gta-mod-manager/internal/installer"
	"github.com/caiojohnston/gta-mod-manager/internal/model"
)

// App bundles the loaded config with the Fyne window so handlers can read and
// persist state without threading a pile of parameters everywhere.
type App struct {
	Win fyne.Window
	Cfg *config.Config

	modsBy map[string]*model.Mod
	order  []int // indexes into Cfg.Mods, display order (see sortedModsView)

	list      *widget.List
	statusBar *widget.Label
	emptyHint *widget.Label
	cleanBtn  *widget.Button

	// needGameDirButtons are disabled until a game folder is set.
	needGameDirButtons []*widget.Button
}

// NewApp builds the App. Call Win.SetContent(app.Build()).
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
	a.order = sortedModsView(a.Cfg.Mods)
}

// Build returns the root content for the main window.
func (a *App) Build() fyne.CanvasObject {
	a.list = widget.NewList(
		func() int { return len(a.order) },
		func() fyne.CanvasObject {
			check := widget.NewCheck("", nil)
			name := widget.NewLabel("mod name")
			name.TextStyle = fyne.TextStyle{Bold: true}
			sub := widget.NewLabel("details")
			sub.Importance = widget.LowImportance
			uninstall := widget.NewButtonWithIcon("", theme.DeleteIcon(), nil)
			uninstall.Importance = widget.LowImportance
			return container.NewBorder(
				nil, nil,
				check,
				uninstall,
				container.NewVBox(name, sub),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			mod := &a.Cfg.Mods[a.order[id]]
			row := obj.(*fyne.Container)
			check := row.Objects[1].(*widget.Check)
			uninstall := row.Objects[2].(*widget.Button)
			box := row.Objects[0].(*fyne.Container)
			name := box.Objects[0].(*widget.Label)
			sub := box.Objects[1].(*widget.Label)

			name.SetText(mod.Name)
			sub.SetText(modSubtitle(mod))

			modID := mod.ID
			check.OnChanged = nil
			check.SetChecked(mod.Enabled)
			check.OnChanged = func(want bool) {
				if m := a.modsBy[modID]; m != nil && want != m.Enabled {
					a.toggleMod(m, want)
				}
			}

			uninstall.OnTapped = func() {
				if m := a.modsBy[modID]; m != nil {
					a.uninstallMod(m)
				}
			}
		},
	)

	// --- toolbar --------------------------------------------------------
	detectBtn := widget.NewButton("Detect game folder", a.detectGameDir)
	setDirBtn := widget.NewButton("Set folder manually...", a.handleSetGameDir)
	installBtn := widget.NewButton("Install mod...", a.handleInstall)
	rescanBtn := widget.NewButton("Rescan folder", func() { a.scanAndImport(true) })
	profilesBtn := widget.NewButton("Profiles...", func() { ShowProfilesPanel(a) })

	enableAllBtn := widget.NewButton("Enable all", a.enableAll)
	disableAllBtn := widget.NewButton("Disable all", a.disableAll)
	a.cleanBtn = widget.NewButton("Launch Clean", a.launchClean)

	shvBtn := widget.NewButton("Install ScriptHookV", func() {
		a.ShowInstallBaseTool(installer.ScriptHookVPreset(), a.Cfg.Downloads.ScriptHookVURL)
	})
	openRPFBtn := widget.NewButton("Install OpenRPF", func() {
		a.ShowInstallBaseTool(installer.OpenRPFPreset(), a.Cfg.Downloads.OpenRPFURL)
	})
	runBtn := widget.NewButtonWithIcon("Run GTA", theme.MediaPlayIcon(), a.runGTA)
	runBtn.Importance = widget.HighImportance

	a.needGameDirButtons = []*widget.Button{
		installBtn, rescanBtn, profilesBtn, enableAllBtn, disableAllBtn,
		shvBtn, openRPFBtn, runBtn,
	}

	topRow := container.NewHBox(detectBtn, setDirBtn, installBtn, rescanBtn, profilesBtn)
	baseRow := container.NewHBox(
		widget.NewLabelWithStyle("Base:", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}),
		shvBtn, openRPFBtn,
		widget.NewSeparator(),
		runBtn,
	)
	bulkRow := container.NewHBox(
		widget.NewLabelWithStyle("Bulk:", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}),
		enableAllBtn, disableAllBtn, a.cleanBtn,
	)
	toolbar := container.NewVBox(topRow, baseRow, bulkRow, widget.NewSeparator())

	// --- status bar --------------------------------------------------
	a.statusBar = widget.NewLabel("")
	a.statusBar.Truncation = fyne.TextTruncateEllipsis
	statusBar := container.NewVBox(widget.NewSeparator(), a.statusBar)

	// --- empty hint ------------------------------------------------
	a.emptyHint = widget.NewLabel("")
	a.emptyHint.Alignment = fyne.TextAlignCenter

	content := container.NewStack(a.list, container.NewCenter(a.emptyHint))

	a.refreshStatus()
	a.updateButtonStates()
	a.refreshEmptyHint()

	return container.NewBorder(toolbar, statusBar, nil, nil, content)
}

func modSubtitle(m *model.Mod) string {
	state := "disabled"
	if m.Enabled {
		state = "enabled"
	}
	src := m.Source
	if src == "" {
		src = "installed"
	}
	return fmt.Sprintf("%s · %d file(s) · %s", state, len(m.Files), src)
}

func (a *App) refreshEmptyHint() {
	if a.emptyHint == nil {
		return
	}
	switch {
	case a.Cfg.GameDir == "":
		a.emptyHint.SetText("Start by choosing your GTA V folder:\nclick \"Detect game folder\" above.")
		a.emptyHint.Show()
	case len(a.Cfg.Mods) == 0:
		a.emptyHint.SetText("No mods yet.\nUse \"Install mod...\" to add one from a .zip,\nor \"Rescan folder\" to import mods already in the game folder.")
		a.emptyHint.Show()
	default:
		a.emptyHint.Hide()
	}
}

func (a *App) handleInstall() { ShowInstallWizard(a) }

func (a *App) handleSetGameDir() {
	dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
		if path, ok := fyneURIToPath(uri, err); ok {
			a.setGameDir(path)
		}
	}, a.Win)
}

// Refresh re-renders everything after mods or settings change elsewhere.
func (a *App) Refresh() {
	a.reindex()
	a.refreshStatus()
	a.updateButtonStates()
	a.refreshEmptyHint()
	if a.list != nil {
		a.list.Refresh()
	}
}
