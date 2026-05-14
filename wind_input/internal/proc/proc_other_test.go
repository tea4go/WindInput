//go:build !windows

package proc

import (
	"os/exec"
	"syscall"
)

// newCmdForTest 在非 Windows 平台返回一个简单 Cmd; 测试会被 GOOS 跳过。
func newCmdForTest() *exec.Cmd { return exec.Command("/bin/true") }

// readCreationFlags 在非 Windows 平台无意义, 直接返回 0。
func readCreationFlags(_ *syscall.SysProcAttr) uint32 { return 0 }
