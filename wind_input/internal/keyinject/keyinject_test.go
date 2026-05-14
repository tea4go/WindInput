package keyinject

import (
	"reflect"
	"testing"
)

func TestParse_HappyPaths(t *testing.T) {
	cases := []struct {
		in   string
		want Combo
	}{
		{"Enter", Combo{Key: "enter"}},
		{"enter", Combo{Key: "enter"}},
		{"Ctrl+C", Combo{Key: "c", Modifiers: []string{"ctrl"}}},
		{"ctrl+c", Combo{Key: "c", Modifiers: []string{"ctrl"}}},
		{"Ctrl+Shift+End", Combo{Key: "end", Modifiers: []string{"ctrl", "shift"}}},
		// Order canonicalisation: alt+ctrl → ctrl+alt
		{"Alt+Ctrl+Delete", Combo{Key: "delete", Modifiers: []string{"ctrl", "alt"}}},
		{"Win+L", Combo{Key: "l", Modifiers: []string{"win"}}},
		{"Shift+Tab", Combo{Key: "tab", Modifiers: []string{"shift"}}},
		{"F1", Combo{Key: "f1"}},
		{"f12", Combo{Key: "f12"}},
		{"/", Combo{Key: "slash"}},
		{".", Combo{Key: "period"}},
		{"-", Combo{Key: "minus"}},
		{"esc", Combo{Key: "escape"}},
		{"return", Combo{Key: "enter"}},
		{"page_up", Combo{Key: "pageup"}},
		{"PageDown", Combo{Key: "pagedown"}},
		{"Ctrl+Alt+Shift+Win+A", Combo{Key: "a", Modifiers: []string{"ctrl", "shift", "alt", "win"}}},
		{"  Ctrl + Shift + End  ", Combo{Key: "end", Modifiers: []string{"ctrl", "shift"}}},
		{"1", Combo{Key: "1"}},
		{"A", Combo{Key: "a"}},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := Parse(c.in)
			if err != nil {
				t.Fatalf("Parse(%q) err: %v", c.in, err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("Parse(%q) = %+v, want %+v", c.in, got, c.want)
			}
		})
	}
}

func TestParse_Errors(t *testing.T) {
	cases := []string{
		"",
		"   ",
		"Ctrl+",
		"+Enter",
		"Ctrl++Enter",
		"NoSuchKey",
		"Bogus+A",
		"f0",
		"f25",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			if _, err := Parse(c); err == nil {
				t.Errorf("Parse(%q) want error, got nil", c)
			}
		})
	}
}

func TestCombo_String(t *testing.T) {
	c := Combo{Key: "end", Modifiers: []string{"ctrl", "shift"}}
	if got := c.String(); got != "Ctrl+Shift+end" {
		t.Errorf("String() = %q", got)
	}
}
