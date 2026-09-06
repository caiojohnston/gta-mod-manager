// Command gta-mod-manager is a native GUI (Fyne) app for installing and
// toggling GTA V Enhanced mods. See docs/SPEC.md and docs/PLAN.md.
package main

import (
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"github.com/caiojohnston/gta-mod-manager/internal/config"
	"github.com/caiojohnston/gta-mod-manager/internal/ui"
)

// version is stamped into the window title; bump on release.
const version = "v1.0.0"

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	a := app.New()
	win := a.NewWindow("GTA V Enhanced Mod Manager " + version)

	appUI := ui.NewApp(win, cfg)
	win.SetContent(appUI.Build())
	win.Resize(fyne.NewSize(760, 560))

	// First-run work: auto-detect the game folder if it isn't set, otherwise
	// rescan the known folder for mods added outside the app since last launch
	// (SPEC.md §4.1). Dialogs opened here are queued by Fyne and shown once the
	// event loop starts.
	appUI.QueueFirstRun()

	win.ShowAndRun()
}
