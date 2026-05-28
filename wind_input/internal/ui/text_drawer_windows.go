package ui

import (
	"image"
	"image/color"
)

// text_drawer_windows.go — TextDrawer 的 GDI + DirectWrite 实现, 仅 Win 平台。
// 文件名后缀 _windows.go 自动应用 windows build tag, 与显式 //go:build windows 等价。
// freetype 实现 (跨平台) 留在 text_drawer.go。

// --- GDI implementation ---

// gdiDrawer wraps TextRenderer for Windows-native GDI text rendering.
type gdiDrawer struct {
	tr *TextRenderer
}

func newGDIDrawer(tr *TextRenderer) *gdiDrawer {
	return &gdiDrawer{tr: tr}
}

func (d *gdiDrawer) SetFont(fontPath string) {
	d.tr.SetFont(fontPath)
}

func (d *gdiDrawer) MeasureString(text string, fontSize float64) float64 {
	return d.tr.MeasureString(text, fontSize)
}

func (d *gdiDrawer) BeginDraw(img *image.RGBA) {
	d.tr.BeginDraw(img)
}

func (d *gdiDrawer) DrawString(text string, x, y float64, fontSize float64, clr color.Color) {
	d.tr.DrawString(text, x, y, fontSize, clr)
}

func (d *gdiDrawer) DrawStringWithWeight(text string, x, y float64, fontSize float64, clr color.Color, weight int) {
	d.tr.DrawStringWithWeight(text, x, y, fontSize, clr, weight)
}

func (d *gdiDrawer) EndDraw() {
	d.tr.EndDraw()
}

func (d *gdiDrawer) Close() {
	d.tr.Close()
}

// --- DirectWrite implementation ---

// directWriteDrawer wraps DWriteRenderer for DirectWrite + Direct2D text rendering.
type directWriteDrawer struct {
	tr *DWriteRenderer
}

func newDirectWriteDrawer(tr *DWriteRenderer) *directWriteDrawer {
	return &directWriteDrawer{tr: tr}
}

func (d *directWriteDrawer) SetFont(fontPath string) {
	d.tr.SetFont(fontPath)
}

func (d *directWriteDrawer) MeasureString(text string, fontSize float64) float64 {
	return d.tr.MeasureString(text, fontSize)
}

func (d *directWriteDrawer) BeginDraw(img *image.RGBA) {
	d.tr.BeginDraw(img)
}

func (d *directWriteDrawer) DrawString(text string, x, y float64, fontSize float64, clr color.Color) {
	d.tr.DrawString(text, x, y, fontSize, clr)
}

func (d *directWriteDrawer) DrawStringWithWeight(text string, x, y float64, fontSize float64, clr color.Color, weight int) {
	d.tr.DrawStringWithWeight(text, x, y, fontSize, clr, weight)
}

func (d *directWriteDrawer) EndDraw() {
	d.tr.EndDraw()
}

func (d *directWriteDrawer) Close() {
	d.tr.Close()
}
