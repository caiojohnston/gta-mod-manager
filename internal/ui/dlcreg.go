package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/caiojohnston/gta-mod-manager/internal/dlclist"
	"github.com/caiojohnston/gta-mod-manager/internal/model"
)

// offerDlcRegistration is called right after installing a mod: if it placed a
// dlcpack under mods/update/x64/dlcpacks/<name>/, offer to add the matching
// <Item> line to the loose dlclist.xml (add-on cars/graphics need this).
func (a *App) offerDlcRegistration(mod model.Mod) {
	rels := make([]string, len(mod.Files))
	for i, f := range mod.Files {
		rels[i] = f.RelPath
	}
	packs := dlclist.PackNames(rels)
	if len(packs) == 0 {
		return
	}
	a.registerPacks(packs)
}

// registerDlcPacksFromAll scans every tracked mod for dlcpack names and offers
// to register any that aren't in dlclist.xml yet. Wired to a toolbar button.
func (a *App) registerDlcPacksFromAll() {
	if a.guardGameDir() != nil {
		return
	}
	var rels []string
	for _, m := range a.Cfg.Mods {
		for _, f := range m.Files {
			rels = append(rels, f.RelPath)
		}
	}
	packs := dlclist.PackNames(rels)
	if len(packs) == 0 {
		dialog.ShowInformation("No dlcpacks", "No dlcpack folders found under mods/update/x64/dlcpacks/.", a.Win)
		return
	}
	a.registerPacks(packs)
}

func (a *App) registerPacks(packs []string) {
	lines := "  " + strings.Join(itemLines(packs), "\n  ")

	switch dlclist.Check(a.Cfg.GameDir) {
	case dlclist.StatusEditable:
		dialog.ShowConfirm("Register dlcpack(s)?",
			fmt.Sprintf("Add to %s:\n\n%s\n\n(lines already present are skipped)", dlclist.RelPath, lines),
			func(ok bool) {
				if !ok {
					return
				}
				added, err := dlclist.EnsureItems(a.Cfg.GameDir, packs)
				if err != nil {
					dialog.ShowError(err, a.Win)
					return
				}
				if len(added) == 0 {
					dialog.ShowInformation("Already registered", "Every dlcpack was already in dlclist.xml.", a.Win)
					return
				}
				dialog.ShowInformation("dlclist.xml updated",
					"Added:\n  "+strings.Join(itemLines(added), "\n  ")+
						"\n\nAlso make sure mods/update/update.rpf exists (copy it from update/ if not).",
					a.Win)
			}, a.Win)

	case dlclist.StatusPackedRPF:
		msg := widget.NewLabel(fmt.Sprintf(
			"The dlc.rpf file(s) are in place. To load them, add:\n\n%s\n\n"+
				"Your mods/update/update.rpf is a packed RPF (OpenIV installed it),\n"+
				"so edit dlclist.xml inside it with OpenIV:\n"+
				"  1. OpenIV → pick the GTA V folder, turn on Edit mode\n"+
				"  2. Go to mods/update/update.rpf/common/data/dlclist.xml\n"+
				"  3. Edit → add the line(s) above before </Paths> → Ctrl+S\n\n"+
				"(The app can only edit a loose dlclist.xml, not one inside a packed RPF.)",
			lines))
		dialog.ShowCustom("Edit dlclist.xml in OpenIV", "OK", msg, a.Win)

	case dlclist.StatusMissing:
		msg := widget.NewLabel(fmt.Sprintf(
			"The dlc.rpf file(s) are in place, but they won't load until this line\n"+
				"is in dlclist.xml:\n\n%s\n\n"+
				"That file lives inside update.rpf. One-time step:\n"+
				"  1. CodeWalker/OpenIV → extract\n     update/update.rpf/common/data/dlclist.xml\n"+
				"  2. Put it at %s\n"+
				"  3. Also copy update/update.rpf to mods/update/update.rpf if missing\n"+
				"Then use \"Register dlcpacks\" again and the app edits it for you.",
			lines, dlclist.RelPath))
		dialog.ShowCustom("dlclist.xml not found", "OK", msg, a.Win)
	}
}

func itemLines(packs []string) []string {
	out := make([]string, len(packs))
	for i, p := range packs {
		out[i] = dlclist.Item(p)
	}
	return out
}
