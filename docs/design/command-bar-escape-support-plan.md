# 命令直通车转义字符支持 — 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让命令直通车的纯字面短语支持 `\n \r \t \\` 转义, 从而支持多行文本短语, 并在候选框渲染层防御换行符撑破布局。

**Architecture:** 新增统一解码器 `cmdbar.DecodeEscapes` (宽松白名单); 三条短语路径接入 —— `$CC`/`$SS` 字符串字面量与模板短语的 lexer/parser 把未知转义由报错改为原样保留, 纯字面短语在 dict 短语层候选 `Text` 定型处解码; 候选渲染层把换行符替换为可见符号 `↵`。

**Tech Stack:** Go (标准库); 模块路径 `github.com/huanfeng/wind_input`。

**设计依据:** `docs/design/command-bar-escape-support.md`

**通用约定:**
- 每个 Go 文件改完跑 `go fmt`。
- 测试命令在 `wind_input/` 目录下执行。
- 提交信息用 conventional commit, 不带 Co-Authored-By 或任何 AI trailer。
- 每个 Task 测试通过后再 commit。

---

### Task 1: 统一转义解码器 `cmdbar.DecodeEscapes`

**Files:**
- Create: `wind_input/internal/cmdbar/escape.go`
- Test: `wind_input/internal/cmdbar/escape_test.go`

- [ ] **Step 1: 写失败测试**

创建 `wind_input/internal/cmdbar/escape_test.go`:

```go
package cmdbar

import "testing"

func TestDecodeEscapes(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"no backslash fast path", "第一行第二行", "第一行第二行"},
		{"newline", `第一行\n第二行`, "第一行\n第二行"},
		{"tab", `a\tb`, "a\tb"},
		{"carriage return", `a\rb`, "a\rb"},
		{"literal backslash", `a\\b`, `a\b`},
		{"unknown escape kept literal", `C:\Users`, `C:\Users`},
		{"unknown escape with hit-letters", `C:\new`, "C:\\new"},
		{"trailing lone backslash", `abc\`, `abc\`},
		{"mixed", `行1\n行2\\tail`, "行1\n行2\\tail"},
		{"consecutive", `\n\n`, "\n\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DecodeEscapes(tc.in)
			if got != tc.want {
				t.Fatalf("DecodeEscapes(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
```

注意 `"unknown escape with hit-letters"`: `C:\new` 中 `\n` 是已知转义会被解码成换行 —— 这是宽松白名单策略下「正好撞上 `\n\r\t` 的路径会中招」的已接受行为, 故 want 是 `"C:\\new"` (即 `C:` + 换行 + `ew`)。

- [ ] **Step 2: 跑测试确认失败**

Run: `cd wind_input && go test ./internal/cmdbar/ -run TestDecodeEscapes -v`
Expected: 编译失败, `undefined: DecodeEscapes`

- [ ] **Step 3: 实现 `DecodeEscapes`**

创建 `wind_input/internal/cmdbar/escape.go`:

```go
package cmdbar

import "strings"

// DecodeEscapes 宽松解码短语字面文本中的转义序列。
// 识别: \n→LF, \r→CR, \t→TAB, \\→\
// 未知 \X (含 Windows 路径反斜杠) 连同反斜杠原样保留, 不报错。
// 末尾孤立的 \ 原样保留。无反斜杠时走快路径原样返回。
//
// \ + ASCII 字母是保留命名空间, 用户不应依赖未知 \X 保持字面值;
// 详见 docs/design/command-bar-escape-support.md §2.3。
func DecodeEscapes(s string) string {
	if strings.IndexByte(s, '\\') < 0 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' || i+1 >= len(s) {
			b.WriteByte(c)
			continue
		}
		switch s[i+1] {
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case '\\':
			b.WriteByte('\\')
		default:
			// 未知转义: 原样保留反斜杠与后续字符。
			b.WriteByte('\\')
			b.WriteByte(s[i+1])
		}
		i++
	}
	return b.String()
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd wind_input && go fmt ./internal/cmdbar/ && go test ./internal/cmdbar/ -run TestDecodeEscapes -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add wind_input/internal/cmdbar/escape.go wind_input/internal/cmdbar/escape_test.go
git commit -m "feat(cmdbar): 新增统一转义解码器 DecodeEscapes"
```

