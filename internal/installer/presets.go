package installer

import (
	"path"
	"strings"

	"github.com/caiojohnston/gta-mod-manager/internal/model"
)

// Preset pre-fills the install review screen for a well-known base tool
// (ScriptHookV, OpenRPF) so the user gets a correct, named, one-click install
// instead of classifying every file by hand.
type Preset struct {
	// Name is the mod name the install is recorded under.
	Name string
	// PageURL is the official download page, opened in a browser when the user
	// doesn't have the archive yet.
	PageURL string
	// ZipNameHints are lower-cased substrings used to auto-spot the archive in
	// the user's Downloads folder.
	ZipNameHints []string
	// Apply rewrites a freshly-Inspected file list: approve the files that make
	// up the tool, drop the rest (proxy loaders, readmes, .lib).
	Apply func(files []ProposedFile)
}

// ScriptHookVPreset installs Alexander Blade's ScriptHookV: the ASI loader
// (dinput8.dll) + ScriptHookV.dll in the game root, plus the bundled
// NativeTrainer.asi. .lib and .txt are skipped.
func ScriptHookVPreset() Preset {
	root := map[string]bool{
		"scripthookv.dll":   true,
		"dinput8.dll":       true,
		"nativetrainer.asi": true,
	}
	return Preset{
		Name:         "ScriptHookV",
		PageURL:      "http://www.dev-c.com/gtav/scripthookv/",
		ZipNameHints: []string{"scripthookv", "script hook v"},
		Apply: func(files []ProposedFile) {
			for i := range files {
				base := strings.ToLower(path.Base(files[i].RelInArchive))
				if root[base] {
					files[i].Kind = model.KindRoot
					files[i].Dest = path.Base(files[i].RelInArchive)
					files[i].Approved = true
				} else {
					files[i].Approved = false
				}
			}
		},
	}
}

// OpenRPFPreset installs OpenRPF: only OpenIV.asi goes into the game root. The
// proxy DLLs it also ships (dsound.dll / dinput8.dll) are left unticked — the
// ASI loader is expected to come from ScriptHookV, and installing another
// proxy DLL on top would clobber it. The user can still tick one if they run
// OpenRPF standalone.
func OpenRPFPreset() Preset {
	return Preset{
		Name:         "OpenRPF",
		PageURL:      "https://www.gta5-mods.com/tools/openrpf-openiv-asi-for-gta-v-enhanced",
		ZipNameHints: []string{"openrpf", "open rpf", "openiv.asi"},
		Apply: func(files []ProposedFile) {
			for i := range files {
				base := strings.ToLower(path.Base(files[i].RelInArchive))
				if base == "openiv.asi" {
					files[i].Kind = model.KindRoot
					files[i].Dest = "OpenIV.asi"
					files[i].Approved = true
				} else {
					files[i].Approved = false
				}
			}
		},
	}
}
