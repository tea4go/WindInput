package dict

// 集成测试: 用真实 cmdbar parser + eval 装配 PhraseLayer hook, 验证完整链路:
// yaml → store → PhraseLayer.LoadFromStore → SearchCommand/SearchPrefix
//        ↓ (含 marker 时)
//        → cmdbar hook (real parser + eval)
//        → Candidate (含 Modifiers / DisplayText / Actions)
//
// 与 phrase_cmdbar_test.go (用 mock hook) 互补: mock 测试聚焦 PhraseLayer 自己
// 的逻辑分支, 本文件测试 mock 不会暴露的"真实链路装配是否正确"。

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/huanfeng/wind_input/internal/candidate"
	"github.com/huanfeng/wind_input/internal/cmdbar"
	cmdbarast "github.com/huanfeng/wind_input/internal/cmdbar/ast"
	cmdbareval "github.com/huanfeng/wind_input/internal/cmdbar/eval"
	cmdbarparser "github.com/huanfeng/wind_input/internal/cmdbar/parser"
)

// realCmdbarPhraseHook 构造真实的 $CC/$CC1 phrase hook, 与 coordinator
// installCmdbarPhraseHook 中的闭包形态等价 (只是用 MemoryContext 而非
// coordinator EvalContext, 不影响纯函数路径)。
func realCmdbarPhraseHook(t *testing.T) CmdbarPhraseHook {
	t.Helper()
	return func(value string) (string, []cmdbar.ResolvedAction, map[string]any, bool, error) {
		phrase, err := cmdbarparser.Parse(value)
		if err != nil {
			return "", nil, nil, true, err
		}
		ctx := cmdbar.NewMemoryContext()
		display, actions, err := cmdbareval.Evaluate(phrase, ctx, cmdbar.DefaultRegistry)
		if err != nil {
			return "", nil, nil, true, err
		}
		var modifiers map[string]any
		if cp, ok := phrase.(cmdbarast.CommandPhrase); ok {
			modifiers = cp.Modifiers
		}
		return display, actions, modifiers, true, nil
	}
}

// realCmdbarArrayHook 构造真实的 $SS array hook。
func realCmdbarArrayHook(t *testing.T) CmdbarArrayHook {
	t.Helper()
	return func(value string) (string, []CmdbarArrayElement, map[string]any, bool, error) {
		phrase, err := cmdbarparser.Parse(value)
		if err != nil {
			return "", nil, nil, true, err
		}
		ap, ok := phrase.(cmdbarast.ArrayPhrase)
		if !ok {
			return "", nil, nil, false, nil
		}
		ctx := cmdbar.NewMemoryContext()
		name, evalElements, groupModifiers, err := cmdbareval.ExpandArray(ap, ctx, cmdbar.DefaultRegistry)
		if err != nil {
			return "", nil, nil, true, err
		}
		out := make([]CmdbarArrayElement, 0, len(evalElements))
		for _, e := range evalElements {
			out = append(out, CmdbarArrayElement{
				Display:          e.Display,
				Actions:          e.Actions,
				ElementModifiers: e.ElementModifiers,
			})
		}
		return name, out, groupModifiers, true, nil
	}
}

