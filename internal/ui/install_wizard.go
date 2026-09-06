package ui

import (
	"fmt"
	"path"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/caiojohnston/gta-mod-manager/internal/config"
	"github.com/caiojohnston/gta-mod-manager/internal/installer"
	"github.com/caiojohnston/gta-mod-manager/internal/model"
	"github.com/caiojohnston/gta-mod-manager/internal/oiv"
)

// Destination choices offered per file in the review screen. "Auto" keeps
// whatever the rule table decided; the rest are explicit user overrides
// (SPEC.md §4.2 step 3 and §G6 — an unresolved file is shown, never guessed).
const (
	destAuto    = "Auto (rule table)"
	destRoot    = "Game root"
	destScripts = "scripts/"
	destMods    = "mods/ (preserve path)"
	destCustom  = "Custom path…"
	destSkip    = "Don't install"
)

var destOptions = []string{destAuto, destRoot, destScripts, destMods, destCustom, destSkip}

// junkExts never get auto-approved by "Set base path for all" — readmes,
// screenshots, videos that ship alongside the actual mod files.
var junkExts = map[string]bool{
	".txt": true, ".md": true, ".pdf": true, ".png": true, ".jpg": true,
	".jpeg": true, ".gif": true, ".webp": true, ".url": true, ".ini": false,
}

