//go:build windows

package proc

import (
	"os/exec"
	"syscall"
)

// newCmdForTest 构造一个不会真启动的 exec.Cmd, 用于检查 SysProcAttr。
func newCmdForTest() *exec.Cmd {
	return exec.Command("notepad.exe")
}

// readCreationFlags 把 SysProcAttr.CreationFlags 取出来 (Windows 专属字段)。
func readCreationFlags(a *syscall.SysProcAttr) uint32 {
	if a == nil {
		return 0
	}
	return a.CreationFlags
}
