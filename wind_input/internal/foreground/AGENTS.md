<!-- Parent: ../AGENTS.md -->
<!-- Created: 2026-05-12 -->

# internal/foreground

## Purpose
读取 Windows 当前前台窗口的进程名与窗口标题, 给命令直通车 (cmdbar) 的
`app()` / `title()` 取值函数提供真实数据来源。非 Windows 构建返回空串以
保证跨平台可编译。

## Key Files
| File | Description |
|------|-------------|
| `foreground.go` | 包注释 (跨平台共用) |
| `foreground_windows.go` | Windows 实现: `GetForegroundWindow` + `QueryFullProcessImageNameW` + `GetWindowTextW` |
| `foreground_other.go` | 非 Windows 桩: `App() = ""`, `Title() = ""` |
| `foreground_test.go` | 仅验证函数可调用, 不断言具体返回值 (依赖前台窗口环境) |

## Public API
- `App() string` — 前台进程的可执行文件 basename (如 `"chrome.exe"`); 失败返回 `""`
- `Title() string` — 前台窗口标题; 失败或无标题返回 `""`

## For AI Agents

### Working In This Directory
- 错误一律返回空字符串, 不 panic; 上游 cmdbar 会把空当作"未知"。
- Windows 实现走 lazy DLL 调用 (与 `internal/clipboard` 同风格), 不引入新的 cgo 依赖。
- 不要在此处缓存值: 前台窗口可能任何时间变化, 求值时实时读。

## Dependencies
- 标准库 + `golang.org/x/sys/windows` (Windows only)

<!-- MANUAL: Any manually added notes below this line are preserved on regeneration -->