// ShowInstallWizard implements SPEC.md §4.2: pick a .zip or folder, review the
// classified file list, override anything, then copy the approved files in.
func ShowInstallWizard(a *App) {
	if a.guardGameDir() != nil {
		return
	}

	pickFolder := func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if p, ok := fyneURIToPath(uri, err); ok {
				a.startInstall(p)
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

// rejectUnsupported shows a helper dialog and returns true when path is an
// archive kind this app can't open (.rar/.7z). .oiv is handled natively, not
// rejected.
func (a *App) rejectUnsupported(p string) bool {
	switch strings.ToLower(path.Ext(p)) {
	case ".rar", ".7z":
		dialog.ShowInformation("Extract it first",
			"Only .zip archives and folders are supported.\n\nExtract this with 7-Zip or WinRAR, then use \"Install mod... → From a folder\".",
			a.Win)
		return true
	}
	return false
}

func (a *App) startInstall(archiveOrFolder string) {
	if a.rejectUnsupported(archiveOrFolder) {
		return
	}
	if strings.EqualFold(path.Ext(archiveOrFolder), ".oiv") {
		a.startOIVInstall(archiveOrFolder)
		return
	}

	staging, files, cleanup, err := installer.Inspect(archiveOrFolder, a.Cfg.Rules)
	if err != nil {
		dialog.ShowError(fmt.Errorf("couldn't read %q: %w", archiveOrFolder, err), a.Win)
		return
	}
	_ = staging

	// A .zip whose only real payload is a nested .oiv: point the user at the
	// .oiv directly (we install those, but not the double-wrapped case yet).
	oiv, real := 0, 0
	for _, f := range files {
		ext := strings.ToLower(path.Ext(f.RelInArchive))
		if ext == ".oiv" {
			oiv++
		} else if !junkExts[ext] {
			real++
		}
	}
	if oiv > 0 && real == 0 {
		cleanup()
		dialog.ShowInformation("It's an OpenIV package",
			"This .zip contains a .oiv package. Extract the .oiv, then use\n"+
				"\"Install mod... → From a .zip\" and pick the .oiv itself.",
			a.Win)
		return
	}

	a.showReviewList(archiveOrFolder, files, cleanup)
}

// startOIVInstall installs the ADD operations of an .oiv package as loose files
// under mods/ (OpenRPF reads them). XML-fragment merges / deletes it can't do
// are listed for the user; XML .ymt/.meta still trigger the compile warning.
func (a *App) startOIVInstall(oivPath string) {
	pkg, err := oiv.Read(oivPath)
	if err != nil {
		dialog.ShowError(err, a.Win)
		return
	}
	if len(pkg.Adds) == 0 {
		dialog.ShowInformation("Needs OpenIV",
			"This .oiv has no plain file-copy steps — it only does RPF/XML edits\n"+
				"this app can't perform. Use OpenIV → Tools → Package Installer.",
			a.Win)
		return
	}
	if err := pkg.Extract(); err != nil {
		dialog.ShowError(fmt.Errorf("reading .oiv contents: %w", err), a.Win)
		return
	}

	files := make([]installer.ProposedFile, 0, len(pkg.Adds))
	for _, op := range pkg.Adds {
		files = append(files, installer.ProposedFile{
			SourcePath:   op.ExtractedPath,
			RelInArchive: op.SourceName,
			Kind:         model.KindCustom,
			Dest:         op.ModsRelPath,
			Approved:     true,
		})
	}

	show := func() {
		name := pkg.Name
		if name == "" {
			name = defaultModName(oivPath)
		}
		a.showReviewList(name, files, pkg.Close)
	}

	if len(pkg.Unsupported) > 0 {
		dialog.ShowCustomConfirm("Some steps need OpenIV", "Install the rest", "Cancel",
			widget.NewLabel(fmt.Sprintf(
				"%d file(s) will be installed as loose overrides.\n\n"+
					"%d step(s) can't be done here (they edit inside RPFs):\n  %s\n\n"+
					"For full parity run the .oiv through OpenIV instead.",
				len(files), len(pkg.Unsupported), strings.Join(pkg.Unsupported, "\n  "))),
			func(ok bool) {
				if ok {
					show()
				} else {
					pkg.Close()
				}
			}, a.Win)
		return
	}
	show()
}

// review holds the mutable state of one install-review screen.
type review struct {
	a         *App
	rules     []config.RulePattern
	files     []installer.ProposedFile
	selection []string // dropdown label shown per file
	custom    []string // last custom path typed per file
	wrapper   string   // shared leading dir across the archive, stripped from guesses
	list      *widget.List
}

func (a *App) showReviewList(source string, files []installer.ProposedFile, cleanup func()) {
	var once sync.Once
	cleanupOnce := func() { once.Do(cleanup) }

	rc := &review{
		a:         a,
		rules:     a.Cfg.Rules,
		files:     files,
		selection: make([]string, len(files)),
		custom:    make([]string, len(files)),
		wrapper:   commonWrapperDir(files),
	}
	for i := range files {
		rc.selection[i] = destAuto
		if files[i].Kind == model.KindCustom {
			rc.selection[i] = destCustom
			rc.custom[i] = files[i].Dest
		}
	}
	rc.list = rc.newList()

	nameEntry := widget.NewEntry()
	nameEntry.SetText(defaultModName(source))
	nameEntry.SetPlaceHolder("Mod name")

	approved := func() int {
		n := 0
		for _, f := range rc.files {
			if f.Approved && f.Dest != "" {
				n++
			}
		}
		return n
	}

	var d dialog.Dialog
	doCommit := func(name string) {
		mod, err := installer.Commit(a.Cfg.GameDir, name, source, rc.files)
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
	}

	confirmBtn := widget.NewButton("Install approved files", func() {
		name := nameEntry.Text
		if name == "" {
			name = defaultModName(source)
		}
		if approved() == 0 {
			dialog.ShowInformation("Nothing selected", "Tick at least one file (with a destination) to install.", a.Win)
			return
		}
		// Warn about raw-XML meta files: the game only reads the compiled
		// binary, and this tool copies, it doesn't convert (needs CodeWalker).
		if raw := installer.UncompiledMeta(rc.files); len(raw) > 0 {
			dialog.ShowCustomConfirm("Uncompiled XML detected", "Install anyway", "Cancel",
				widget.NewLabel(fmt.Sprintf(
					"%d file(s) are editable XML, not the binary form GTA loads:\n\n  %s\n\n"+
						"Open them in CodeWalker (drag into the same mods/ path, accept the\n"+
						"conversion) or the change won't apply. Installing now just copies\n"+
						"the XML as-is.",
					len(raw), strings.Join(raw, "\n  "))),
				func(ok bool) {
					if ok {
						doCommit(name)
					}
				}, a.Win)
			return
		}
		doCommit(name)
	})
	confirmBtn.Importance = widget.HighImportance

	basePathBtn := widget.NewButton("Set base path for all…", rc.promptBasePath)

	header := container.NewVBox(
		widget.NewForm(widget.NewFormItem("Name", nameEntry)),
		container.NewBorder(nil, nil, nil, basePathBtn,
			widget.NewLabelWithStyle("Review placement — untick extras, or override a destination.",
				fyne.TextAlignLeading, fyne.TextStyle{Italic: true})),
		widget.NewLabelWithStyle("Content mods (RPF tree) need OpenRPF in the game folder to load.",
			fyne.TextAlignLeading, fyne.TextStyle{Italic: true}),
	)
	content := container.NewBorder(header, confirmBtn, nil, nil, rc.list)
	d = dialog.NewCustom("Review install", "Cancel", content, a.Win)
	d.SetOnClosed(cleanupOnce)
	d.Resize(fyne.NewSize(720, 560))
	d.Show()
}

// newList builds the per-file review list. Standalone (on *review) so a
// headless test can render a row and catch template/index mismatches.
func (rc *review) newList() *widget.List {
	list := widget.NewList(
		func() int { return len(rc.files) },
		func() fyne.CanvasObject {
			check := widget.NewCheck("", nil)
			sel := widget.NewSelect(destOptions, nil)
			label := widget.NewLabel("path")
			// NewBorder lists the center object first in .Objects, then each
			// non-nil border in top/bottom/left/right order — so this row is
			// [label, check, sel]. Keep the update func's indices in sync.
			return container.NewBorder(nil, nil, check, sel, label)
		},
		func(i widget.ListItemID, obj fyne.CanvasObject) {
			f := &rc.files[i]
			row := obj.(*fyne.Container)
			label := row.Objects[0].(*widget.Label)
			check := row.Objects[1].(*widget.Check)
			sel := row.Objects[2].(*widget.Select)

			label.SetText(fmt.Sprintf("%s  →  %s", f.RelInArchive, destSummary(f)))

			check.OnChanged = nil
			check.SetChecked(f.Approved)
			idx := i
			check.OnChanged = func(v bool) { rc.files[idx].Approved = v }

			sel.OnChanged = nil
			sel.SetSelected(rc.selection[i])
			sel.OnChanged = func(choice string) { rc.applyChoice(idx, choice) }
		},
	)
	return list
}

// applyChoice reacts to the per-row destination dropdown.
func (rc *review) applyChoice(i int, choice string) {
	rc.selection[i] = choice
	if choice == destCustom {
		rc.promptCustomPath(i)
		return
	}
	applyDestChoice(&rc.files[i], rc.rules, choice)
	rc.redraw()
}

func (rc *review) redraw() {
	if rc.list != nil {
		rc.list.Refresh()
	}
}

// promptCustomPath asks the user for an explicit destination, relative to the
// game folder, for one file (SPEC.md §G6 — the app never guesses this).
func (rc *review) promptCustomPath(i int) {
	f := &rc.files[i]
	guess := rc.custom[i]
	if guess == "" {
		guess = "mods/" + strings.TrimPrefix(f.RelInArchive, rc.wrapper)
	}
	entry := widget.NewEntry()
	entry.SetText(guess)
	entry.Validator = nil

	form := []*widget.FormItem{
		widget.NewFormItem("Path", entry),
	}
	dialog.ShowForm("Destination for "+path.Base(f.RelInArchive), "Use path", "Cancel", form,
		func(ok bool) {
			if !ok {
				rc.selection[i] = destAuto
				applyDestChoice(&rc.files[i], rc.rules, destAuto)
				rc.redraw()
				return
			}
			p := cleanRelDest(entry.Text)
			rc.custom[i] = p
			rc.files[i].Dest = p
			rc.files[i].Kind = model.KindCustom
			rc.files[i].Approved = p != ""
			rc.redraw()
		}, rc.a.Win)
}

// promptBasePath asks for one prefix and applies it to every non-junk file:
// <prefix>/<archive path minus the shared wrapper dir>. This is the fast path
// for content mods where the ReadMe says "everything goes under mods/…".
func (rc *review) promptBasePath() {
	entry := widget.NewEntry()
	entry.SetPlaceHolder("e.g. mods/update/update.rpf")
	entry.SetText("mods")
	info := widget.NewLabelWithStyle(
		"Applies <prefix>/<file path> to every file (readmes/images left unticked).",
		fyne.TextAlignLeading, fyne.TextStyle{Italic: true})

	dialog.ShowForm("Set base path for all files", "Apply", "Cancel",
		[]*widget.FormItem{widget.NewFormItem("Base path", entry), widget.NewFormItem("", info)},
		func(ok bool) {
			if !ok {
				return
			}
			prefix := cleanRelDest(entry.Text)
			for i := range rc.files {
				rel := strings.TrimPrefix(rc.files[i].RelInArchive, rc.wrapper)
				dest := path.Join(prefix, rel)
				rc.files[i].Dest = dest
				rc.files[i].Kind = model.KindCustom
				rc.files[i].Approved = !junkExts[strings.ToLower(path.Ext(rel))]
				rc.custom[i] = dest
				rc.selection[i] = destCustom
			}
			if rc.list != nil {
				rc.list.Refresh()
			}
		}, rc.a.Win)
}

func destSummary(f *installer.ProposedFile) string {
	if f.Dest == "" {
		return "(no destination — won't be installed)"
	}
	return f.Dest
}

// cleanRelDest normalises a user-typed destination: forward slashes, no
// leading slash, no "." / ".." segments (those are rejected again in
// installer.Commit, this just keeps the display sane).
func cleanRelDest(s string) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), "\\", "/")
	s = strings.TrimPrefix(s, "/")
	parts := make([]string, 0, strings.Count(s, "/")+1)
	for _, seg := range strings.Split(s, "/") {
		if seg == "" || seg == "." || seg == ".." {
			continue
		}
		parts = append(parts, seg)
	}
	return strings.Join(parts, "/")
}