---

### Task 2: `$CC`/`$SS` 字符串字面量未知转义改为宽松

**Files:**
- Modify: `wind_input/internal/cmdbar/parser/lexer.go:236-237`
- Test: `wind_input/internal/cmdbar/parser/lexer_test.go`

- [ ] **Step 1: 写失败测试**

在 `wind_input/internal/cmdbar/parser/lexer_test.go` 末尾追加:

```go
func TestScanString_UnknownEscapeLenient(t *testing.T) {
	// 未知转义 \p 不再报错, 原样保留为反斜杠 + p。
	toks, err := NewLexer(`"C:\path"`).Tokenize()
	if err != nil {
		t.Fatalf("Tokenize returned error: %v", err)
	}
	if toks[0].Kind != tkString {
		t.Fatalf("token 0 kind = %v, want tkString", toks[0].Kind)
	}
	var lit string
	for _, p := range toks[0].Parts {
		if !p.IsInterp {
			lit += p.Lit
		}
	}
	if lit != `C:\path` {
		t.Fatalf("decoded literal = %q, want %q", lit, `C:\path`)
	}
}
```

若 `lexer_test.go` 不存在, 创建并加 `package parser` 与 `import "testing"` 头。

- [ ] **Step 2: 跑测试确认失败**

Run: `cd wind_input && go test ./internal/cmdbar/parser/ -run TestScanString_UnknownEscapeLenient -v`
Expected: FAIL —— 当前 `scanString` 对 `\p` 返回 `unknown escape \p` 错误。

- [ ] **Step 3: 改 `scanString` default 分支**

`wind_input/internal/cmdbar/parser/lexer.go` 中, `scanString` 的转义 switch (约 236-237 行) 把:

```go
			default:
				return Token{}, fmt.Errorf("unknown escape \\%c at offset %d", next, l.pos)
```

改为:

```go
			default:
				// 未知转义: 原样保留反斜杠与后续字符 (宽松白名单策略,
				// 与 cmdbar.DecodeEscapes 一致, 见
				// docs/design/command-bar-escape-support.md §2)。
				lit = append(lit, '\\', next)
```

- [ ] **Step 4: 跑测试确认通过 + 全包回归**

Run: `cd wind_input && go fmt ./internal/cmdbar/parser/ && go test ./internal/cmdbar/...`
Expected: PASS (含新测试与全部既有测试)。若有既有测试断言 "unknown escape" 报错, 它属于过时断言, 更新为新的宽松行为。

- [ ] **Step 5: 提交**

```bash
git add wind_input/internal/cmdbar/parser/lexer.go wind_input/internal/cmdbar/parser/lexer_test.go
git commit -m "feat(cmdbar): \$CC 字符串字面量未知转义改为原样保留"
```

---

### Task 3: 模板短语未知转义改为宽松

**Files:**
- Modify: `wind_input/internal/cmdbar/parser/parser.go:544-545`
- Test: `wind_input/internal/cmdbar/parser/parser_test.go`

- [ ] **Step 1: 写失败测试**

在 `wind_input/internal/cmdbar/parser/parser_test.go` 末尾追加:

```go
func TestParseTemplatePhrase_UnknownEscapeLenient(t *testing.T) {
	// 模板短语 (含 {expr}) 里的未知转义 \p 不再报错。
	ph, err := Parse(`路径 C:\path 时间 {now}`)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	tp, ok := ph.(ast.TemplatePhrase)
	if !ok {
		t.Fatalf("Parse returned %T, want ast.TemplatePhrase", ph)
	}
	sl, ok := tp.Expr.(ast.StringLit)
	if !ok {
		t.Fatalf("template expr is %T, want ast.StringLit", tp.Expr)
	}
	var lit string
	for _, part := range sl.Parts {
		if lp, ok := part.(ast.LiteralPart); ok {
			lit += lp.Text
		}
	}
	if lit != `路径 C:\path 时间 ` {
		t.Fatalf("decoded literal parts = %q, want %q", lit, `路径 C:\path 时间 `)
	}
}
```

