//go:build windows

package proc

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

// Windows CreateProcess flag 常量。syscall 包对部分 flag 未导出, 这里
// 用裸常量, 与 MSDN ProcessCreationFlags 文档一致:
//
//	DETACHED_PROCESS         (0x00000008) 子进程无 console
//	CREATE_NEW_CONSOLE       (0x00000010) 给子进程分配独立 console 窗口
//	CREATE_NO_WINDOW         (0x08000000) 不显示 console 窗口
//	CREATE_NEW_PROCESS_GROUP (0x00000200) 独立 Ctrl 信号进程组
//
// 不使用 CREATE_BREAKAWAY_FROM_JOB —— 它在某些受限环境 (VS 调试器、
// Docker、企业 Job 限制) 下会让 CreateProcess 直接 ACCESS_DENIED 失败,
// 而 wind_input.exe 作为普通用户 GUI 进程通常不在 kill-on-close Job 里,
// 所以 NEW_PROCESS_GROUP 已足够实现"父进程退出不带走子进程"的目标。
const (
	flagDetachedProcess       = 0x00000008
	flagCreateNewConsole      = 0x00000010
	flagCreateNoWindow        = 0x08000000
	flagCreateNewProcessGroup = 0x00000200
)

// applyRunAttr 用于 Run / Open 启动 GUI 应用 (notepad / calc / 浏览器
// 等)。只设 NEW_PROCESS_GROUP 让子进程脱离 Ctrl 信号传播; **绝不**设
// HideWindow / CREATE_NO_WINDOW —— 否则会把 GUI 窗口隐藏, 用户看到的
// 就是"繁忙但没打开"。
func applyRunAttr(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: flagCreateNewProcessGroup,
	}
}

// applyShellAttr 用于 Shell/ShellEx 启动 cmd 或 powershell。
//   - term=true: 用户希望看到 console, 用 CREATE_NEW_CONSOLE
//   - term=false: 静默执行, DETACHED_PROCESS + CREATE_NO_WINDOW + HideWindow,
//     彻底不弹任何窗口
func applyShellAttr(c *exec.Cmd, term bool) {
	flags := uint32(flagCreateNewProcessGroup)
	if term {
		flags |= flagCreateNewConsole
		c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: flags}
		return
	}
	flags |= flagDetachedProcess | flagCreateNoWindow
	c.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: flags,
	}
}

// Run 在 Windows 上走 exec.Command (CreateProcess) —— 临时切换以便
// 对比 ShellExecuteW 版本。只设 NEW_PROCESS_GROUP, 不隐藏窗口。
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

// Open launches target through Windows' ShellExecuteW with verb "open".
// Works for both URLs (https://...) and filesystem paths (.exe, .pdf,
// folders, ...). Asynchronous.
func Open(target string) error {
	t := strings.TrimSpace(target)
	if t == "" {
		return fmt.Errorf("proc.Open: empty target")
	}
	verb, err := syscall.UTF16PtrFromString("open")
	if err != nil {
		return fmt.Errorf("proc.Open verb: %w", err)
	}
	file, err := syscall.UTF16PtrFromString(t)
	if err != nil {
		return fmt.Errorf("proc.Open file: %w", err)
	}
	// hwnd=0, verb="open", lpFile=target, lpParameters=nil, lpDirectory=nil, nShowCmd=SW_SHOWNORMAL(1)
	if err := windows.ShellExecute(0, verb, file, nil, nil, 1); err != nil {
		return fmt.Errorf("ShellExecute(%q): %w", t, err)
	}
	return nil
}

// Shell runs cmdline via `cmd /c <cmdline>` asynchronously.
func Shell(cmdline string) error {
	return ShellEx(cmdline, nil)
}

// resolvePowerShell 返回首个可用的 PowerShell 可执行路径名。
// 优先 pwsh.exe (PowerShell 7+), 找不到回落到 powershell.exe (Windows 内置)。
// 返回的字符串可直接传给 exec.Command (依赖 PATH 解析)。
func resolvePowerShell() string {
	if _, err := exec.LookPath("pwsh.exe"); err == nil {
		return "pwsh.exe"
	}
	return "powershell.exe"
}

// ShellEx 是 Shell 的扩展形式, 通过 flags 控制 shell 类型与窗口可见性。
// flags 中:
//   - 含 "pwsh": 用 PowerShell 代替 cmd
//   - 含 "term": 创建可见 console 窗口 (CREATE_NEW_CONSOLE); cmd 走 /k,
//     powershell 走 -NoExit -Command, 命令执行完窗口保留
//
// 未识别 flag 返回 error; nil/空 flags 等同 Shell 旧行为 (cmd /c, 隐藏窗口)。
func ShellEx(cmdline string, flags []string) error {
	if strings.TrimSpace(cmdline) == "" {
		return fmt.Errorf("proc.Shell: empty cmdline")
	}
	term, pwsh, err := shellFlagSet(flags)
	if err != nil {
		return err
	}

	var (
		shellBin string
		shellArg string // "/c"|"/k" or "-Command"|"-NoExit -Command"
	)
	if pwsh {
		shellBin = resolvePowerShell()
		if term {
			shellArg = "-NoExit"
		} else {
			shellArg = "-Command"
		}
	} else {
		shellBin = "cmd"
		if term {
			shellArg = "/k"
		} else {
			shellArg = "/c"
		}
	}

	var c *exec.Cmd
	if pwsh && term {
		// powershell 在 -NoExit 模式下需要 -Command 才能识别后续片段。
		c = exec.Command(shellBin, "-NoExit", "-Command", cmdline)
	} else {
		c = exec.Command(shellBin, shellArg, cmdline)
	}

	// 进程脱钩 + 窗口可见性: term=true 用 CREATE_NEW_CONSOLE 弹独立窗口;
	// term=false 用 DETACHED_PROCESS+HideWindow 完全静默。
	applyShellAttr(c, term)
	if err := c.Start(); err != nil {
		return fmt.Errorf("proc.Shell: %w", err)
	}
	go func() { _ = c.Wait() }()
	return nil
}
