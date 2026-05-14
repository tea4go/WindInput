package funcs

import (
	"testing"

	"github.com/huanfeng/wind_input/internal/cmdbar"
)

func TestCalc_Basic(t *testing.T) {
	spec, _ := cmdbar.DefaultRegistry.Lookup("calc")
	cases := map[string]string{
		"1+2*3":     "7",
		"(1+2)*3":   "9",
		"10/4":      "2.5",
		"10%3":      "1",
		"-3 + 4":    "1",
		"2 * -3":    "-6",
		"1.5 + 0.5": "2",
		"  3 + 4  ": "7",
	}
	for src, want := range cases {
		v, err := spec.Eval(cmdbar.NewMemoryContext(), []string{src})
		if err != nil {
			t.Errorf("calc(%q) err %v", src, err)
			continue
		}
		if v != want {
			t.Errorf("calc(%q) = %q want %q", src, v, want)
		}
	}
}

func TestCalc_EmptyInputSilent(t *testing.T) {
	spec, _ := cmdbar.DefaultRegistry.Lookup("calc")
	for _, src := range []string{"", " ", "   \t\n  "} {
		v, err := spec.Eval(cmdbar.NewMemoryContext(), []string{src})
		if err != nil {
			t.Errorf("calc(%q) want silent, got err %v", src, err)
		}
		if v != "" {
			t.Errorf("calc(%q) want empty string, got %q", src, v)
		}
	}
}

func TestCalc_Errors(t *testing.T) {
	spec, _ := cmdbar.DefaultRegistry.Lookup("calc")
	for _, src := range []string{"1+", "1/0", "(1+2", "abc"} {
		if _, err := spec.Eval(cmdbar.NewMemoryContext(), []string{src}); err == nil {
			t.Errorf("expected error for %q", src)
		}
	}
}

func TestNum_BaseConversion(t *testing.T) {
	spec, _ := cmdbar.DefaultRegistry.Lookup("num")
	cases := []struct {
		s, base, want string
	}{
		{"0xff", "10", "255"},
		{"255", "16", "ff"},
		{"10", "2", "1010"},
		{"0b1010", "10", "10"},
		{"0o17", "10", "15"},
	}
	for _, c := range cases {
		v, err := spec.Eval(cmdbar.NewMemoryContext(), []string{c.s, c.base})
		if err != nil {
			t.Errorf("num(%v) err %v", c, err)
			continue
		}
		if v != c.want {
			t.Errorf("num(%v) = %q want %q", c, v, c.want)
		}
	}
	if _, err := spec.Eval(cmdbar.NewMemoryContext(), []string{"10", "3"}); err == nil {
		t.Error("expected error for unsupported base")
	}
}