确认测试文件已 `import` 了 `"github.com/huanfeng/wind_input/internal/cmdbar/ast"`; 若未 import 则补上。

- [ ] **Step 2: 跑测试确认失败**

Run: `cd wind_input && go test ./internal/cmdbar/parser/ -run TestParseTemplatePhrase_UnknownEscapeLenient -v`
Expected: FAIL —— 当前 `parseTemplatePhrase` 对 `\p` 返回 `unknown escape \p`。

- [ ] **Step 3: 改 `parseTemplatePhrase` default 分支**

`wind_input/internal/cmdbar/parser/parser.go` 中, `parseTemplatePhrase` 的转义 switch (约 544-545 行) 把:

```go
			default:
				return nil, fmt.Errorf("unknown escape \\%c at offset %d", next, i)
```

改为:

```go
			default:
				// 未知转义: 原样保留反斜杠与后续字符 (宽松白名单策略,
				// 与 cmdbar.DecodeEscapes 一致, 见
				// docs/design/command-bar-escape-support.md §2)。
				lit = append(lit, '\\', next)
```

- [ ] **Step 4: 跑测试确认通过 + 全包回归**

Run: `cd wind_input && go fmt ./internal/cmdbar/parser/ && go test ./internal/cmdbar/...`
Expected: PASS。若 `fmt` import 因不再使用而报 "imported and not used", 检查 `parser.go` 是否还有其他 `fmt` 用法 (有 —— `errf` 等); 正常不会触发。

- [ ] **Step 5: 提交**

```bash
git add wind_input/internal/cmdbar/parser/parser.go wind_input/internal/cmdbar/parser/parser_test.go
git commit -m "feat(cmdbar): 模板短语未知转义改为原样保留"
```

---

### Task 4: dict 短语层解码纯字面短语

**Files:**
- Create: `wind_input/internal/dict/phrase_escape.go`
- Modify: `wind_input/internal/dict/phrase.go` (`Search` 约 310 行, `expandDynamicEntry` 约 895-910 行)
- Test: `wind_input/internal/dict/phrase_escape_test.go`

**背景:** 纯字面短语 (`第一行\n第二行`, 无 `$`) 落在 `staticPhrases`, 经 `Search` 出口; 含 `$Y` 变量的非命令动态短语经 `expandDynamicEntry` 出口。两处 candidate `Text` 当前是原始文本。`$SS` 元素的转义已由 `scanString` 在解析期解码, 无需在此处理。含 marker (`$CC(`/`$CC1(`/`$SS(`/`$AA(`) 的文本不能提前解码 —— marker 内字符串字面量的转义由各自 parser 负责。

- [ ] **Step 1: 写失败测试**

创建 `wind_input/internal/dict/phrase_escape_test.go`:

```go
package dict

import "testing"

func TestDecodePhraseEscapes(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain newline", `第一行\n第二行`, "第一行\n第二行"},
		{"plain tab", `a\tb`, "a\tb"},
		{"no escape", "你好", "你好"},
		{"cmdbar marker untouched", `$CC("x\n", open("y"))`, `$CC("x\n", open("y"))`},
		{"aa marker untouched", `$AA("g", "ab\n")`, `$AA("g", "ab\n")`},
		{"ss marker untouched", `$SS("g", "a\nb")`, `$SS("g", "a\nb")`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := decodePhraseEscapes(tc.in)
			if got != tc.want {
				t.Fatalf("decodePhraseEscapes(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd wind_input && go test ./internal/dict/ -run TestDecodePhraseEscapes -v`
Expected: 编译失败, `undefined: decodePhraseEscapes`

- [ ] **Step 3: 实现 `decodePhraseEscapes`**

创建 `wind_input/internal/dict/phrase_escape.go`:

