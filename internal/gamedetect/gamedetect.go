// Package gamedetect locates an existing GTA V install (Enhanced preferred,
// Legacy as a fallback) so the app can offer a "detect automatically" action
// instead of making every user hunt for the folder by hand (SPEC.md §4.1).
//
// The OS-specific probing lives in detect_windows.go / detect_other.go so a
// non-Windows dev build still compiles (PLAN.md, "Target OS" note).
package gamedetect

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Source labels where a candidate folder came from, shown in the picker UI.
type Source string

const (
	SourceRockstar   Source = "Rockstar Games Launcher"
	SourceSteam      Source = "Steam"
	SourceEpic       Source = "Epic Games"
	SourceCommonPath Source = "Common install path"
)

// Candidate is one detected install folder.
type Candidate struct {
	Path string // absolute, cleaned
	// Source is where this path was found (registry, Steam library, etc.).
	Source Source
	// Exe is the confirming executable's filename, e.g. "GTA5_Enhanced.exe".
	Exe string
	// Enhanced is true when Exe is the Enhanced-edition binary; those sort first.
	Enhanced bool
}

// exeNames are the executables that confirm a folder is a GTA V install. The
// Enhanced binary is listed first so it wins when a folder somehow has both.
var exeNames = []string{"GTA5_Enhanced.exe", "GTA5.exe", "PlayGTAV.exe"}

// Detect returns every validated GTA V folder found on this machine, best
// guess first (Enhanced edition, then by how reliable the source is). Never
// returns an error: probing is entirely best-effort and a missing registry
// key or launcher just yields fewer candidates.
func Detect() []Candidate {
	var out []Candidate
	seen := make(map[string]bool)

	add := func(path string, src Source) {
		if path == "" {
			return
		}
		clean := filepath.Clean(path)
		key := strings.ToLower(clean)
		if seen[key] {
			return
		}
		exe, enhanced, ok := confirm(clean)
		if !ok {
			return
		}
		seen[key] = true
		out = append(out, Candidate{Path: clean, Source: src, Exe: exe, Enhanced: enhanced})
	}

	for _, c := range probeFn() { // OS-specific, see detect_*.go
		add(c.path, c.src)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Enhanced != out[j].Enhanced {
			return out[i].Enhanced // Enhanced installs first
		}
		return sourceRank(out[i].Source) < sourceRank(out[j].Source)
	})
	return out
}

// rawCandidate is an unvalidated path from an OS probe.
type rawCandidate struct {
	path string
	src  Source
}

// probeFn is the OS-specific candidate source (osProbe, see detect_*.go). It's
// a package var so tests can substitute a fixture without a real registry or
// Steam install.
var probeFn = osProbe

// confirm checks whether dir holds a recognised GTA V executable.
func confirm(dir string) (exe string, enhanced bool, ok bool) {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "", false, false
	}
	for _, name := range exeNames {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return name, name == "GTA5_Enhanced.exe", true
		}
	}
	return "", false, false
}

func sourceRank(s Source) int {
	switch s {
	case SourceRockstar:
		return 0
	case SourceSteam:
		return 1
	case SourceEpic:
		return 2
	default:
		return 3
	}
}
