//go:build !windows

package proc

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// applyRunAttr / applyShellAttr 在非 Windows 上是 no-op (Windows 上
// applyRunAttr 实际未使用, 留作占位让跨平台测试不破)。
func applyRunAttr(_ *exec.Cmd)           {}
func applyShellAttr(_ *exec.Cmd, _ bool) {}

// Run 在非 Windows 上走 exec.Command (POSIX 没有 ShellExecuteW 对应)。
// 异步启动, 不等待退出。
func Run(cmd string, args ...string) error {
	if strings.TrimSpace(cmd) == "" {
		return fmt.Errorf("proc.Run: empty command")
	}
	c := exec.Command(cmd, args...)
	if err := c.Start(); err != nil {
		return fmt.Errorf("proc.Run %q: %w", cmd, err)
	}
	go func() { _ = c.Wait() }()
	return nil
}

// ErrUnsupportedPlatform is returned by Open/Shell on non-Windows
// platforms. Run remains cross-platform because exec.Command is
// portable; Open/Shell rely on Windows-specific behaviour (ShellExecute
// for URL associations, `cmd /c` for shell parsing).
var ErrUnsupportedPlatform = errors.New("proc: not supported on this platform")

// Open is a stub on non-Windows platforms.
func Open(target string) error {
	if strings.TrimSpace(target) == "" {
		return fmt.Errorf("proc.Open: empty target")
	}
	return ErrUnsupportedPlatform
}

// Shell is a stub on non-Windows platforms.
func Shell(cmdline string) error {
	return ShellEx(cmdline, nil)
}

// ShellEx is a stub on non-Windows platforms.
// flags 解析仍执行, 便于在跨平台单测中验证 flag 校验路径。
func ShellEx(cmdline string, flags []string) error {
	if strings.TrimSpace(cmdline) == "" {
		return fmt.Errorf("proc.Shell: empty cmdline")
	}
	if _, _, err := shellFlagSet(flags); err != nil {
		return err
	}
	return ErrUnsupportedPlatform
}
