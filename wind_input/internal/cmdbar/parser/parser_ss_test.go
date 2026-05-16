package parser

import (
	"strings"
	"testing"

	"github.com/huanfeng/wind_input/internal/cmdbar/ast"
)

// TestParseArrayPhrase_PureStrings 验证纯字符串元素的 $SS。
func TestParseArrayPhrase_PureStrings(t *testing.T) {
	src := `$SS("常用网址", "https://a.com", "https://b.com", "https://c.com")`
	p, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	ap, ok := p.(ast.ArrayPhrase)
	if !ok {
		t.Fatalf("want ArrayPhrase, got %T", p)
	}
	if ap.Name != "常用网址" {
		t.Errorf("Name = %q, want 常用网址", ap.Name)
	}
	if len(ap.Elements) != 3 {
		t.Fatalf("Elements count = %d, want 3", len(ap.Elements))
	}
	for i, e := range ap.Elements {
		if _, ok := e.(ast.StringLit); !ok {
			t.Errorf("element %d: want StringLit, got %T", i, e)
		}
	}
	// marker default modifiers: prefix=true, expand="exact", nav=true
	if v, ok := ap.Modifiers["prefix"].(bool); !ok || !v {
		t.Errorf("default prefix modifier missing or wrong: %v", ap.Modifiers["prefix"])
	}
	if v, ok := ap.Modifiers["nav"].(bool); !ok || !v {
		t.Errorf("default nav modifier missing or wrong: %v", ap.Modifiers["nav"])
	}
}

// TestParseArrayPhrase_WithEmbeddedCC 验证 $SS 允许嵌入 $CC 元素。
func TestParseArrayPhrase_WithEmbeddedCC(t *testing.T) {
	src := `$SS("百度", $CC("打开", open("https://baidu.com")), "https://baidu.com", $CC("搜索 {tail(code,2)}", open("x")))`
	p, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	ap, ok := p.(ast.ArrayPhrase)
	if !ok {
		t.Fatalf("want ArrayPhrase, got %T", p)
	}
	if len(ap.Elements) != 3 {
		t.Fatalf("Elements count = %d, want 3", len(ap.Elements))
	}
	if _, ok := ap.Elements[0].(ast.CommandPhrase); !ok {
		t.Errorf("element 0: want CommandPhrase, got %T", ap.Elements[0])
	}
	if _, ok := ap.Elements[1].(ast.StringLit); !ok {
		t.Errorf("element 1: want StringLit, got %T", ap.Elements[1])
	}
	if _, ok := ap.Elements[2].(ast.CommandPhrase); !ok {
		t.Errorf("element 2: want CommandPhrase, got %T", ap.Elements[2])
	}
}

// TestParseArrayPhrase_RejectsEmbeddedPrefixModifier 验证嵌入 $CC 中
// 含 prefix modifier 时解析报错 (group prefix 由外层 $SS 控制)。
func TestParseArrayPhrase_RejectsEmbeddedPrefixModifier(t *testing.T) {
	src := `$SS("g", $CC("d", open("x"), {prefix: true}))`
	_, err := Parse(src)
	if err == nil {
		t.Fatal("expected error for embedded $CC with prefix modifier, got nil")
	}
	if !strings.Contains(err.Error(), "prefix") {
		t.Errorf("error should mention 'prefix', got: %v", err)
	}
}

// TestParseArrayPhrase_RequiresStringName 验证第一参必须是 string lit。
func TestParseArrayPhrase_RequiresStringName(t *testing.T) {
	bads := []string{
		`$SS(notString, "a")`,      // bare ident
		`$SS(123, "a")`,            // number
		`$SS($CC("d", open("x")))`, // CC as first arg
	}
	for _, src := range bads {
		_, err := Parse(src)
		if err == nil {
			t.Errorf("Parse(%q) should fail, got nil", src)
		}
	}
}

// TestParseArrayPhrase_ExplicitOptionsOverride 验证显式 options 覆盖 marker 默认。
func TestParseArrayPhrase_ExplicitOptionsOverride(t *testing.T) {
	src := `$SS("g", "a", "b", {prefix: false, nav: false})`
	p, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	ap := p.(ast.ArrayPhrase)
	if v, _ := ap.Modifiers["prefix"].(bool); v {
		t.Errorf("prefix should be overridden to false")
	}
	if v, _ := ap.Modifiers["nav"].(bool); v {
		t.Errorf("nav should be overridden to false")
	}
	// expand 默认值 "exact" 未被覆盖, 应保留
	if v, _ := ap.Modifiers["expand"].(string); v != "exact" {
		t.Errorf("expand = %q, want \"exact\" (marker default)", v)
	}
}

// TestParseArrayPhrase_OnlyName 验证只给 name 不给元素是允许的 (后续 SS 短语
// 解析为空 elements 列表, 运行时展开 0 候选, 不报错)。
func TestParseArrayPhrase_OnlyName(t *testing.T) {
	p, err := Parse(`$SS("empty")`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	ap := p.(ast.ArrayPhrase)
	if ap.Name != "empty" {
		t.Errorf("Name = %q, want \"empty\"", ap.Name)
	}
	if len(ap.Elements) != 0 {
		t.Errorf("Elements count = %d, want 0", len(ap.Elements))
	}
}
