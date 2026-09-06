// Command gta-mod-manager is a native GUI (Fyne) app for installing and
// toggling GTA V Enhanced mods. See docs/SPEC.md and docs/PLAN.md.
package main

import (
	"log"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/dialog"

	"github.com/user/gta-mod-manager/internal/config"
	"github.com/user/gta-mod-manager/internal/model"
	"github.com/user/gta-mod-manager/internal/scanner"
	"github.com/user/gta-mod-manager/internal/ui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	a := app.New()
	win := a.NewWindow("GTA V Enhanced Mod Manager")

	appUI := ui.NewApp(win, cfg)
	win.SetContent(appUI.Build())
	win.Resize(fyne.NewSize(700, 500))

	// First-run scan (SPEC.md §4.1): only runs if a game dir is already set
	// and there's at least a chance of finding something new.
	if cfg.GameDir != "" {
		found, err := scanner.FindUnmanaged(cfg.GameDir, cfg.Rules, cfg.Mods)
		if err != nil {
			log.Printf("scan on startup failed (non-fatal): %v", err)
		} else if len(found) > 0 {
			cfg.Mods = append(cfg.Mods, found...)
			if err := config.Save(cfg); err != nil {
				log.Printf("failed saving newly scanned mods: %v", err)
			}
			appUI.Refresh()
			dialog.ShowInformation(
				"Imported existing mods",
				modsSummary(found),
				win,
			)
		}
	}

	win.ShowAndRun()
}

func modsSummary(found []model.Mod) string {
	names := make([]string, len(found))
	for i, m := range found {
		names[i] = m.Name
	}
	return "Found and imported:\n" + strings.Join(names, "\n")
}
