//go:build !darwin

package ui

import (
	"image/color"
	"image/draw"

	ggtext "github.com/gogpu/gg/text"
)

// drawColorEmoji 在非 darwin 平台不处理 (Win 走 DirectWrite 彩色字形路径)。
func drawColorEmoji(_ draw.Image, _ ggtext.Face, _ string, _, _ float64, _ color.Color) (float64, bool) {
	return 0, false
}

// colorEmojiAdvance 非 darwin 平台不处理。
func colorEmojiAdvance(_ string, _ float64) (float64, bool) { return 0, false }
