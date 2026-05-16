package dict

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/huanfeng/wind_input/internal/cmdbar"
)

// TestPhraseLayerCmdbarHookFiresOnCC 验证 PhraseLayer.SearchCommand 在遇到
// 含 "$CC(" 的短语 value 时, 会调用注入的 hook 并把返回的 display/actions
// 装到候选上; hook 没注入或 value 不含 $CC( 时不应触发。
func TestPhraseLayerCmdbarHookFiresOnCC(t *testing.T) {
	tmpDir := t.TempDir()
	systemFile := filepath.Join(tmpDir, "system.phrases.yaml")
	content := `phrases:
  - code: "ocbd"
    text: "$CC(\"打开百度\", open(\"https://baidu.com\"))"
    position: 1
`
	if err := os.WriteFile(systemFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	pl := loadPhraseLayerFromYAML(t, systemFile, "")

	// 未注入 hook 前: 应当走旧 templateEngine 路径, 候选无 Actions。
	resultsNoHook := pl.SearchCommand("ocbd", 10)
	if len(resultsNoHook) == 0 {
		t.Fatal("SearchCommand should return at least one candidate")
	}
	if len(resultsNoHook[0].Actions) != 0 {
		t.Fatalf("without hook, Actions must be empty; got %d", len(resultsNoHook[0].Actions))
	}

	pl.InvalidateCache()

	// 注入 hook: 返回固定 display + 1 个 ActionEffect。
	var actionFired int32
	hook := func(value string) (string, []cmdbar.ResolvedAction, map[string]any, bool, error) {
		return "打开百度", []cmdbar.ResolvedAction{
			{Kind: cmdbar.ActionEffect, Run: func() (string, error) {
				atomic.AddInt32(&actionFired, 1)
				return "", nil
			}},
		}, nil, true, nil
	}
	pl.SetCmdbarHook(hook)

	results := pl.SearchCommand("ocbd", 10)
	if len(results) == 0 {
		t.Fatal("SearchCommand should return at least one candidate with hook installed")
	}
	cand := results[0]
	if cand.DisplayText != "打开百度" {
		t.Fatalf("expected DisplayText=%q, got %q", "打开百度", cand.DisplayText)
	}
	if cand.Text != cand.DisplayText {
		t.Fatalf("expected Text == DisplayText, got Text=%q DisplayText=%q",
			cand.Text, cand.DisplayText)
	}
	if !cand.IsCommand {
		t.Fatal("cmdbar candidate must keep IsCommand=true")
	}
	if len(cand.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(cand.Actions))
	}

	// 触发 action, 校验闭包真的被执行。
	if _, err := cand.Actions[0].Run(); err != nil {
		t.Fatalf("action returned error: %v", err)
	}
	if atomic.LoadInt32(&actionFired) != 1 {
		t.Fatalf("expected action to fire once, got %d", actionFired)
	}
}

