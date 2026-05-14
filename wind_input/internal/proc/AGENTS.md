<!-- Parent: ../AGENTS.md -->
<!-- Updated: 2026-05-12 -->

# proc

## Purpose
启动外部进程 / URL / shell 命令行, 用于命令直通车 (cmdbar) 的 `open` / `run` / `shell` 动作函数。所有操作都是异步的, 不等待目标进程退出。

## Key Files
| File | Description |
|------|-------------|
| `proc.go` | 跨平台 API: `IsURL`, `Run(cmd, args...)`, shell flag 白名单常量 (`ShellFlagTerm`/`ShellFlagPwsh`) 与解析函数 `shellFlagSet`, 错误类型 |
| `proc_windows.go` | Windows 下 `Open` (走 `ShellExecuteW`) 与 `Shell` (走 `cmd /c`, CREATE_NO_WINDOW); `ShellEx(cmdline, flags)` 支持 `"term"` (cmd `/k` / pwsh `-NoExit`, 可见 console) 与 `"pwsh"` (优先 `pwsh.exe`, 回落 `powershell.exe`) flag |
| `proc_other.go` | 非 Windows 占位, `Open` / `Shell` / `ShellEx` 返回 `ErrUnsupportedPlatform`; `ShellEx` 仍执行 flag 解析以便跨平台单测 |

## For AI Agents

### Working In This Directory
- `Open` 接受 URL 或文件 / 可执行路径, 由 `ShellExecuteW` 统一分派 (verb=`open`); 不要自己 fork `cmd /c start` 这种二阶解释。
- `Run` 是直接的 `exec.Command(...).Start()`, 子进程通过后台 `Wait()` 收尸, 调用方无需感知。
- `Shell` 仅供需要管道 / 通配符的场景; 优先走 `Run`。

### Testing Requirements
- `proc_test.go` 覆盖 happy / error 路径。Windows-only happy path 用 `runtime.GOOS != "windows"` 时 `t.Skip()`。
- `go test ./internal/proc/...` 必须在 linux / windows 都能编译并通过。

## Dependencies

### Internal
- 无。

### External
- `golang.org/x/sys/windows` (仅 Windows 文件)。

<!-- MANUAL: Any manually added notes below this line are preserved on regeneration -->