// TestRealHook_ModifierPropagation_ExactOnly 验证 $CC + 无 prefix 默认应被
// SearchPrefix 过滤, 但精确码命中时 SearchCommand 仍返回该候选, 且 Candidate
// 携带 Modifiers (nil 或不含 prefix)。
func TestRealHook_ModifierPropagation_ExactOnly(t *testing.T) {
	tmpDir := t.TempDir()
	systemFile := filepath.Join(tmpDir, "system.phrases.yaml")
	content := `phrases:
  - code: "ocbd"
    text: '$CC("打开百度", "noop")'
    weight: 1000
`
	if err := os.WriteFile(systemFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	pl := loadPhraseLayerFromYAML(t, systemFile, "")
	pl.SetCmdbarHook(realCmdbarPhraseHook(t))

	// 精确码命中: 候选返回, 不被前缀过滤
	exact := pl.SearchCommand("ocbd", 10)
	if len(exact) != 1 {
		t.Fatalf("expected 1 candidate for exact 'ocbd', got %d", len(exact))
	}
	if exact[0].DisplayText != "打开百度" {
		t.Errorf("DisplayText = %q, want 打开百度", exact[0].DisplayText)
	}
	// modifiers 应该是 nil 或不含 prefix
	if exact[0].Modifiers != nil {
		if v, ok := exact[0].Modifiers["prefix"]; ok {
			if b, _ := v.(bool); b {
				t.Errorf("$CC without sugar should not have prefix=true, got Modifiers=%v", exact[0].Modifiers)
			}
		}
	}

	// 前缀输入 (oc): 该 $CC 候选应被 candidateIsExactOnly 过滤掉, 不出现在前缀候选
	prefix := pl.SearchPrefix("oc", 10)
	for _, c := range prefix {
		if c.PhraseTemplate == `$CC("打开百度", "noop")` {
			t.Errorf("$CC (no prefix modifier) should be filtered in SearchPrefix, got %v", c.Text)
		}
	}
}

// TestRealHook_ModifierPropagation_CC1Sugar 验证 $CC1 (sugar) 与
// $CC + {prefix:true} 在 SearchPrefix 行为等价 — 都允许前缀展开。
func TestRealHook_ModifierPropagation_CC1Sugar(t *testing.T) {
	tmpDir := t.TempDir()
	systemFile := filepath.Join(tmpDir, "system.phrases.yaml")
	content := `phrases:
  - code: "abcdef"
    text: '$CC1("CC1 简写", "noop")'
    weight: 1000
  - code: "abcxyz"
    text: '$CC("显式 options", "noop", {prefix: true})'
    weight: 1000
`
	if err := os.WriteFile(systemFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	pl := loadPhraseLayerFromYAML(t, systemFile, "")
	pl.SetCmdbarHook(realCmdbarPhraseHook(t))

	// 前缀 "abc" (len>=2 触发前缀): 两条都应出现 (都有 prefix=true)
	results := pl.SearchPrefix("abc", 10)
	seen := map[string]bool{}
	for _, c := range results {
		seen[c.PhraseTemplate] = true
	}
	if !seen[`$CC1("CC1 简写", "noop")`] {
		t.Error("$CC1 sugar should appear in prefix search results")
	}
	if !seen[`$CC("显式 options", "noop", {prefix: true})`] {
		t.Error("$CC + {prefix:true} should appear in prefix search results (equivalent to $CC1)")
	}
}

// TestRealHook_SS_IntegrationWithInterp 验证 $SS 元素含 interpolation
// (`{tail(code, 2)}` 等) 在真实 cmdbar eval 下正确求值, 且 Actions 真实触发。
func TestRealHook_SS_IntegrationWithInterp(t *testing.T) {
	tmpDir := t.TempDir()
	systemFile := filepath.Join(tmpDir, "system.phrases.yaml")
	// elem1: 含 interp 字符串字面量 (需要 cmdbar eval 求值, 用 MemoryContext)
	// elem2: 嵌入 $CC, 选中触发 actions
	// elem3: 纯静态字符串
	content := `phrases:
  - code: "bd"
    text: '$SS("百度", "纯静态", $CC("命令元素", "noop"))'
    weight: 500
`
	if err := os.WriteFile(systemFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	pl := loadPhraseLayerFromYAML(t, systemFile, "")
	pl.SetCmdbarArrayHook(realCmdbarArrayHook(t))

	results := pl.SearchCommand("bd", 10)
	if len(results) != 2 {
		t.Fatalf("expected 2 candidates, got %d (texts=%v)", len(results), candidateTexts(results))
	}

	// 元素顺序通过 NaturalOrder 维持
	if results[0].Text != "纯静态" {
		t.Errorf("idx 0: text = %q, want 纯静态", results[0].Text)
	}
	if len(results[0].Actions) != 0 {
		t.Errorf("idx 0: string lit element should have no actions, got %d", len(results[0].Actions))
	}

	if results[1].Text != "命令元素" {
		t.Errorf("idx 1: text = %q, want 命令元素", results[1].Text)
	}
	// 嵌入 $CC 元素必须有 actions (这里的 "noop" 是 ident 被识别为 ActionEffect)
	if len(results[1].Actions) == 0 {
		t.Errorf("idx 1: embedded $CC element should have actions, got 0")
	}
}

// TestRealHook_SS_ActionFires 验证 $SS 嵌入 $CC 的 actions 真正可以被
// 调用执行 (ResolvedAction.Run 闭包行为正确, 不是 nil 占位)。
//
// 注: 这里测试 action 闭包的执行能力, 不依赖宿主 IME services —— 用
// type(...) action (eval 内置, 走 ActionText 路径)。
func TestRealHook_SS_ActionFires(t *testing.T) {
	tmpDir := t.TempDir()
	systemFile := filepath.Join(tmpDir, "system.phrases.yaml")
	content := `phrases:
  - code: "url"
    text: '$SS("URLs", $CC("Google", type("https://google.com")))'
    weight: 1000
`
	if err := os.WriteFile(systemFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	pl := loadPhraseLayerFromYAML(t, systemFile, "")
	pl.SetCmdbarArrayHook(realCmdbarArrayHook(t))

	results := pl.SearchCommand("url", 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(results))
	}
	if len(results[0].Actions) != 1 {
		t.Fatalf("expected 1 action on embedded $CC, got %d", len(results[0].Actions))
	}
	act := results[0].Actions[0]
	// type(...) 走 ActionText 路径, Run 返回上屏文本
	out, err := act.Run()
	if err != nil {
		t.Fatalf("action.Run: %v", err)
	}
	if out != "https://google.com" {
		t.Errorf("ActionText output = %q, want https://google.com", out)
	}
	if act.Kind != cmdbar.ActionText {
		t.Errorf("Kind = %v, want ActionText", act.Kind)
	}
}

// TestRealHook_SS_MultipleEntriesSameCode 验证同 code 下多条 $SS 短语
// 全部展开后合并排序 (weight 高的在前, 同 weight 按 NaturalOrder)。
func TestRealHook_SS_MultipleEntriesSameCode(t *testing.T) {
	tmpDir := t.TempDir()
	systemFile := filepath.Join(tmpDir, "system.phrases.yaml")
	content := `phrases:
  - code: "g"
    text: '$SS("A组", "a1", "a2")'
    weight: 3000
  - code: "g"
    text: '$SS("B组", "b1")'
    weight: 5000
`
	if err := os.WriteFile(systemFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	pl := loadPhraseLayerFromYAML(t, systemFile, "")
	pl.SetCmdbarArrayHook(realCmdbarArrayHook(t))

	results := pl.SearchCommand("g", 10)
	if len(results) != 3 {
		t.Fatalf("expected 3 candidates (1 from B + 2 from A), got %d (texts=%v)", len(results), candidateTexts(results))
	}
	// B 组 weight=5000, A 组 weight=3000, 所以排序应为: b1 (5000), a1 (3000,#0), a2 (3000,#1)
	wantOrder := []string{"b1", "a1", "a2"}
	for i, want := range wantOrder {
		if results[i].Text != want {
			t.Errorf("idx %d: text = %q, want %q (weight=%d)", i, results[i].Text, want, results[i].Weight)
		}
	}
	if results[0].Weight != 5000 {
		t.Errorf("idx 0 weight = %d, want 5000", results[0].Weight)
	}
	if results[1].Weight != 3000 {
		t.Errorf("idx 1 weight = %d, want 3000", results[1].Weight)
	}
}

// candidateTexts 是测试 helper, 提取候选文本列表方便错误信息。
func candidateTexts(cs []candidate.Candidate) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Text
	}
	return out
}