// TestPhraseLayerCmdbarHookFallbackOnError 验证 hook 返回 err 时,
// SearchCommand 退化为字面量短语 (Text == value) 而不是中断。
func TestPhraseLayerCmdbarHookFallbackOnError(t *testing.T) {
	tmpDir := t.TempDir()
	systemFile := filepath.Join(tmpDir, "system.phrases.yaml")
	content := `phrases:
  - code: "bad"
    text: "$CC(broken-expr"
    position: 1
`
	if err := os.WriteFile(systemFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	pl := loadPhraseLayerFromYAML(t, systemFile, "")
	pl.SetCmdbarHook(func(value string) (string, []cmdbar.ResolvedAction, map[string]any, bool, error) {
		return "", nil, nil, true, errSentinel
	})

	results := pl.SearchCommand("bad", 10)
	if len(results) == 0 {
		t.Fatal("fallback path should still return a candidate")
	}
	if results[0].Text != `$CC(broken-expr` {
		t.Fatalf("expected literal fallback, got Text=%q", results[0].Text)
	}
	if len(results[0].Actions) != 0 {
		t.Fatal("fallback candidate must have no actions")
	}
}

// TestPhraseLayerCmdbarHookSkipsNonCC 验证 value 不含 "$CC(" 时,
// 即使注入了 hook 也仍走 templateEngine 老路径 (确保完全兼容)。
func TestPhraseLayerCmdbarHookSkipsNonCC(t *testing.T) {
	tmpDir := t.TempDir()
	systemFile := filepath.Join(tmpDir, "system.phrases.yaml")
	content := `phrases:
  - code: "rq"
    text: "$Y-$MM-$DD"
    position: 1
`
	if err := os.WriteFile(systemFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	pl := loadPhraseLayerFromYAML(t, systemFile, "")
	called := false
	pl.SetCmdbarHook(func(value string) (string, []cmdbar.ResolvedAction, map[string]any, bool, error) {
		called = true
		return "X", nil, nil, true, nil
	})
	results := pl.SearchCommand("rq", 10)
	if called {
		t.Fatal("hook must not be invoked for non-$CC dynamic phrases")
	}
	if len(results) == 0 {
		t.Fatal("expected templateEngine path candidate")
	}
	if len(results[0].Actions) != 0 {
		t.Fatal("non-$CC candidate must have no actions")
	}
}

// errSentinel 是测试用的固定错误值, 避免引入 errors 包的散用法。
type sentinelErr string

func (e sentinelErr) Error() string { return string(e) }

var errSentinel = sentinelErr("test sentinel error")

// stubCmdbarHook 返回固定 display 的 hook, 便于 prefix_nav 测试断言展开行为。
func stubCmdbarHook(display string) CmdbarPhraseHook {
	return func(value string) (string, []cmdbar.ResolvedAction, map[string]any, bool, error) {
		return display, []cmdbar.ResolvedAction{
			{Kind: cmdbar.ActionEffect, Run: func() (string, error) { return "", nil }},
		}, nil, true, nil
	}
}

// TestPhraseLayerSearchPrefix_CC_NotVisible 验证: 含 $CC( (仅精确) marker 的命令短语
// 不应出现在 SearchPrefix("co") 结果中。
func TestPhraseLayerSearchPrefix_CC_NotVisible(t *testing.T) {
	tmpDir := t.TempDir()
	systemFile := filepath.Join(tmpDir, "system.phrases.yaml")
	content := `phrases:
  - code: "cogh"
    text: "$CC(\"打开 GitHub\", open(\"https://github.com\"))"
    position: 2
`
	if err := os.WriteFile(systemFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	pl := loadPhraseLayerFromYAML(t, systemFile, "")
	pl.SetCmdbarHook(stubCmdbarHook("X"))

	results := pl.SearchPrefix("co", 0)
	for _, c := range results {
		if c.Code == "cogh" {
			t.Fatalf("$CC( 短语不应出现在前缀候选, 得到 %+v", c)
		}
	}
}

// TestPhraseLayerSearchPrefix_CC1_Visible 验证: 含 $CC1( (前缀可见) marker 的命令短语
// 应出现在 SearchPrefix("co") 结果中, 候选 Actions 已挂。
func TestPhraseLayerSearchPrefix_CC1_Visible(t *testing.T) {
	tmpDir := t.TempDir()
	systemFile := filepath.Join(tmpDir, "system.phrases.yaml")
	content := `phrases:
  - code: "cobd"
    text: "$CC1(\"打开百度\", open(\"https://baidu.com\"))"
    position: 1
  - code: "cogh"
    text: "$CC(\"打开 GitHub\", open(\"https://github.com\"))"
    position: 2
`
	if err := os.WriteFile(systemFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	pl := loadPhraseLayerFromYAML(t, systemFile, "")
	pl.SetCmdbarHook(stubCmdbarHook("打开 X"))

	results := pl.SearchPrefix("co", 0)
	gotCodes := map[string]bool{}
	for _, c := range results {
		gotCodes[c.Code] = true
	}
	if !gotCodes["cobd"] {
		t.Fatalf("$CC1( 短语 cobd 应出现在前缀候选, 实际 %v", gotCodes)
	}
	if gotCodes["cogh"] {
		t.Fatalf("$CC( 短语 cogh 不应出现在前缀候选, 实际 %v", gotCodes)
	}
}

// TestPhraseLayerSearchPrefix_NonCmdbarDynamicIgnored 验证: 普通 $X 模板 (date 等)
// 不应在前缀路径出现, 维持精确匹配语义。
func TestPhraseLayerSearchPrefix_NonCmdbarDynamicIgnored(t *testing.T) {
	tmpDir := t.TempDir()
	systemFile := filepath.Join(tmpDir, "system.phrases.yaml")
	content := `phrases:
  - code: "corq"
    text: "$Y-$MM-$DD"
    position: 1
`
	if err := os.WriteFile(systemFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	pl := loadPhraseLayerFromYAML(t, systemFile, "")
	pl.SetCmdbarHook(stubCmdbarHook("X"))

	results := pl.SearchPrefix("co", 0)
	for _, c := range results {
		if c.Code == "corq" {
			t.Fatal("普通 $X 模板短语 corq 不应出现在前缀候选")
		}
	}
}
