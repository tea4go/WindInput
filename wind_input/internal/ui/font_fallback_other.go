//go:build !darwin

package ui

// platformTextFallbackFonts 在非 darwin 平台无额外原生回退 (Win 走 defaultTextFallbackFontNames)。
func platformTextFallbackFonts() []string { return nil }
