package main

import "testing"

func TestResolveRunMode(t *testing.T) {
	cases := []struct {
		args []string
		want runMode
	}{
		{[]string{"--web"}, modeWeb},
		{[]string{"--gui"}, modeGUI},
		{[]string{}, modeGUI},
		{[]string{"--page", "about"}, modeGUI},
		{[]string{"--web", "--page", "x"}, modeWeb},
	}
	for _, c := range cases {
		if got := resolveRunMode(c.args); got != c.want {
			t.Fatalf("resolveRunMode(%v)=%v want %v", c.args, got, c.want)
		}
	}
}