```go
package dict

import (
	"strings"

	"github.com/huanfeng/wind_input/internal/cmdbar"
)

// decodePhraseEscapes 对纯字面短语文本解码转义序列 (\n \r \t \\)。
//
// 含 cmdbar / 字符组 marker ($CC( $CC1( $SS( $AA() 的文本原样返回:
// marker 内字符串字面量的转义由各自 parser (cmdbar lexer / aa_marker)
// 处理, 不能在此处提前解码, 否则会破坏 marker 语法。
//
// 详见 docs/design/command-bar-escape-support.md §3.3。
func decodePhraseEscapes(text string) string {
	if HasCmdbarMarker(text) || strings.Contains(text, "$AA(") {
		return text
	}
	return cmdbar.DecodeEscapes(text)
}
```

注: `HasCmdbarMarker` 已在 `internal/dict` 包内定义 (见 `value_expand.go` 使用处); `internal/dict` 已 import `internal/cmdbar`。

- [ ] **Step 4: 跑测试确认通过**

Run: `cd wind_input && go fmt ./internal/dict/ && go test ./internal/dict/ -run TestDecodePhraseEscapes -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add wind_input/internal/dict/phrase_escape.go wind_input/internal/dict/phrase_escape_test.go
git commit -m "feat(dict): 新增纯字面短语转义解码 decodePhraseEscapes"
```

- [ ] **Step 6: 写 `Search` 出口集成失败测试**

在 `wind_input/internal/dict/phrase_escape_test.go` 末尾追加 (按既有 `phrase_test.go` 里 `PhraseLayer` / `staticPhrases` 的构造方式调整; 下方为参考结构, 实现时对齐既有测试 helper):

```go
func TestSearch_DecodesLiteralEscapes(t *testing.T) {
	pl := NewPhraseLayer()
	pl.staticPhrases = map[string][]PhraseEntry{
		"ml": {{Text: `第一行\n第二行`}},
	}
	got := pl.Search("ml", 10)
	if len(got) != 1 {
		t.Fatalf("Search returned %d candidates, want 1", len(got))
	}
	if got[0].Text != "第一行\n第二行" {
		t.Fatalf("candidate Text = %q, want decoded newline", got[0].Text)
	}
	if got[0].PhraseTemplate != `第一行\n第二行` {
		t.Fatalf("PhraseTemplate = %q, want raw escaped form", got[0].PhraseTemplate)
	}
}
```

实现前先看 `phrase_test.go` 确认 `PhraseLayer` 的构造入口 (`NewPhraseLayer` 或等价) 与 `staticPhrases` 字段填充方式, 对齐写法。

- [ ] **Step 7: 跑测试确认失败**

Run: `cd wind_input && go test ./internal/dict/ -run TestSearch_DecodesLiteralEscapes -v`
Expected: FAIL —— `Text` 仍是 `第一行\n第二行` 字面。

- [ ] **Step 8: 改 `Search` 候选构造**

`wind_input/internal/dict/phrase.go` 的 `Search` 中, candidate 构造 (约 309-316 行) 把:

```go
		cand := candidate.Candidate{
			Text:           e.Text,
			Code:           code,
			Weight:         resolvePhraseWeight(e.Weight),
			IsPhrase:       true, // 短语永远保留，但不计入 hasCommon 避免污染同编码码表字过滤
			PhraseTemplate: e.Text,
			ID:             PhraseCandidateID(code, e.Text),
		}
```

改为 (仅 `Text` 字段套 `decodePhraseEscapes`; `PhraseTemplate` 与 `ID` 仍用原始 `e.Text`):

```go
		cand := candidate.Candidate{
			Text:           decodePhraseEscapes(e.Text),
			Code:           code,
			Weight:         resolvePhraseWeight(e.Weight),
			IsPhrase:       true, // 短语永远保留，但不计入 hasCommon 避免污染同编码码表字过滤
			PhraseTemplate: e.Text,
			ID:             PhraseCandidateID(code, e.Text),
		}
```

- [ ] **Step 9: 跑测试确认通过**

