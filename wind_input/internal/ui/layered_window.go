//go:build windows

package ui

import (
	"fmt"
	"image"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// UpdateLayeredWindowFromImage renders an image.RGBA to the specified layered window.
// Handles RGBA→BGRA channel swap, CreateDIBSection, and UpdateLayeredWindow call.
// image.RGBA stores premultiplied alpha, which is what UpdateLayeredWindow expects.
func UpdateLayeredWindowFromImage(hwnd windows.HWND, img *image.RGBA, x, y int) error {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	hdcScreen, _, _ := procGetDC.Call(0)
	if hdcScreen == 0 {
		return fmt.Errorf("GetDC failed")
	}
	defer procReleaseDC.Call(0, hdcScreen)

	hdcMem, _, _ := procCreateCompatibleDC.Call(hdcScreen)
	if hdcMem == 0 {
		return fmt.Errorf("CreateCompatibleDC failed")
	}
	defer procDeleteDC.Call(hdcMem)

	bi := BITMAPINFO{
		BmiHeader: BITMAPINFOHEADER{
			BiSize:        uint32(unsafe.Sizeof(BITMAPINFOHEADER{})),
			BiWidth:       int32(width),
			BiHeight:      -int32(height), // Top-down DIB
			BiPlanes:      1,
			BiBitCount:    32,
			BiCompression: BI_RGB,
		},
	}

	var bits unsafe.Pointer
	hBitmap, _, err := procCreateDIBSection.Call(
		hdcMem,
		uintptr(unsafe.Pointer(&bi)),
		DIB_RGB_COLORS,
		uintptr(unsafe.Pointer(&bits)),
		0, 0,
	)
	if hBitmap == 0 {
		return fmt.Errorf("CreateDIBSection failed: %w", err)
	}
	defer procDeleteObject.Call(hBitmap)

	procSelectObject.Call(hdcMem, hBitmap)

	// RGBA to BGRA channel swap. image.RGBA is already premultiplied alpha,
	// matching UpdateLayeredWindow's expectation with AC_SRC_ALPHA.
	pixelCount := width * height
	dstSlice := unsafe.Slice((*byte)(bits), pixelCount*4)

	for i := 0; i < pixelCount; i++ {
		srcIdx := i * 4
		dstIdx := i * 4

		dstSlice[dstIdx+0] = img.Pix[srcIdx+2] // B
		dstSlice[dstIdx+1] = img.Pix[srcIdx+1] // G
		dstSlice[dstIdx+2] = img.Pix[srcIdx+0] // R
		dstSlice[dstIdx+3] = img.Pix[srcIdx+3] // A
	}

	ptSrc := POINT{X: 0, Y: 0}
	ptDst := POINT{X: int32(x), Y: int32(y)}
	size := SIZE{Cx: int32(width), Cy: int32(height)}
	blend := BLENDFUNCTION{
		BlendOp:             AC_SRC_OVER,
		BlendFlags:          0,
		SourceConstantAlpha: 255,
		AlphaFormat:         AC_SRC_ALPHA,
	}

	ret, _, err := procUpdateLayeredWindow.Call(
		uintptr(hwnd),
		hdcScreen,
		uintptr(unsafe.Pointer(&ptDst)),
		uintptr(unsafe.Pointer(&size)),
		hdcMem,
		uintptr(unsafe.Pointer(&ptSrc)),
		0,
		uintptr(unsafe.Pointer(&blend)),
		ULW_ALPHA,
	)

	if ret == 0 {
		return fmt.Errorf("UpdateLayeredWindow failed: %w", err)
	}

	return nil
}

// LayeredWindowConfig describes parameters for creating an IME layered window.
type LayeredWindowConfig struct {
	ClassName  string
	WndProc    uintptr // syscall.NewCallback result
	ExtraStyle uint32  // Additional exStyle flags (e.g., WS_EX_TRANSPARENT)
}

// CreateLayeredWindow creates a standard IME layered window.
// Base style: WS_EX_LAYERED | WS_EX_TOPMOST | WS_EX_TOOLWINDOW | WS_EX_NOACTIVATE.
// Additional flags can be set via cfg.ExtraStyle.
func CreateLayeredWindow(cfg LayeredWindowConfig) (windows.HWND, error) {
	className, _ := syscall.UTF16PtrFromString(cfg.ClassName)

	wc := WNDCLASSEXW{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEXW{})),
		LpfnWndProc:   cfg.WndProc,
		LpszClassName: className,
	}

	// RegisterClassExW may fail if already registered; this is expected.
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	exStyle := uint32(WS_EX_LAYERED|WS_EX_TOPMOST|WS_EX_TOOLWINDOW|WS_EX_NOACTIVATE) | cfg.ExtraStyle
	style := uint32(WS_POPUP)

	hwnd, _, err := procCreateWindowExW.Call(
		uintptr(exStyle),
		uintptr(unsafe.Pointer(className)),
		0,
		uintptr(style),
		0, 0, 1, 1,
		0, 0, 0, 0,
	)

	if hwnd == 0 {
		return 0, fmt.Errorf("CreateWindowExW failed for %s: %w", cfg.ClassName, err)
	}

	return windows.HWND(hwnd), nil
}
