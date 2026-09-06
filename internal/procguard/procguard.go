// Package procguard implements SPEC.md G5: never move mod files while the
// game is running, to avoid corrupting a live install.
package procguard

import (
	"fmt"
	"strings"

	ps "github.com/mitchellh/go-ps"
)

// listProcesses returns the running process executable names. It's a package
// variable so tests can substitute a fake list without spawning real
// processes; production code uses the go-ps implementation below.
var listProcesses = func() ([]string, error) {
	procs, err := ps.Processes()
	if err != nil {
		return nil, fmt.Errorf("procguard: listing processes: %w", err)
	}
	names := make([]string, len(procs))
	for i, p := range procs {
		names[i] = p.Executable()
	}
	return names, nil
}

// IsGameRunning checks the process list for exeName (case-insensitive,
// e.g. "GTA5_Enhanced.exe"). Returns an error only on a genuine failure to
// read the process list, never as a way of reporting "not running".
func IsGameRunning(exeName string) (bool, error) {
	names, err := listProcesses()
	if err != nil {
		return false, err
	}
	target := strings.ToLower(exeName)
	for _, n := range names {
		if strings.ToLower(n) == target {
			return true, nil
		}
	}
	return false, nil
}

// ErrGameRunning is returned by any file-moving operation that checks the
// guard and finds the game active. Callers should surface this to the user
// verbatim rather than a generic failure.
type ErrGameRunning struct{ ExeName string }

func (e *ErrGameRunning) Error() string {
	return fmt.Sprintf("%s is running — close the game before changing mods", e.ExeName)
}
