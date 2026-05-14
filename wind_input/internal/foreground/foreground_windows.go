//go:build windows

package foreground

import (
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procGetWindowTextW           = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW     = user32.NewProc("GetWindowTextLengthW")
	procGetWindowThreadProcessID = user32.NewProc("GetWindowThreadProcessId")

	procOpenProcess                = kernel32.NewProc("OpenProcess")
	procCloseHandle                = kernel32.NewProc("CloseHandle")
	procQueryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")
)

const (
	processQueryLimitedInformation = 0x1000
)

// App 返回前台进程可执行文件的 basename (例如 "chrome.exe")。失败返回 ""。
func App() string {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return ""
	}
	var pid uint32
	procGetWindowThreadProcessID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return ""
	}
	h, _, _ := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(pid))
	if h == 0 {
		return ""
	}
	defer procCloseHandle.Call(h)

	// 32768 = NT 内核最大路径长度 (\\?\ 前缀路径)。QueryFullProcessImageNameW
	// 写入 wchar 数, 而非字节数。
	buf := make([]uint16, 32768)
	size := uint32(len(buf))
	r, _, _ := procQueryFullProcessImageNameW.Call(h, 0,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if r == 0 || size == 0 {
		return ""
	}
	full := syscall.UTF16ToString(buf[:size])
	if full == "" {
		return ""
	}
	return filepath.Base(full)
}

// Title 返回前台窗口的标题文本。失败或无标题返回 ""。
func Title() string {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return ""
	}
	n, _, _ := procGetWindowTextLengthW.Call(hwnd)
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n+1)
	r, _, _ := procGetWindowTextW.Call(hwnd,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:r])
}
