<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-04-20 | Updated: 2026-04-20 -->

# pkg/systemfont

## Purpose
跨平台系统字体目录扫描和信息提供。枚举已安装的字体族并提供本地化显示名称, 供 `internal/ui` 字体解析使用。对外 API (`List`/`HasFamily`/`ResolveFile`/`ResolveDWFamily`) 两平台同名同签名, 调用方零差异。

- **Windows**: 通过 Registry（`HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Fonts`）枚举, 异步解析本地化名。
- **darwin**: 扫描 `/System/Library/Fonts`、`Supplemental`、`/Library/Fonts`、`~/Library/Fonts` + 递归 `/System/Library/AssetsV2`（Tahoe 26 把 PingFang/SF Pro 等迁到此处）, 解 sfnt name table 取 family 名, 一次同步完成。

## Key Files
| File | Description |
|------|-------------|
| `catalog_windows.go` | Win 实现 (`//go:build windows`): `FontInfo`；`List`/`HasFamily`/`ResolveFile`/`ResolveDWFamily`；Registry 扫描 + `sync.Once` 缓存 + 字体名本地化异步解析 |
| `catalog_darwin.go` | darwin 实现 (`//go:build darwin`): 同名 API；目录扫描 (含 AssetsV2 递归) + TTC 全子字体枚举 + name table platformID 0/1/3 三平台解析 (Apple Unicode / Mac Roman / Windows)；`sync.Once` 缓存；fallback PingFang/Helvetica |
| `nametable.go` | 平台无关: sfnt Name Table 解析 (`readNameTableData`/`parseChineseFamilyName`/`parseAllFamilyNames`/`decodeUTF16BE`)；Win + darwin 共用 |
| `catalog_windows_test.go` | Registry 扫描单元测试 (Win) |

## For AI Agents

### Working In This Directory
- **Registry 路径**：`HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Fonts`
- **Registry 扫描**：值名称为 `字体族名 [修饰符]`（如 `Consolas Bold`），值数据为文件路径（如 `consolasb.ttf`）
- **字体样式处理**：移除后缀修饰符（`Bold`、`Italic` 等），仅保留字体族名
- **本地化**：从 TTF 文件的 Name Table 提取多语言显示名称；异步后台执行，不阻塞 `List()` 返回
- **缓存**：使用 `sync.Once` 缓存扫描结果，避免重复 Registry 操作

### Testing Requirements
- 依赖 Windows 环境测试
- Registry 操作可 mock（使用 `golang.org/x/sys/windows/registry` 的可测试接口）
- 字体 TTF 解析可用示例字体文件测试

### Common Patterns
- 错误处理：字体名称本地化失败时回退到英文 Registry 名称
- 样式后缀列表：`" ExtraBold"`、`" Bold"`、`" Italic"` 等，按优先级移除

## Dependencies
### Internal
- 无

### External
- Win: `golang.org/x/sys/windows/registry` — Registry 操作
- darwin: 仅标准库 `os`/`path/filepath`/`io/fs`/`encoding/binary`
- 共用: `os`/`path/filepath` — 文件读取与路径处理

<!-- MANUAL: -->
