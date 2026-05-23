//go:build windows

package foreground

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shell32 = windows.NewLazySystemDLL("shell32.dll")

	procGetWindowRect                = user32.NewProc("GetWindowRect")
	procMonitorFromWindow            = user32.NewProc("MonitorFromWindow")
	procFullscreenGetMonitorInfoW    = user32.NewProc("GetMonitorInfoW")
	procGetDesktopWindow             = user32.NewProc("GetDesktopWindow")
	procGetShellWindow               = user32.NewProc("GetShellWindow")
	procSHQueryUserNotificationState = shell32.NewProc("SHQueryUserNotificationState")
)

const (
	monitorDefaultToNearest = 2

	// QUERY_USER_NOTIFICATION_STATE values (shellapi.h)
	qunsRunningD3DFullScreen = 3
	qunsPresentationMode     = 4
)

type fsRect struct {
	Left, Top, Right, Bottom int32
}

type fsMonitorInfo struct {
	CbSize    uint32
	RcMonitor fsRect
	RcWork    fsRect
	DwFlags   uint32
}

// IsForegroundFullscreen 判定当前前台窗口是否处于全屏状态。
//
// 采用两类互补判据：
//  1. SHQueryUserNotificationState 返回 D3D 独占全屏或演示模式 —— 覆盖
//     游戏（DirectX 独占）和 PowerPoint 放映等系统级标记的全屏场景。
//  2. 窗口矩形完全覆盖其所在显示器的物理矩形 —— 覆盖浏览器 F11、
//     视频播放器无边框全屏、远程桌面全屏等常规场景。
//
// 排除桌面窗口与 Shell 窗口，避免空桌面被误判。
//
// 非 Windows 平台见 fullscreen_other.go，恒返回 false。
func IsForegroundFullscreen() bool {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return false
	}

	desktop, _, _ := procGetDesktopWindow.Call()
	shell, _, _ := procGetShellWindow.Call()
	if hwnd == desktop || (shell != 0 && hwnd == shell) {
		return false
	}

	// 判据 1：系统通知状态。返回 S_OK (0) 时检查全屏标志。
	var quns uint32
	hr, _, _ := procSHQueryUserNotificationState.Call(uintptr(unsafe.Pointer(&quns)))
	if hr == 0 && (quns == qunsRunningD3DFullScreen || quns == qunsPresentationMode) {
		return true
	}

	// 判据 2：窗口矩形 ⊇ 显示器物理矩形。
	var wr fsRect
	ok, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr)))
	if ok == 0 {
		return false
	}

	hMon, _, _ := procMonitorFromWindow.Call(hwnd, monitorDefaultToNearest)
	if hMon == 0 {
		return false
	}

	var mi fsMonitorInfo
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	ok2, _, _ := procFullscreenGetMonitorInfoW.Call(hMon, uintptr(unsafe.Pointer(&mi)))
	if ok2 == 0 {
		return false
	}

	return wr.Left <= mi.RcMonitor.Left &&
		wr.Top <= mi.RcMonitor.Top &&
		wr.Right >= mi.RcMonitor.Right &&
		wr.Bottom >= mi.RcMonitor.Bottom
}
