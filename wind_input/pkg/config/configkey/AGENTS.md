<!-- Parent: ../AGENTS.md -->
<!-- Updated: 2026-06-11 -->

# pkg/config/configkey

## Purpose
配置 YAML 键路径的命名常量包，**键路径字符串的单一权威来源（SSOT）**。由 `pkg/config` 的 `GenerateConfigKeyConsts()` 反射 `Config` 结构体 yaml tag 生成，让调用方以编译期可校验的常量（如 `configkey.UiThemeName`）替代裸字符串 `"ui.theme.name"`，杜绝键路径写错 / 重构后漂移（历史上 config.set/toggle/applySection 曾因此出 bug）。

## Key Files
| File | Description |
|------|-------------|
| `keys_gen.go` | **生成文件，勿手改**（头部含 `DO NOT EDIT`）。全量键路径命名常量。命名规则：路径按 `.`/`_` 拆词、每词首字母大写后拼接（`ui.theme.name` → `UiThemeName`，不做 initialism 美化以保证无歧义、可机械逆推） |

## For AI Agents
- **不要手改** `keys_gen.go`。改 `Config` 结构体 yaml tag 后运行 `go test ./pkg/config -run TestExportKeyPaths -update` 重新生成（同步写出前端 `config-keys.json`）。
- 调用方读写配置项时引用本包常量而非裸字符串；常量名拼错会编译失败，重构 tag 后失配会被 `pkg/config` 的 `TestFieldsKeysAreValid` 抓出。
- 本包**无外部依赖**（纯字符串常量），可被任何包安全引用（含 `pkg/config` 自身，无循环）。

## Dependencies
### Internal
- 被 `internal/cmdbar/funcs`、`internal/coordinator`、`wind_setting`（app_service）等读写配置项的调用方引用。

### External
- 无。

## 全局约束
- 枚举与魔法字符串约束：见 [`/docs/design/enum-constraint.md`](../../../../docs/design/enum-constraint.md)。
