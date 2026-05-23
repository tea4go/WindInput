//go:build !windows

package foreground

// IsForegroundFullscreen 在非 Windows 平台恒返回 false。
func IsForegroundFullscreen() bool { return false }
