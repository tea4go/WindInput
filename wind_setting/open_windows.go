//go:build windows

package main

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modShell32        = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteW = modShell32.NewProc("ShellExecuteW")
)

// shellOpen 通过 ShellExecuteW 打开文件、目录或 URL。
// 相比 explorer.exe / rundll32，ShellExecuteW 直接调用系统 shell，
// 对所有默认浏览器均兼容，且不存在 cmd shell 注入风险。
func shellOpen(path string) error {
	verbPtr, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	ret, _, _ := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verbPtr)),
		uintptr(unsafe.Pointer(pathPtr)),
		0, 0,
		uintptr(windows.SW_SHOWNORMAL),
	)
	if ret <= 32 {
		return fmt.Errorf("ShellExecuteW 失败，错误码: %d", ret)
	}
	return nil
}