Run: `cd wind_input && go fmt ./internal/dict/ && go test ./internal/dict/ -run TestSearch_DecodesLiteralEscapes -v`
Expected: PASS

- [ ] **Step 10: 改 `expandDynamicEntry` 候选构造**

`wind_input/internal/dict/phrase.go` 的 `expandDynamicEntry` 中, candidate 构造 (约 895-904 行) 把:

```go
	out := candidate.Candidate{
		Text:           res.Text,
		Code:           code,
		Weight:         resolvePhraseWeight(e.Weight),
		NaturalOrder:   e.LoadSeq, // 同 weight 下按 yaml 写入顺序输出 (SearchPrefix 路径 3 走 candidate.Better 时生效)
		IsCommand:      len(res.Actions) > 0,
		IsPhrase:       true,
		PhraseTemplate: e.Text,
		ID:             PhraseCandidateID(code, e.Text),
	}
```

改为 (非命令短语才解码; 命令短语的 `Text` 来自 cmdbar eval, 已由 `scanString` 解码, 不重复处理):

```go
	text := res.Text
	if !res.IsCommand {
		text = decodePhraseEscapes(text)
	}
	out := candidate.Candidate{
		Text:           text,
		Code:           code,
		Weight:         resolvePhraseWeight(e.Weight),
		NaturalOrder:   e.LoadSeq, // 同 weight 下按 yaml 写入顺序输出 (SearchPrefix 路径 3 走 candidate.Better 时生效)
		IsCommand:      len(res.Actions) > 0,
		IsPhrase:       true,
		PhraseTemplate: e.Text,
		ID:             PhraseCandidateID(code, e.Text),
	}
```

- [ ] **Step 11: 跑全包回归测试**

Run: `cd wind_input && go fmt ./internal/dict/ && go test ./internal/dict/...`
Expected: PASS

- [ ] **Step 12: 提交**

```bash
git add wind_input/internal/dict/phrase.go wind_input/internal/dict/phrase_escape_test.go
git commit -m "feat(dict): Search/expandDynamicEntry 解码纯字面短语转义"
```

---

### Task 5: 候选渲染层换行符防御

**Files:**
- Modify: `wind_input/internal/ui/renderer_layout.go:1-26`
- Test: `wind_input/internal/ui/renderer_layout_test.go`

**背景:** `candidateDisplayText` 是所有候选标签渲染的单一出口 (宽度测量与绘制都调它)。候选 `Text` 含真实换行符时, 单行控件会换行撑破布局, 需在此替换为可见符号 `↵`。`candidate.Text` 本身不改 —— 上屏多行文本仍多行。

- [ ] **Step 1: 写失败测试**

创建 `wind_input/internal/ui/renderer_layout_test.go`:

```go
package ui

import "testing"

func TestCandidateDisplayText_NewlineGlyph(t *testing.T) {
	cases := []struct {
		name string
		cand Candidate
		want string
	}{
		{"plain no newline", Candidate{Text: "你好"}, "你好"},
		{"lf replaced", Candidate{Text: "行1\n行2"}, "行1" + CandidateNewlineGlyph + "行2"},
		{"cr replaced", Candidate{Text: "行1\r行2"}, "行1" + CandidateNewlineGlyph + "行2"},
		{"crlf folds to one glyph", Candidate{Text: "行1\r\n行2"}, "行1" + CandidateNewlineGlyph + "行2"},
		{"command prefix kept", Candidate{Text: "打开", Actions: []ResolvedActionStub()}, CmdbarCandidatePrefix + "打开"},
	}
	_ = cases // 见下方说明
}
```

注意: `Candidate.Actions` 的元素类型是 `cmdbar.ResolvedAction` (`Candidate` = `candidate.Candidate` 类型别名)。上面 `ResolvedActionStub()` 是占位 —— 实现时改为构造一个非空 `Actions` 切片即可, 不依赖具体动作内容。落地写法:

