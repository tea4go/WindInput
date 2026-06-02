//go:build windows

package ui

import (
	"image"
	"unsafe"
)

// render renders the menu to an image（盒模型 View 引擎）。
// root LayoutColumn + 每项 LayoutRow（check/text/arrow 文本叶子）；分隔线后处理。
func (m *PopupMenu) render() *image.RGBA {
	m.mu.Lock()
	items := m.items
	hoverIdx := m.hoverIndex
	submenuIdx := m.submenuIndex
	width := m.width
	height := m.height
	hasChecked := m.hasChecked
	hasChildren := m.hasChildren
	rmv := m.resolveMenuColors()
	td := m.textDrawer
	baseFontSize := m.getMenuFontSize()
	m.mu.Unlock()

	scale := m.dpiScale()
	itemHeightLogical := m.getMenuItemHeight()

	mt := buildMenuTree(items, hoverIdx, submenuIdx, hasChecked, hasChildren, rmv, width, height, baseFontSize, itemHeightLogical, scale)
	Layout(mt.root, 0, 0, td)
	dc, img := newSharedDrawContext(width, height)
	PaintTree(mt.root, dc, img, td)

	// 后处理分隔线（矢量，定位用分隔项 Rect()）
	for _, sep := range mt.separators {
		r := sep.Rect()
		sepY := float64(r.Min.Y) + float64(r.Dy())/2
		dc.SetColor(rmv.SeparatorColor)
		dc.DrawLine(4*scale, sepY, float64(width)-4*scale, sepY)
		dc.Stroke()
	}

	DrawDebugBanner(img)
	return img
}

// updateWindow updates the layered window with the rendered image
func (m *PopupMenu) updateWindow() {
	img := m.render()

	m.mu.Lock()
	x, y := m.x, m.y
	m.mu.Unlock()

	UpdateLayeredWindowFromImage(m.hwnd, img, x, y)
}

// trackMouseLeave enables mouse leave tracking
func (m *PopupMenu) trackMouseLeave() {
	tme := TRACKMOUSEEVENT{
		CbSize:    uint32(unsafe.Sizeof(TRACKMOUSEEVENT{})),
		DwFlags:   TME_LEAVE,
		HwndTrack: uintptr(m.hwnd),
	}
	procTrackMouseEvent.Call(uintptr(unsafe.Pointer(&tme)))
}
