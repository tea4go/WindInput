package funcs

import (
	"testing"

	"github.com/huanfeng/wind_input/internal/cmdbar"
)

func textCall(t *testing.T, name string, args ...string) string {
	t.Helper()
	spec, ok := cmdbar.DefaultRegistry.Lookup(name)
	if !ok {
		t.Fatalf("%s not registered", name)
	}
	v, err := spec.Eval(cmdbar.NewMemoryContext(), args)
	if err != nil {
		t.Fatalf("%s(%v) err: %v", name, args, err)
	}
	return v
}

func TestLen_Upper_Lower(t *testing.T) {
	if v := textCall(t, "len", "hello"); v != "5" {
		t.Errorf("len = %q", v)
	}
	if v := textCall(t, "len", "中文"); v != "2" {
		t.Errorf("len rune = %q", v)
	}
	if v := textCall(t, "upper", "abc"); v != "ABC" {
		t.Errorf("upper = %q", v)
	}
	if v := textCall(t, "lower", "ABC"); v != "abc" {
		t.Errorf("lower = %q", v)
	}
}

func TestTrim(t *testing.T) {
	if v := textCall(t, "trim", "  hi \n"); v != "hi" {
		t.Errorf("trim space = %q", v)
	}
	if v := textCall(t, "trim", "##wow##", "#"); v != "wow" {
		t.Errorf("trim chars = %q", v)
	}
}

func TestSub(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"abcdef", "2"}, "bcdef"},
		{[]string{"abcdef", "2", "4"}, "bcd"},
		{[]string{"abcdef", "-2"}, "ef"},
		{[]string{"abcdef", "1", "-1"}, "abcdef"},
		{[]string{"中文测试", "2", "3"}, "文测"},
		{[]string{"abc", "10"}, ""},
	}
	for _, c := range cases {
		got := textCall(t, "sub", c.args...)
		if got != c.want {
			t.Errorf("sub(%v) = %q want %q", c.args, got, c.want)
		}
	}
}

func TestReplace_Regex_Split(t *testing.T) {
	if v := textCall(t, "replace", "a b a", "a", "X"); v != "X b X" {
		t.Errorf("replace = %q", v)
	}
	if v := textCall(t, "regex", "abc123", `\d+`, "*"); v != "abc*" {
		t.Errorf("regex = %q", v)
	}
	if v := textCall(t, "split", "a,b,c", ",", "2"); v != "b" {
		t.Errorf("split = %q", v)
	}
	if v := textCall(t, "split", "a,b,c", ",", "-1"); v != "c" {
		t.Errorf("split neg = %q", v)
	}
	if v := textCall(t, "split", "a,b,c", ",", "9"); v != "" {
		t.Errorf("split oob = %q", v)
	}
}

func TestConcat_Reverse_Default(t *testing.T) {
	if v := textCall(t, "concat", "a", "b", "c"); v != "abc" {
		t.Errorf("concat = %q", v)
	}
	if v := textCall(t, "concat"); v != "" {
		t.Errorf("concat() = %q", v)
	}
	if v := textCall(t, "reverse", "abc"); v != "cba" {
		t.Errorf("reverse = %q", v)
	}
	if v := textCall(t, "reverse", "中文"); v != "文中" {
		t.Errorf("reverse rune = %q", v)
	}
	if v := textCall(t, "default", "", "fallback"); v != "fallback" {
		t.Errorf("default empty = %q", v)
	}
	if v := textCall(t, "default", "real", "fallback"); v != "real" {
		t.Errorf("default real = %q", v)
	}
}

func TestEncodings(t *testing.T) {
	if v := textCall(t, "url", "a b"); v != "a+b" {
		t.Errorf("url = %q", v)
	}
	if v := textCall(t, "html", "<b>"); v != "&lt;b&gt;" {
		t.Errorf("html = %q", v)
	}
	if v := textCall(t, "json", `a"b`); v != `"a\"b"` {
		t.Errorf("json = %q", v)
	}
	if v := textCall(t, "base64", "abc"); v != "YWJj" {
		t.Errorf("base64 = %q", v)
	}
}

func TestT2S_S2T_Pinyin_Stubs(t *testing.T) {
	// P2 stubs should pass through unchanged.
	if v := textCall(t, "t2s", "繁體"); v != "繁體" {
		t.Errorf("t2s stub = %q", v)
	}
	if v := textCall(t, "s2t", "简体"); v != "简体" {
		t.Errorf("s2t stub = %q", v)
	}
	if v := textCall(t, "pinyin", "你好"); v != "你好" {
		t.Errorf("pinyin stub = %q", v)
	}
}