```go
package ui

import "testing"

func TestCandidateDisplayText_NewlineGlyph(t *testing.T) {
	withAction := Candidate{Text: "打开"}
	withAction.Actions = make([]cmdbarResolvedActionType, 1) // 见下

	cases := []struct {
		name string
		cand Candidate
		want string
	}{
		{"plain no newline", Candidate{Text: "你好"}, "你好"},
		{"lf replaced", Candidate{Text: "行1\n行2"}, "行1" + CandidateNewlineGlyph + "行2"},
		{"cr replaced", Candidate{Text: "行1\r行2"}, "行1" + CandidateNewlineGlyph + "行2"},
		{"crlf folds to one glyph", Candidate{Text: "行1\r\n行2"}, "行1" + CandidateNewlineGlyph + "行2"},
		{"newline plus command prefix", withAction, CmdbarCandidatePrefix + "打开"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := candidateDisplayText(tc.cand); got != tc.want {
				t.Fatalf("candidateDisplayText = %q, want %q", got, tc.want)
			}
		})
	}
}
```

实现此测试时, 先 `grep -n "Actions" internal/candidate/*.go` 查清 `candidate.Candidate.Actions` 的元素类型 (来自 `internal/cmdbar`, 即 `cmdbar.ResolvedAction`), 把 `cmdbarResolvedActionType` 替换为正确类型并补 import; 构造 `make([]cmdbar.ResolvedAction, 1)` 即可让 `len(Actions) > 0` 成立。`withAction.Text` 不含换行, 该用例只验证前缀逻辑不被破坏。

- [ ] **Step 2: 跑测试确认失败**

Run: `cd wind_input && go test ./internal/ui/ -run TestCandidateDisplayText_NewlineGlyph -v`
Expected: 编译失败, `undefined: CandidateNewlineGlyph`

- [ ] **Step 3: 改 `renderer_layout.go`**

`wind_input/internal/ui/renderer_layout.go` 顶部 import 块加入 `"strings"`:

```go
import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/huanfeng/wind_input/pkg/config"
)
```

把 `candidateDisplayText` (约 19-26 行) 及其上方常量替换为:

```go
// CandidateNewlineGlyph 候选标签中换行符 (\r / \n) 的占位渲染符号。
// 候选框是单行控件, 候选文本含真实换行会撑破布局或与相邻候选重叠,
// 故渲染前统一替换为该符号。candidate.Text 本身不受影响 (上屏仍多行)。
// 不同字体渲染效果可能有差异, 后续可调 (如改用 ⏎ / ¶)。
const CandidateNewlineGlyph = "↵"

// candidateNewlineReplacer 把候选文本里的换行符折叠为 CandidateNewlineGlyph。
// \r\n 列在最前, NewReplacer 按参数顺序优先匹配, 保证 CRLF 折叠为单个符号。
var candidateNewlineReplacer = strings.NewReplacer(
	"\r\n", CandidateNewlineGlyph,
	"\r", CandidateNewlineGlyph,
	"\n", CandidateNewlineGlyph,
)

// candidateDisplayText 返回候选实际渲染到候选框的文本。
//   - 换行符 (\r / \n) 替换为 CandidateNewlineGlyph, 保证单行渲染;
//   - 命令直通车候选 (Actions 非空) 在文本前加 CmdbarCandidatePrefix。
// candidate 自身的 Text 字段保持原状, 仅渲染时变换, 避免污染历史记录与
// 右键菜单文案。
func candidateDisplayText(cand Candidate) string {
	text := candidateNewlineReplacer.Replace(cand.Text)
	if len(cand.Actions) > 0 {
		return CmdbarCandidatePrefix + text
	}
	return text
}
```

`CmdbarCandidatePrefix` 常量定义保持不变 (在同文件 17 行附近), 不要删除。

- [ ] **Step 4: 跑测试确认通过 + 全包回归**