// commonWrapperDir returns the single leading directory shared by every file in
// the archive (many mods ship inside one "Cool Mod v1/" folder), or "" if the
// files don't all share one.
func commonWrapperDir(files []installer.ProposedFile) string {
	if len(files) == 0 {
		return ""
	}
	first := files[0].RelInArchive
	slash := strings.IndexByte(first, '/')
	if slash < 0 {
		return ""
	}
	prefix := first[:slash+1]
	for _, f := range files {
		if !strings.HasPrefix(f.RelInArchive, prefix) {
			return ""
		}
	}
	return prefix
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

// applyDestChoice rewrites a ProposedFile's Kind/Dest/Approved to match a
// non-custom dropdown selection.
func applyDestChoice(f *installer.ProposedFile, rules []config.RulePattern, choice string) {
	explicit := true
	switch choice {
	case destSkip:
		f.Approved = false
		f.Dest = ""
		f.Kind = model.KindOther
		return
	case destAuto:
		f.Kind = installer.Classify(f.RelInArchive, rules)
		explicit = false
	case destRoot:
		f.Kind = model.KindRoot
	case destScripts:
		f.Kind = model.KindScripts
	case destMods:
		f.Kind = model.KindMods
	}
	dest, ok := installer.DestinationPath(f.RelInArchive, f.Kind)
	f.Dest = dest
	// An explicit dropdown pick is trusted; "Auto" defers to the cautious
	// AutoApprove heuristic so guessed content-mod paths stay unticked.
	f.Approved = ok && (explicit || installer.AutoApprove(f.RelInArchive, f.Kind))
}
