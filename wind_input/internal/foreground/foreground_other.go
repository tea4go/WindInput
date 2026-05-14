//go:build !windows

package foreground

// App always returns "" on non-Windows builds.
func App() string { return "" }

// Title always returns "" on non-Windows builds.
func Title() string { return "" }
