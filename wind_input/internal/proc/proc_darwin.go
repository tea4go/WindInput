//go:build darwin

package proc

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

// ErrUnsupportedPlatform 在 macOS 上仅用于 ShellEx 的 "term" flag —— 弹出
// 可见终端窗口需要 osascript (触发 TCC 自动化授权) 或临时脚本, 当前未实现。
// Open/Run/Shell 本身在 macOS 都是真实现, 不会返回此错误。
var ErrUnsupportedPlatform = errors.New("proc: not supported on this platform")

// applyRunAttr / applyShellAttr 把子进程放进独立进程组 (Setpgid), 等价于
// Windows 的 CREATE_NEW_PROCESS_GROUP: cmdbar 启动的进程脱离输入法服务的
// 进程组, 服务重启 / 被杀 (LaunchAgent 重载) 不会把信号连带传给子进程。
// macOS 不需要也没有 Windows 的窗口可见性 flag, 所以 term 参数在此忽略
// (term=true 的请求在 ShellEx 入口就已被 ErrUnsupportedPlatform 拦截)。
func applyRunAttr(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func applyShellAttr(c *exec.Cmd, _ bool) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// Run 直接 exec.Command 启动可执行文件, 异步, 不等待退出。
// macOS 上对“启动一个 GUI 应用”更推荐 Open (走 `open`), Run 适用于直接
// 调用 PATH 中的命令行程序。
func Run(cmd string, args ...string) error {
	if strings.TrimSpace(cmd) == "" {
		return fmt.Errorf("proc.Run: empty command")
	}
	c := exec.Command(cmd, args...)
	applyRunAttr(c)
	if err := c.Start(); err != nil {
		return fmt.Errorf("proc.Run %q: %w", cmd, err)
	}
	go func() { _ = c.Wait() }()
	return nil
}

// Open 通过 macOS 的 `open` 命令打开 target, 是 Windows ShellExecuteW 的天然
// 对应: URL (https://...)、文件 (.pdf/.txt)、目录、`.app` 应用包都由 `open`
// 经 LaunchServices 统一分派, 并自然获得前台焦点。异步, 不等待退出。
func Open(target string) error {
	t := strings.TrimSpace(target)
	if t == "" {
		return fmt.Errorf("proc.Open: empty target")
	}
	c := exec.Command("open", t)
	applyRunAttr(c)
	if err := c.Start(); err != nil {
		return fmt.Errorf("proc.Open %q: %w", t, err)
	}
	go func() { _ = c.Wait() }()
	return nil
}

// Shell 通过 `/bin/sh -c <cmdline>` 静默执行命令行 (支持管道 / 通配符)。
func Shell(cmdline string) error {
	return ShellEx(cmdline, nil)
}

// ShellEx 是 Shell 的扩展形式, 通过 flags 控制 shell 类型与窗口可见性。
// macOS 行为:
//   - 无 flag: `/bin/sh -c <cmdline>`, 静默执行
//   - "pwsh":  使用 PATH 中的 PowerShell 7+ (`pwsh -Command`); macOS 无
//     `powershell.exe` 回落, 找不到 pwsh 时返回明确错误
//   - "term":  暂不支持 (返回 ErrUnsupportedPlatform) —— 弹出可见终端窗口
//     需 osascript (TCC 授权) 或临时 .command 脚本, 留待后续
//
// 未识别 flag 由 shellFlagSet 报错; nil/空 flags 等同 Shell。
func ShellEx(cmdline string, flags []string) error {
	if strings.TrimSpace(cmdline) == "" {
		return fmt.Errorf("proc.Shell: empty cmdline")
	}
	term, pwsh, err := shellFlagSet(flags)
	if err != nil {
		return err
	}
	if term {
		return fmt.Errorf("proc.Shell: term flag not supported on macOS: %w", ErrUnsupportedPlatform)
	}

	var c *exec.Cmd
	if pwsh {
		bin, lookErr := exec.LookPath("pwsh")
		if lookErr != nil {
			return fmt.Errorf("proc.Shell: pwsh not found in PATH: %w", lookErr)
		}
		c = exec.Command(bin, "-Command", cmdline)
	} else {
		c = exec.Command("/bin/sh", "-c", cmdline)
	}
	applyShellAttr(c, term)
	if err := c.Start(); err != nil {
		return fmt.Errorf("proc.Shell: %w", err)
	}
	go func() { _ = c.Wait() }()
	return nil
}
