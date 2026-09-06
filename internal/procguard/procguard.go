// Package procguard implements SPEC.md G5: never move mod files while the
// game is running, to avoid corrupting a live install.
package procguard

import (
	"fmt"
	"strings"

	ps "github.com/mitchellh/go-ps"
)

// IsGameRunning checks the process list for exeName (case-insensitive,
// e.g. "GTA5_Enhanced.exe"). Returns an error only on a genuine failure to
// read the process list, never as a way of reporting "not running".
func IsGameRunning(exeName string) (bool, error) {
	procs, err := ps.Processes()
	if err != nil {
		return false, fmt.Errorf("procguard: listing processes: %w", err)
	}
	target := strings.ToLower(exeName)
	for _, p := range procs {
		if strings.ToLower(p.Executable()) == target {
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
