//go:build windows

package gamedetect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// osProbe gathers unvalidated candidate folders from the Windows registry, the
// Steam library config, Epic's manifest files, and a list of default install
// locations. gamedetect.Detect filters these down to folders that actually
// contain a GTA V executable.
func osProbe() []rawCandidate {
	var out []rawCandidate
	out = append(out, probeRockstar()...)
	out = append(out, probeSteam()...)
	out = append(out, probeEpic()...)
	out = append(out, probeCommonPaths()...)
	return out
}

// --- Rockstar Games Launcher --------------------------------------------------

func probeRockstar() []rawCandidate {
	// Rockstar's key names vary by title and installer vintage ("GTA V Enhanced",
	// "Grand Theft Auto V", ...), and each can carry several "InstallFolder*"
	// values (InstallFolder, InstallFolderSteam, InstallFolderEnhanced). Rather
	// than guess, enumerate every subkey under "Rockstar Games" and harvest any
	// value whose name starts with "InstallFolder". Detect() drops the ones that
	// don't actually contain a GTA V executable.
	roots := []struct {
		root registry.Key
		path string
	}{
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Rockstar Games`},
		{registry.LOCAL_MACHINE, `SOFTWARE\Rockstar Games`},
		{registry.CURRENT_USER, `SOFTWARE\Rockstar Games`},
	}

	var out []rawCandidate
	for _, r := range roots {
		parent, err := registry.OpenKey(r.root, r.path, registry.ENUMERATE_SUB_KEYS)
		if err != nil {
			continue
		}
		subNames, _ := parent.ReadSubKeyNames(-1)
		parent.Close()

		for _, name := range subNames {
			lname := strings.ToLower(name)
			if !strings.Contains(lname, "gta") && !strings.Contains(lname, "grand theft auto") {
				continue
			}
			sub, err := registry.OpenKey(r.root, r.path+`\`+name, registry.QUERY_VALUE)
			if err != nil {
				continue
			}
			valNames, _ := sub.ReadValueNames(-1)
			for _, vn := range valNames {
				if !strings.HasPrefix(strings.ToLower(vn), "installfolder") {
					continue
				}
				if v, _, err := sub.GetStringValue(vn); err == nil && v != "" {
					out = append(out, rawCandidate{path: v, src: SourceRockstar})
				}
			}
			sub.Close()
		}
	}
	return out
}

// --- Steam ------------------------------------------------------------------

var vdfPathRe = regexp.MustCompile(`"(?:path|BaseInstallFolder_\d+)"\s+"([^"]+)"`)

func probeSteam() []rawCandidate {
	steamRoot := steamPath()
	if steamRoot == "" {
		return nil
	}

	libraries := []string{steamRoot}
	for _, vdf := range []string{
		filepath.Join(steamRoot, "steamapps", "libraryfolders.vdf"),
		filepath.Join(steamRoot, "config", "libraryfolders.vdf"),
	} {
		data, err := os.ReadFile(vdf)
		if err != nil {
			continue
		}
		for _, m := range vdfPathRe.FindAllStringSubmatch(string(data), -1) {
			libraries = append(libraries, strings.ReplaceAll(m[1], `\\`, `\`))
		}
	}

	// GTA V Enhanced ships in "Grand Theft Auto V Enhanced"; Legacy in
	// "Grand Theft Auto V". Check both under every library.
	folders := []string{"Grand Theft Auto V Enhanced", "Grand Theft Auto V"}
	var out []rawCandidate
	for _, lib := range libraries {
		for _, f := range folders {
			out = append(out, rawCandidate{
				path: filepath.Join(lib, "steamapps", "common", f),
				src:  SourceSteam,
			})
		}
	}
	return out
}

func steamPath() string {
	for _, k := range []struct {
		root registry.Key
		path string
		val  string
	}{
		{registry.CURRENT_USER, `Software\Valve\Steam`, "SteamPath"},
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Valve\Steam`, "InstallPath"},
		{registry.LOCAL_MACHINE, `SOFTWARE\Valve\Steam`, "InstallPath"},
	} {
		key, err := registry.OpenKey(k.root, k.path, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		v, _, err := key.GetStringValue(k.val)
		key.Close()
		if err == nil && v != "" {
			return filepath.Clean(strings.ReplaceAll(v, "/", `\`))
		}
	}
	return ""
}

// --- Epic Games Launcher ---------------------------------------------------

func probeEpic() []rawCandidate {
	programData := os.Getenv("ProgramData")
	if programData == "" {
		programData = `C:\ProgramData`
	}
	manifestDir := filepath.Join(programData, "Epic", "EpicGamesLauncher", "Data", "Manifests")
	entries, err := os.ReadDir(manifestDir)
	if err != nil {
		return nil
	}

	var out []rawCandidate
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".item") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(manifestDir, e.Name()))
		if err != nil {
			continue
		}
		var m struct {
			InstallLocation string `json:"InstallLocation"`
			DisplayName     string `json:"DisplayName"`
			AppName         string `json:"AppName"`
		}
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		hay := strings.ToLower(m.DisplayName + " " + m.AppName)
		if m.InstallLocation != "" && (strings.Contains(hay, "grand theft auto") || strings.Contains(hay, "gta")) {
			out = append(out, rawCandidate{path: m.InstallLocation, src: SourceEpic})
		}
	}
	return out
}

// --- Default install locations ------------------------------------------------

func probeCommonPaths() []rawCandidate {
	suffixes := []string{
		`Program Files\Rockstar Games\Grand Theft Auto V Enhanced`,
		`Program Files\Rockstar Games\Grand Theft Auto V`,
		`Program Files (x86)\Steam\steamapps\common\Grand Theft Auto V Enhanced`,
		`Program Files (x86)\Steam\steamapps\common\Grand Theft Auto V`,
		`SteamLibrary\steamapps\common\Grand Theft Auto V Enhanced`,
		`SteamLibrary\steamapps\common\Grand Theft Auto V`,
		`Program Files\Epic Games\GTAV Enhanced`,
		`Program Files\Epic Games\GTAV`,
		`Games\Grand Theft Auto V Enhanced`,
		`Games\Grand Theft Auto V`,
	}

	var drives []string
	for c := 'A'; c <= 'Z'; c++ {
		d := string(c) + `:\`
		if _, err := os.Stat(d); err == nil {
			drives = append(drives, d)
		}
	}

	var out []rawCandidate
	for _, d := range drives {
		for _, s := range suffixes {
			out = append(out, rawCandidate{path: filepath.Join(d, s), src: SourceCommonPath})
		}
	}
	return out
}
