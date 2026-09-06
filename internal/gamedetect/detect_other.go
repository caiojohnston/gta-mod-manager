//go:build !windows

package gamedetect

// osProbe is a no-op off Windows. GTA V Enhanced only runs on Windows for PC
// modding purposes; a Linux/macOS build exists only for fast iteration on the
// non-UI logic (PLAN.md, "Target OS").
func osProbe() []rawCandidate { return nil }
