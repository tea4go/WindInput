<!-- Parent: ../AGENTS.md -->
<!-- Updated: 2026-05-12 -->

# cmdbar

## Purpose
命令直通车 (Command Bar) 的核心库实现, 给 wind_input 的快捷短语扩展一个轻量表达式语言: 字面短语 / `{expr}` 模板 / `$CC(display, action...)` 命令三种形态共用一套解析器与求值器。完整规约见 `docs/design/2026-05-12-command-bar-design.md`。

本目录是**纯 Go 库**, 不持有任何全局状态, 也不直接产生副作用。动作函数 (§3.4) 通过 `Services` 接口集获得依赖, 由宿主侧 (P4 起 coordinator) 注入到 `EvalContext.Services()`; P3 完成 9 个动作 (`type / open / run / shell / key.tap / key.seq / clip.copy / clip.paste / search`) 的纯库实现, 真正连线在 P4。

## Key Files
| File | Description |
|------|-------------|
| `context.go` | `EvalContext` 接口 (含 `Services()`) 与 `MemoryContext` 测试实现; 含环形 `History` (容量自定, 默认 16) |
| `services.go` | 动作函数所需依赖接口集: `ClipboardService` / `KeyInjector` / `URLOpener` / `ProcessRunner` (含 `Shell` + `ShellEx(cmd, flags)`) / `DictService` / `IMEController` / `SearchEngine` 与 `Services` 聚合; `ErrServiceUnavailable` 用于缺失服务降级 |
| `registry.go` | `FuncSpec` 元信息 + 线程安全 `Registry`; 默认注册 §3.4-§3.5 副作用函数 stub (Pure=false, Eval 返回 ErrNotImplemented), 等 `funcs.RegisterActions` 调用后被真实实现覆盖 |
| `ast/ast.go` | `Expr` / `Phrase` 节点定义 (`StringLit`/`NumberLit`/`Ident`/`Call` + `LiteralPhrase`/`TemplatePhrase`/`CommandPhrase`) |
| `parser/lexer.go` | 手写词法; 字符串内 `{...}` 切出 interp 段, 支持 `\" \\ \{ \} \( \) \n \t \r` 转义 |
| `parser/parser.go` | 入口 `Parse(src) (Phrase, error)`; 顶层根据 `$CC(` 与 `{` 出现位置三选一 |
| `action.go` | P5 引入的 `ResolvedAction` 模型 (`Kind ActionEffect/ActionText` + `Run func() (string, error)`), 把动作区分为纯副作用与文本上屏 |
| `eval/eval.go` | `Evaluate(phrase, ctx, reg)` → display + `[]cmdbar.ResolvedAction`; display 表达式做 Pure 函数白名单校验; `type(...)` 由 eval 直接拦截构造为 `ActionText`, 不走 registry.Lookup |
| `funcs/value.go` | §3.1 取值函数 (`code/tail/last/clip/sel/app/title/date/time/now/env`); `code` 返回触发候选时的 inputBuffer 快照, 旧名 `input` 已迁移 |
| `funcs/text.go` | §3.2 文本处理 (`len/upper/lower/trim/sub/replace/regex/split/concat/reverse/url/html/json/base64/default`); `t2s/s2t/pinyin` 为占位 stub |
| `funcs/calc.go` | §3.3 `calc` (递归下降算术求值, 支持 `+ - * / % ( )`, 空输入静默返回 `""` 无错) 与 `num` (2/8/10/16 进制互转) |
| `funcs/action.go` | §3.4 动作: `open / run / shell / key.tap / key.seq / clip.copy / clip.paste / search`; 每个函数从 `ctx.Services()` 取依赖, 缺失返回 `ErrServiceUnavailable`。`type` 在 P5 后由 eval 直接拦截为 `ActionText`, 不再走 registry。`shell(cmd[, flags])` 第二参可选 flag 字符串 (逗号分隔, 白名单 `term`/`pwsh`), 走 `Proc.ShellEx`; 1 参形式保留 `Proc.Shell` 旧通路 |
| `funcs/register.go` | `init` 把纯函数注册到 `DefaultRegistry`; `RegisterActions(reg)` 用真实 §3.4 实现覆盖 stub (调用方需主动调用, 避免对 Services 的隐式依赖) |

