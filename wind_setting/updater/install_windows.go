//go:build windows

package updater

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shell32           = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteW = shell32.NewProc("ShellExecuteW")
)

// InstallRelease 通过 ShellExecuteW 启动 NSIS 安装程序，支持 UAC 自动提权。
// silent=true 时传入 /QUIET 参数（显示进度界面但自动安装），否则显示完整交互界面。
func InstallRelease(installerPath string, silent bool) error {
	verbPtr, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	filePtr, err := windows.UTF16PtrFromString(installerPath)
	if err != nil {
		return err
	}
	var paramsPtr *uint16
	if silent {
		paramsPtr, err = windows.UTF16PtrFromString("/QUIET")
		if err != nil {
			return err
		}
	}
	ret, _, _ := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verbPtr)),
		uintptr(unsafe.Pointer(filePtr)),
		uintptr(unsafe.Pointer(paramsPtr)),
		0,
		uintptr(windows.SW_SHOWNORMAL),
	)
	// ShellExecuteW 返回值 > 32 表示成功
	if ret <= 32 {
		return fmt.Errorf("启动安装程序失败，错误码 %d", ret)
	}
	return nil
}
