package ui

import (
	"fmt"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/caiojohnston/gta-mod-manager/internal/config"
	"github.com/caiojohnston/gta-mod-manager/internal/installer"
	"github.com/caiojohnston/gta-mod-manager/internal/model"
)

// Destination choices offered per file in the review screen. "Auto" keeps
// whatever the rule table decided; the rest are explicit user overrides
// (SPEC.md §4.2 step 3 and §G6 — an unresolved file is shown, never guessed).
const (
	destAuto    = "Auto (rule table)"
	destRoot    = "Game root"
	destScripts = "scripts/"
	destMods    = "mods/"
	destSkip    = "Don't install"
)

var destOptions = []string{destAuto, destRoot, destScripts, destMods, destSkip}

// ShowInstallWizard implements SPEC.md §4.2: pick a .zip or folder, review the
// classified file list, override anything, then copy the approved files in.
func ShowInstallWizard(a *App) {
	if a.guardGameDir() != nil {
		return
	}

	pickFolder := func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if path, ok := fyneURIToPath(uri, err); ok {
				a.startInstall(path)
			}
		}, a.Win)
	}
	pickZip := func() {
		dialog.ShowFileOpen(func(uri fyne.URIReadCloser, err error) {
			if err != nil || uri == nil {
				return
			}
			p := uri.URI().Path()
			uri.Close()
			a.startInstall(p)
		}, a.Win)
	}

	var choose dialog.Dialog
	zipBtn := widget.NewButton("From a .zip file", func() { choose.Hide(); pickZip() })
	zipBtn.Importance = widget.HighImportance
	folderBtn := widget.NewButton("From a folder", func() { choose.Hide(); pickFolder() })
	body := container.NewVBox(
		widget.NewLabel("Install a mod from a .zip archive, or from an already-extracted folder?"),
		zipBtn, folderBtn,
	)
	choose = dialog.NewCustom("Install a mod", "Cancel", body, a.Win)
	choose.Show()
}

func (a *App) startInstall(archiveOrFolder string) {
	staging, files, cleanup, err := installer.Inspect(archiveOrFolder, a.Cfg.Rules)
	if err != nil {
		dialog.ShowError(fmt.Errorf("couldn't read %q: %w", archiveOrFolder, err), a.Win)
		return
	}
	_ = staging
	a.showReviewList(archiveOrFolder, files, cleanup)
}

func (a *App) showReviewList(source string, files []installer.ProposedFile, cleanup func()) {
	var once sync.Once
	cleanupOnce := func() { once.Do(cleanup) }

	// selection[i] is the dropdown label currently shown for files[i].
	selection := make([]string, len(files))
	for i := range files {
		selection[i] = destAuto
	}

	var list *widget.List
	list = widget.NewList(
		func() int { return len(files) },
		func() fyne.CanvasObject {
			check := widget.NewCheck("", nil)
			sel := widget.NewSelect(destOptions, nil)
			label := widget.NewLabel("path")
			return container.NewBorder(nil, nil, check, sel, label)
		},
		func(i widget.ListItemID, obj fyne.CanvasObject) {
			f := &files[i]
			row := obj.(*fyne.Container)
			check := row.Objects[0].(*widget.Check)
			label := row.Objects[1].(*widget.Label)
			sel := row.Objects[2].(*widget.Select)

			label.SetText(fmt.Sprintf("%s  →  %s", f.RelInArchive, destSummary(f)))

			check.OnChanged = nil
			check.SetChecked(f.Approved)
			idx := i
			check.OnChanged = func(v bool) { files[idx].Approved = v }

			sel.OnChanged = nil
			sel.SetSelected(selection[i])
			sel.OnChanged = func(choice string) {
				selection[idx] = choice
				applyDestChoice(&files[idx], a.Cfg.Rules, choice)
				label.SetText(fmt.Sprintf("%s  →  %s", files[idx].RelInArchive, destSummary(&files[idx])))
				list.Refresh()
			}
		},
	)

	nameEntry := widget.NewEntry()
	nameEntry.SetText(defaultModName(source))
	nameEntry.SetPlaceHolder("Mod name")

	approved := func() int {
		n := 0
		for _, f := range files {
			if f.Approved && f.Dest != "" {
				n++
			}
		}
		return n
	}

	var d dialog.Dialog
	confirmBtn := widget.NewButton("Install approved files", func() {
		name := nameEntry.Text
		if name == "" {
			name = defaultModName(source)
		}
		if approved() == 0 {
			dialog.ShowInformation("Nothing selected", "Tick at least one file (with a destination) to install.", a.Win)
			return
		}
		mod, err := installer.Commit(a.Cfg.GameDir, name, source, files)
		cleanupOnce()
		if err != nil {
			dialog.ShowError(fmt.Errorf("install failed: %w", err), a.Win)
			return
		}
		a.Cfg.Mods = append(a.Cfg.Mods, mod)
		a.persist()
		a.Refresh()
		d.Hide()
		dialog.ShowInformation("Installed", fmt.Sprintf("%q added with %d file(s).", mod.Name, len(mod.Files)), a.Win)
	})
	confirmBtn.Importance = widget.HighImportance

	header := container.NewVBox(
		widget.NewForm(widget.NewFormItem("Name", nameEntry)),
		widget.NewLabelWithStyle("Review file placement — untick extras, or override a destination:",
			fyne.TextAlignLeading, fyne.TextStyle{Italic: true}),
	)
	content := container.NewBorder(header, confirmBtn, nil, nil, list)
	d = dialog.NewCustom("Review install", "Cancel", content, a.Win)
	d.SetOnClosed(cleanupOnce)
	d.Resize(fyne.NewSize(680, 520))
	d.Show()
}

func destSummary(f *installer.ProposedFile) string {
	if f.Dest == "" {
		return "(no destination — won't be installed)"
	}
	return f.Dest
}

func defaultModName(source string) string {
	base := source
	for i := len(base) - 1; i >= 0; i-- {
		if base[i] == '/' || base[i] == '\\' {
			base = base[i+1:]
			break
		}
	}
	for _, ext := range []string{".zip", ".ZIP", ".Zip"} {
		if len(base) > len(ext) && base[len(base)-len(ext):] == ext {
			return base[:len(base)-len(ext)]
		}
	}
	return base
}

// applyDestChoice rewrites a ProposedFile's Kind/Dest/Approved to match the
// dropdown selection the user made.
func applyDestChoice(f *installer.ProposedFile, rules []config.RulePattern, choice string) {
	switch choice {
	case destSkip:
		f.Approved = false
		f.Dest = ""
		f.Kind = model.KindOther
		return
	case destAuto:
		f.Kind = installer.Classify(f.RelInArchive, rules)
	case destRoot:
		f.Kind = model.KindRoot
	case destScripts:
		f.Kind = model.KindScripts
	case destMods:
		f.Kind = model.KindMods
	}
	dest, ok := installer.DestinationPath(f.RelInArchive, f.Kind)
	f.Dest = dest
	f.Approved = ok
}
