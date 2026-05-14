package funcs

import (
	"testing"
	"time"

	"github.com/huanfeng/wind_input/internal/cmdbar"
)

// fixedCtx returns a MemoryContext with a deterministic clock at
// 2026-05-12 09:30:45.
func fixedCtx() *cmdbar.MemoryContext {
	c := cmdbar.NewMemoryContext()
	c.Clock = time.Date(2026, 5, 12, 9, 30, 45, 0, time.Local)
	return c
}

func call(t *testing.T, name string, args ...string) string {
	t.Helper()
	spec, ok := cmdbar.DefaultRegistry.Lookup(name)
	if !ok {
		t.Fatalf("%s not registered", name)
	}
	if !spec.Accepts(len(args)) {
		t.Fatalf("%s does not accept %d args (min=%d max=%d)", name, len(args), spec.MinArgs, spec.MaxArgs)
	}
	v, err := spec.Eval(fixedCtx(), args)
	if err != nil {
		t.Fatalf("%s(%v) err: %v", name, args, err)
	}
	return v
}

func TestDate_Aliases(t *testing.T) {
	if v := call(t, "date", "YYYY-MM-DD"); v != "2026-05-12" {
		t.Errorf("date = %q", v)
	}
	if v := call(t, "date", "YY/M/D"); v != "26/5/12" {
		t.Errorf("short fmt = %q", v)
	}
	if v := call(t, "date", "HH:mm:ss"); v != "09:30:45" {
		t.Errorf("hms = %q", v)
	}
}

func TestDate_Offset(t *testing.T) {
	cases := map[string]string{
		"+1d": "2026-05-13",
		"-2w": "2026-04-28",
		"+3M": "2026-08-12",
		"-1y": "2025-05-12",
	}
	for off, want := range cases {
		v := call(t, "date", "YYYY-MM-DD", off)
		if v != want {
			t.Errorf("offset %q = %q want %q", off, v, want)
		}
	}
}

func TestDate_BadOffset(t *testing.T) {
	spec, _ := cmdbar.DefaultRegistry.Lookup("date")
	_, err := spec.Eval(fixedCtx(), []string{"YYYY", "garbage"})
	if err == nil {
		t.Error("expected error for bad offset")
	}
}

func TestNow_Now(t *testing.T) {
	if v := call(t, "now"); v != "2026-05-12 09:30:45" {
		t.Errorf("now = %q", v)
	}
}

func TestTime_Default(t *testing.T) {
	if v := call(t, "time"); v != "09:30:45" {
		t.Errorf("time = %q", v)
	}
}

func TestCode_Tail_Last(t *testing.T) {
	c := fixedCtx()
	c.InputStr = "bd上海"
	c.History.Push("first")
	c.History.Push("second")

	spec, _ := cmdbar.DefaultRegistry.Lookup("code")
	if v, _ := spec.Eval(c, []string{"3"}); v != "上海" {
		t.Errorf("code(3) = %q", v)
	}
	if v, _ := spec.Eval(c, nil); v != "bd上海" {
		t.Errorf("code() = %q", v)
	}
	tailSpec, _ := cmdbar.DefaultRegistry.Lookup("tail")
	if v, _ := tailSpec.Eval(c, []string{"hello", "2"}); v != "ello" {
		t.Errorf("tail = %q", v)
	}
	lastSpec, _ := cmdbar.DefaultRegistry.Lookup("last")
	if v, _ := lastSpec.Eval(c, nil); v != "second" {
		t.Errorf("last() = %q", v)
	}
	if v, _ := lastSpec.Eval(c, []string{"2"}); v != "first" {
		t.Errorf("last(2) = %q", v)
	}
	if v, _ := lastSpec.Eval(c, []string{"5"}); v != "" {
		t.Errorf("last(5) = %q want empty", v)
	}
}

func TestClip_HistoryAndCurrent(t *testing.T) {
	c := fixedCtx()
	c.ClipStr = "current"
	c.ClipStack = []string{"current", "older", "oldest"}
	spec, _ := cmdbar.DefaultRegistry.Lookup("clip")
	if v, _ := spec.Eval(c, nil); v != "current" {
		t.Errorf("clip() = %q", v)
	}
	if v, _ := spec.Eval(c, []string{"2"}); v != "older" {
		t.Errorf("clip(2) = %q", v)
	}
	if v, _ := spec.Eval(c, []string{"9"}); v != "" {
		t.Errorf("clip(9) = %q want empty", v)
	}
}

func TestEnv(t *testing.T) {
	c := fixedCtx()
	c.EnvMap = map[string]string{"FOO": "bar"}
	spec, _ := cmdbar.DefaultRegistry.Lookup("env")
	if v, _ := spec.Eval(c, []string{"FOO"}); v != "bar" {
		t.Errorf("env(FOO) = %q", v)
	}
	if v, _ := spec.Eval(c, []string{"MISSING"}); v != "" {
		t.Errorf("env(MISSING) = %q", v)
	}
}
