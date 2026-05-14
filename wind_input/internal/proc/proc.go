// Package proc launches external processes / URLs / shell command
// lines. It backs the command-bar `open` / `run` / `shell` action
// functions. All operations are best-effort and asynchronous: launchers
// do not wait for the spawned process to exit.
package proc

import (
	"fmt"
	"strings"
)

// ShellFlag 是 ShellEx 支持的 flag 白名单 (string set 形式)。
// flag 集合: "term", "pwsh"。未知 flag 由 ShellEx 拒绝。
const (
	ShellFlagTerm = "term" // 新建可见 console 窗口 (cmd /k / pwsh -NoExit)
	ShellFlagPwsh = "pwsh" // 使用 PowerShell (pwsh.exe 优先, 回落 powershell.exe)
)

// shellFlagSet 解析 + 校验 flags 列表, 返回 (term, pwsh, error)。
// 空串/全空白 flag 自动跳过。未知 flag 立即报错, 不静默忽略, 方便用户排错。
func shellFlagSet(flags []string) (term, pwsh bool, err error) {
	for _, raw := range flags {
		f := strings.ToLower(strings.TrimSpace(raw))
		if f == "" {
			continue
		}
		switch f {
		case ShellFlagTerm:
			term = true
		case ShellFlagPwsh:
			pwsh = true
		default:
			return false, false, fmt.Errorf("shell: unknown flag %q", raw)
		}
	}
	return term, pwsh, nil
}

// IsURL reports whether target looks like an `http://` or `https://`
// URL. Shared helper exposed for `search()` action wiring.
func IsURL(target string) bool {
	l := strings.ToLower(target)
	return strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://")
}

// Run 启动 cmd 进程, 异步; 不等待退出。Windows 实现走 ShellExecuteW
// (见 proc_windows.go), POSIX 走 exec.Command (见 proc_other.go)。
//
// 选 ShellExecute 而不是 exec.Command 是 Windows 端的关键决策:
//   - 由 Explorer shell 代为启动, 等同用户双击, 自然继承前台焦点权限
//     (避免 IME 在后台时因 SetForegroundWindow 限制而无法前台)
//   - 不继承父进程的 stdin/stdout/stderr 句柄, 避免 IME 输出管道被锁
//   - 自动 PATH 解析 + 文件关联 (notepad.exe / .pdf / URL 一视同仁)
//   - 不阻塞调用线程 (内部异步 spawn)
//
// 用 exec.Command 的旧实现实测: notepad/calc 等 GUI 应用启动慢、有时
// 不前台、IME 服务出现繁忙 (因为 exec.Command 的句柄继承 + 前台权限缺
// 失合并触发)。
//
// 这里只导出函数签名, 真正实现见平台分支。