## Subdirectories
| Directory | Purpose |
|-----------|---------|
| `ast/` | 表达式/短语 AST 节点 |
| `parser/` | 词法 + 语法, 生成 AST |
| `eval/` | AST 求值器 + 显示名纯函数校验 |
| `funcs/` | 内建函数实现 (§3.1-§3.4 已落地, §3.5 ime/dict 等待 P4) |

## For AI Agents

### Working In This Directory
- 入口接口只有两个: `parser.Parse` 和 `eval.Evaluate`; 其余类型通过这两个入口暴露。
- 给注册表增加函数时, 同时填好 `Pure` 字段, 否则会被 display 校验拒绝或反过来污染显示名。
- 副作用函数在 `registry.go::registerSideEffectStubs` 默认占位; P3 的 9 个动作在 `funcs.RegisterActions(reg)` 中覆盖 stub, 保留 `Name/MinArgs/MaxArgs/Pure` 不变。剩余 stub (`dict.addword / ime.toggle / ime.setting / ask / pick`) 留到 P4-P5。
- 给宿主接线时, 实现 `Services` 各字段, 注入到 `EvalContext.Services()` 返回值即可; 缺失字段的动作会返回 `ErrServiceUnavailable`, 调用方据此降级。
- action 函数返回的 display 字符串恒为空, 求值器只把它们当 thunk; 不要把 action 调用放到 display 表达式里 (`assertPureDisplay` 会拦截)。
- action 参数表达式**延迟求值**: thunk 触发时再走 `evalExpr`, 这样 `type(last())` 等动作每次都拿到最新 history (`coordinator` 在 commit 后 Push, 下一次触发即反映)。
- 字符串字面量内的 `{expr}` 在词法层只做花括号匹配, **不**做转义解码; interp 段是 raw substring, 后由 parser 再次 lex/parse。这意味着写在 `{}` 里的子串与外层字符串脱钩, 内部嵌套字符串可以照常用 `"..."` 或 `'...'`。
- `History.Push` 互斥写, `EvalContext.Last` 同走互斥; 后续接 coordinator 时直接调用即可。

### Testing Requirements
- `cd wind_input && go test ./internal/cmdbar/...` 必须全绿。
- 新增内建函数请同步在 `funcs/*_test.go` 表驱动加用例 (典型 + 边界 + 错误)。
- 端到端覆盖通过 `eval/eval_test.go::TestEvaluate_DesignExamples` 跑设计文档 §3.6 全部条目。

### Common Patterns
- 用 `cmdbar.NewMemoryContext()` 构造测试 ctx; 设置 `Clock` 即可固定 `date/time/now` 输出。
- 注册函数典型形态:
  ```go
  {Name: "foo", MinArgs: 1, MaxArgs: 2, Pure: true, Eval: func(ctx, args) (string, error) { ... }}
  ```
- 1-based 索引参数通过 `funcs/text.go::resolve1Based` 处理, 支持负数倒数, 越界返回 ok=false。

## Dependencies

### Internal
- 仅依赖标准库 + `internal/cmdbar/ast`; 通过 `Services` 接口约束 (而非直接导入) 与 `internal/clipboard` / `internal/keyinject` / `internal/proc` / `internal/engine` / `internal/ui` 解耦。
- P4 已接通: `internal/coordinator/cmdbar_services.go` 装配 `Services` 字段, `cmdbar_context.go` 实现 `EvalContext`, `NewCoordinator` 调用 `installCmdbarPhraseHook` 把 (parser.Parse + eval.Evaluate) 注入到 `dict.PhraseLayer.SetCmdbarHook`; `recordCommit` 同步 `History.Push`, 让 `last()` 反映上一次 commit 文本。

### External
- 无第三方依赖。

## 全局约束

- 枚举与魔法字符串约束: 见 [`/docs/design/enum-constraint.md`](../../../docs/design/enum-constraint.md)。
- 日志隐私: 该目录目前不打日志; 后续接入时 INFO 级别禁止记录候选/输入文本, 仅记录元数据 (函数名、arity、错误码), 详见根 `CLAUDE.md`。

<!-- MANUAL: Any manually added notes below this line are preserved on regeneration -->
