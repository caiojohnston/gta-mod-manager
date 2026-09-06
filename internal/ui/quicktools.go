package ui

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/caiojohnston/gta-mod-manager/internal/installer"
	"github.com/caiojohnston/gta-mod-manager/internal/launcher"
)

// ShowInstallBaseTool drives the one-click install of a base modding tool
// (ScriptHookV / OpenRPF). The official pages have no download API, so the flow
// is: use a configured direct URL if there is one, otherwise let the user open
// the page and pick the .zip — then the preset pre-fills and names the install.
func (a *App) ShowInstallBaseTool(preset installer.Preset, directURL string) {
	if a.guardGameDir() != nil {
		return
	}

	runPreset := func(archive string) {
		staging, files, cleanup, err := installer.Inspect(archive, a.Cfg.Rules)
		if err != nil {
			dialog.ShowError(fmt.Errorf("couldn't read %q: %w", archive, err), a.Win)
			return
		}
		_ = staging
		preset.Apply(files)
		a.showReviewList(preset.Name, files, cleanup)
	}

	var d dialog.Dialog
	body := container.NewVBox()

	// Row 1: a .zip already sitting in Downloads?
	if found := findDownloadedArchive(preset.ZipNameHints); found != "" {
		useBtn := widget.NewButton("Use "+filepath.Base(found), func() { d.Hide(); runPreset(found) })
		useBtn.Importance = widget.HighImportance
		body.Add(useBtn)
	}

	// Row 2: direct download, only when a URL is configured.
	if directURL != "" {
		body.Add(widget.NewButton("Download & install automatically", func() {
			d.Hide()
			a.downloadThenInstall(preset.Name, directURL, runPreset)
		}))
	}

	// Row 3: open the official page.
	body.Add(widget.NewButton("Open download page", func() {
		if u, err := url.Parse(preset.PageURL); err == nil {
			_ = fyne.CurrentApp().OpenURL(u)
		}
	}))

	// Row 4: pick a .zip manually.
	body.Add(widget.NewButton("Choose a .zip…", func() {
		d.Hide()
		dialog.ShowFileOpen(func(rc fyne.URIReadCloser, err error) {
			if err != nil || rc == nil {
				return
			}
			p := rc.URI().Path()
			rc.Close()
			runPreset(p)
		}, a.Win)
	}))

	msg := widget.NewLabel(fmt.Sprintf(
		"%s is a base tool other mods depend on.\nDownload it, then install — the placement is filled in for you.",
		preset.Name))
	d = dialog.NewCustom("Install "+preset.Name, "Cancel", container.NewVBox(msg, widget.NewSeparator(), body), a.Win)
	d.Show()
}

// downloadThenInstall fetches url into a temp file and hands it to install.
func (a *App) downloadThenInstall(name, rawURL string, install func(string)) {
	dialog.ShowConfirm("Download "+name+"?",
		"Fetch from:\n"+rawURL+"\n\nThe file is saved to a temp folder and installed.",
		func(ok bool) {
			if !ok {
				return
			}
			prog := dialog.NewCustomWithoutButtons("Downloading "+name+"…", widget.NewProgressBarInfinite(), a.Win)
			prog.Show()
			go func() {
				path, err := downloadArchive(rawURL)
				prog.Hide()
				if err != nil {
					dialog.ShowError(fmt.Errorf("download failed: %w", err), a.Win)
					return
				}
				install(path)
			}()
		}, a.Win)
}

func downloadArchive(rawURL string) (string, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(rawURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned %s", resp.Status)
	}

	name := filepath.Base(rawURL)
	if !strings.HasSuffix(strings.ToLower(name), ".zip") {
		name = "download.zip"
	}
	out, err := os.CreateTemp("", "gtamod-dl-*-"+name)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return "", err
	}
	return out.Name(), nil
}

// findDownloadedArchive looks in the user's Downloads folder for a .zip whose
// name contains any of the hints, returning the newest match.
func findDownloadedArchive(hints []string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, "Downloads")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var best string
	var bestMod time.Time
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		lname := strings.ToLower(e.Name())
		if !strings.HasSuffix(lname, ".zip") {
			continue
		}
		match := false
		for _, h := range hints {
			if strings.Contains(lname, h) {
				match = true
				break
			}
		}
		if !match {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if best == "" || info.ModTime().After(bestMod) {
			best, bestMod = filepath.Join(dir, e.Name()), info.ModTime()
		}
	}
	return best
}

// runGTA launches the game through the platform launcher (PlayGTAV.exe).
func (a *App) runGTA() {
	if a.guardGameDir() != nil {
		return
	}
	exe, err := launcher.Launch(a.Cfg.GameDir)
	if err != nil {
		dialog.ShowError(err, a.Win)
		return
	}
	enabled := 0
	for _, m := range a.Cfg.Mods {
		if m.Enabled {
			enabled++
		}
	}
	dialog.ShowInformation("Launching GTA V",
		fmt.Sprintf("Started %s\n%d mod(s) enabled.\n\nReminder: mods are Story Mode only — never GTA Online.",
			filepath.Base(exe), enabled),
		a.Win)
}
