// Package launcher starts GTA V through the platform launcher, the same way
// double-clicking the game would, so Steam/Epic/Rockstar overlay and cloud
// saves all initialise normally.
package launcher

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// candidates are tried in order. PlayGTAV.exe is the universal shim Rockstar
// ships: it detects the store the copy came from and hands off to that
// launcher. The raw game exe is the last resort.
var candidates = []string{"PlayGTAV.exe", "GTA5_Enhanced.exe", "GTA5.exe"}

// Resolve returns the executable Launch would start for gameDir, or an error if
// none of the known entrypoints exist there.
func Resolve(gameDir string) (string, error) {
	if gameDir == "" {
		return "", fmt.Errorf("launcher: no game folder set")
	}
	for _, name := range candidates {
		p := filepath.Join(gameDir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("launcher: no PlayGTAV.exe / GTA5_Enhanced.exe found in %q", gameDir)
}

// Launch starts the game detached (the app does not wait for it) and returns
// the path it started.
func Launch(gameDir string) (string, error) {
	exe, err := Resolve(gameDir)
	if err != nil {
		return "", err
	}
	cmd := exec.Command(exe)
	cmd.Dir = gameDir
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("launcher: starting %q: %w", exe, err)
	}
	// Don't Wait: let the game outlive the manager. Release the process handle.
	_ = cmd.Process.Release()
	return exe, nil
}
