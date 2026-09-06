package procguard

// SetProcessLister replaces the process-list source and returns a function
// that restores the original. Intended for tests in this and other packages
// (toggler, profiles) that must run without the real game process check
// interfering. Not used by production code.
func SetProcessLister(fn func() ([]string, error)) (restore func()) {
	prev := listProcesses
	listProcesses = fn
	return func() { listProcesses = prev }
}
