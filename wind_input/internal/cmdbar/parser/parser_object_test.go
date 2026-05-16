package parser

import (
	"strings"
	"testing"

	"github.com/huanfeng/wind_input/internal/cmdbar/ast"
)

// TestParseCommand_OptionsBag 验证 $CC(...) 末尾 ObjectLit 被抽出为 Modifiers,
// AST.Actions 不再包含它; 显式 modifiers 覆盖 marker 默认。
func TestParseCommand_OptionsBag(t *testing.T) {
	cases := []struct {
		name      string
		src       string
		actions   int
		modifiers map[string]any
	}{
		{
			name:      "no options",
			src:       `$CC("d", open("x"))`,
			actions:   1,
			modifiers: nil,
		},
		{
			name:      "options-only no actions",
			src:       `$CC("d", {prefix: true})`,
			actions:   0,
			modifiers: map[string]any{"prefix": true},
		},
		{
			name:      "options after one action",
			src:       `$CC("d", open("x"), {prefix: true})`,
			actions:   1,
			modifiers: map[string]any{"prefix": true},
		},
		{
			name:      "multiple modifiers",
			src:       `$CC("d", open("x"), {prefix: true, expand: "always", async: false})`,
			actions:   1,
			modifiers: map[string]any{"prefix": true, "expand": "always", "async": false},
		},
		{
			name:      "$CC1 sugar injects prefix:true",
			src:       `$CC1("d", open("x"))`,
			actions:   1,
			modifiers: map[string]any{"prefix": true},
		},
		{
			name:      "explicit prefix:false overrides $CC1 sugar",
			src:       `$CC1("d", open("x"), {prefix: false})`,
			actions:   1,
			modifiers: map[string]any{"prefix": false},
		},
		{
			name:      "$CC1 sugar merges with extra modifier",
			src:       `$CC1("d", open("x"), {async: false})`,
			actions:   1,
			modifiers: map[string]any{"prefix": true, "async": false},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := Parse(c.src)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			cp, ok := p.(ast.CommandPhrase)
			if !ok {
				t.Fatalf("want CommandPhrase, got %T", p)
			}
			if got := len(cp.Actions); got != c.actions {
				t.Fatalf("actions count = %d, want %d (AST: %s)", got, c.actions, cp)
			}
			if !equalModifierMap(cp.Modifiers, c.modifiers) {
				t.Fatalf("modifiers = %v, want %v", cp.Modifiers, c.modifiers)
			}
		})
	}
}

// TestParseCommand_OptionsBag_RejectMidList 验证 ObjectLit 出现在中间位置必须报错。
func TestParseCommand_OptionsBag_RejectMidList(t *testing.T) {
	_, err := Parse(`$CC("d", {prefix: true}, open("x"))`)
	if err == nil {
		t.Fatal("expected error for mid-list ObjectLit, got nil")
	}
	if !strings.Contains(err.Error(), "last argument") {
		t.Fatalf("expected error mention 'last argument', got %v", err)
	}
}

// TestParseCommand_OptionsBag_RejectNonLiteralValue 验证 modifier value 限字面量,
// 嵌套 call / nested object / namespaced ident 都必须报错。
func TestParseCommand_OptionsBag_RejectNonLiteralValue(t *testing.T) {
	bads := []string{
		`$CC("d", {prefix: open("x")})`,      // call
		`$CC("d", {prefix: {nested: true}})`, // nested object
		`$CC("d", {prefix: ns.name})`,        // namespaced ident
		`$CC("d", {prefix: "v {interp}"})`,   // string interp banned
		`$CC("d", {prefix})`,                 // missing :value
		`$CC("d", {: true})`,                 // missing key
	}
	for _, src := range bads {
		_, err := Parse(src)
		if err == nil {
			t.Errorf("expected parse error for %q, got nil", src)
		}
	}
}

// TestParseObjectLit_EmptyAndTrailingComma 验证空 bag 与尾逗号都允许。
func TestParseObjectLit_EmptyAndTrailingComma(t *testing.T) {
	cases := []string{
		`$CC("d", {})`,
		`$CC("d", {prefix: true,})`,
		`$CC("d", {prefix: true, async: false,})`,
	}
	for _, src := range cases {
		t.Run(src, func(t *testing.T) {
			if _, err := Parse(src); err != nil {
				t.Fatalf("Parse(%q): %v", src, err)
			}
		})
	}
}

// TestParseObjectLit_ValueTypes 验证三种字面量值都按正确 Go 类型存入 Modifiers。
func TestParseObjectLit_ValueTypes(t *testing.T) {
	src := `$CC("d", {b: true, s: "hi", n: 42, sym: custom})`
	p, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	cp := p.(ast.CommandPhrase)
	if v, ok := cp.Modifiers["b"].(bool); !ok || v != true {
		t.Errorf("b: want bool true, got %T %v", cp.Modifiers["b"], cp.Modifiers["b"])
	}
	if v, ok := cp.Modifiers["s"].(string); !ok || v != "hi" {
		t.Errorf("s: want string \"hi\", got %T %v", cp.Modifiers["s"], cp.Modifiers["s"])
	}
	if v, ok := cp.Modifiers["n"].(float64); !ok || v != 42 {
		t.Errorf("n: want float64 42, got %T %v", cp.Modifiers["n"], cp.Modifiers["n"])
	}
	// 非 true/false 的裸 ident 当字符串符号保留 (host-defined enum)
	if v, ok := cp.Modifiers["sym"].(string); !ok || v != "custom" {
		t.Errorf("sym: want string \"custom\", got %T %v", cp.Modifiers["sym"], cp.Modifiers["sym"])
	}
}

// equalModifierMap 比较两个 modifier map 是否完全相等 (允许 nil == empty).
func equalModifierMap(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok || va != vb {
			return false
		}
	}
	return true
}