Run: `cd wind_input && go fmt ./internal/ui/ && go test ./internal/ui/...`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add wind_input/internal/ui/renderer_layout.go wind_input/internal/ui/renderer_layout_test.go
git commit -m "feat(ui): 候选标签换行符渲染为 ↵ 防御单行布局"
```

---

### Task 6: 全量构建验证

**Files:** 无 (仅验证)

- [ ] **Step 1: 全量构建**

Run: `cd wind_input && go build ./...`
Expected: 无错误。

- [ ] **Step 2: 全量测试**

Run: `cd wind_input && go test ./...`
Expected: 全绿。若有失败, 回到对应 Task 修复。

- [ ] **Step 3: go vet**

Run: `cd wind_input && go vet ./internal/cmdbar/... ./internal/dict/... ./internal/ui/...`
Expected: 无报告。

---

### Task 7: 文档同步

**Files:**
- Modify: `docs/design/command-bar-design.md` (§2.1, §5)
- Modify: `wind_input/internal/cmdbar/AGENTS.md`

- [ ] **Step 1: 更新 `command-bar-design.md` §2.1 转义节**

`docs/design/command-bar-design.md` §2.1 当前转义说明是单行:

```
转义: `\"` `\\` `\{` `\}` `\(` `\)` 在字符串中按字面值处理。
```

替换为:

```
转义: `\"` `\\` `\{` `\}` `\(` `\)` `\n` `\t` `\r` 在字符串中按转义解码。
未知转义 (`\` + 其它字符) 原样保留反斜杠与后续字符, 不报错 —— 宽松白名单
策略, 三条短语路径 (`$CC`/`$SS` 字符串、模板短语、纯字面短语) 行为一致。
纯字面短语的转义解码由 dict 短语层的 `decodePhraseEscapes` 完成 (因为不含
`$` 的字面短语不经过 cmdbar parser)。完整方案见
`command-bar-escape-support.md`。

> **`\` + ASCII 字母是保留命名空间**: 用户不应依赖未知 `\X` 组合保持字面值。
> Windows 路径请写 `\\` (如 `C:\\Users`), 避免 `\n` `\t` `\r` 被解码。
```

- [ ] **Step 2: 更新 `command-bar-design.md` §5 显示名渲染节**

在 §5 末尾追加一条:

```
- 候选标签是单行控件。候选文本 (任何来源) 含换行符 (`\r` / `\n`) 时, 渲染层
  统一替换为可见符号 `↵` (`ui.CandidateNewlineGlyph`), 仅作用于显示, 候选
  实际上屏文本保留真实换行。
```

- [ ] **Step 3: 更新 `cmdbar/AGENTS.md`**

`wind_input/internal/cmdbar/AGENTS.md` 的 Key Files 表格新增一行 (放在 `registry.go` 行附近):

```
| `escape.go` | `DecodeEscapes(s)` 宽松转义解码器: `\n \r \t \\` 解码, 未知 `\X` 原样保留; 供 dict 短语层解码纯字面短语转义。详见 docs/design/command-bar-escape-support.md |
```

并在 `lexer.go` / `parser.go` 行的描述里把 "字符串内 `{...}` ... 支持 `\" \\ ...` 转义" 的说明补一句 "未知转义原样保留 (宽松白名单)"。

- [ ] **Step 4: 检查 AGENTS.md 引用**

Run: `pwsh -File scripts/lint_agents_md.ps1`
Expected: 无悬空引用报告 (若该脚本存在; 不存在则跳过)。

- [ ] **Step 5: 提交**

```bash
git add docs/design/command-bar-design.md wind_input/internal/cmdbar/AGENTS.md
git commit -m "docs(cmdbar): 同步转义字符支持设计到 design 文档与 AGENTS.md"
```

---

## 完成标准

- `cd wind_input && go build ./...` 与 `go test ./...` 全绿。
- 纯字面短语 `第一行\n第二行` 上屏为两行文本。
- 候选框中多行短语显示为单行, 换行处为 `↵`。
- 含 Windows 路径 (`C:\Users\name`) 的现存短语未被破坏 (`\U` `\n` 之外的反斜杠原样保留; `\n` 等正好撞上的属已接受的中招范围)。
- `$CC`/`$SS`/模板短语的未知转义不再报错。
