package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/user/gta-mod-manager/internal/config"
	"github.com/user/gta-mod-manager/internal/installer"
)

// ShowInstallWizard implements SPEC.md §4.2 step 3: let the user review and
// approve/override each classified file before anything is copied.
func ShowInstallWizard(a *App) {
	if a.Cfg.GameDir == "" {
		dialog.ShowInformation("No game folder set", "Set the game folder first.", a.Win)
		return
	}

	dialog.ShowFileOpen(func(uri fyne.URIReadCloser, err error) {
		if err != nil || uri == nil {
			return
		}
		archivePath := uri.URI().Path()
		uri.Close()

		staging, files, cleanup, err := installer.Inspect(archivePath, a.Cfg.Rules)
		if err != nil {
			dialog.ShowError(err, a.Win)
			return
		}
		showReviewList(a, staging, archivePath, files, cleanup)
	}, a.Win)
}

func showReviewList(a *App, staging, source string, files []installer.ProposedFile, cleanup func()) {
	list := widget.NewList(
		func() int { return len(files) },
		func() fyne.CanvasObject {
			return container.NewBorder(nil, nil,
				widget.NewCheck("", func(bool) {}),
				nil,
				widget.NewLabel("path -> dest"),
			)
		},
		func(i widget.ListItemID, obj fyne.CanvasObject) {
			f := &files[i]
			row := obj.(*fyne.Container)
			check := row.Objects[0].(*widget.Check)
			label := row.Objects[1].(*widget.Label)

			dest := f.Dest
			if dest == "" {
				dest = "(unresolved — won't be installed unless approved)"
			}
			label.SetText(fmt.Sprintf("%s  ->  %s", f.RelInArchive, dest))
			check.SetChecked(f.Approved)
			check.OnChanged = func(v bool) { f.Approved = v }
		},
	)

	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Mod name")

	var d dialog.Dialog
	confirmBtn := widget.NewButton("Install approved files", func() {
		name := nameEntry.Text
		if name == "" {
			name = source
		}
		mod, err := installer.Commit(a.Cfg.GameDir, name, source, files)
		cleanup()
		if err != nil {
			dialog.ShowError(err, a.Win)
			return
		}
		a.Cfg.Mods = append(a.Cfg.Mods, mod)
		if err := config.Save(a.Cfg); err != nil {
			dialog.ShowError(err, a.Win)
		}
		a.Refresh()
		d.Hide()
	})

	content := container.NewBorder(nameEntry, confirmBtn, nil, nil, list)
	d = dialog.NewCustom("Review install", "Cancel", content, a.Win)
	d.SetOnClosed(cleanup)
	d.Resize(fyne.NewSize(600, 500))
	d.Show()
	_ = staging // kept for future conflict-detection stretch goal (PLAN.md open questions)
}
