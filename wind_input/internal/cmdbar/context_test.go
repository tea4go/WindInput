package cmdbar

import "testing"

func TestHistory_Push_Get(t *testing.T) {
	h := NewHistory(3)
	if got := h.Get(1); got != "" {
		t.Errorf("empty Get(1) = %q", got)
	}
	h.Push("a")
	h.Push("b")
	if got := h.Get(1); got != "b" {
		t.Errorf("Get(1) = %q want b", got)
	}
	if got := h.Get(2); got != "a" {
		t.Errorf("Get(2) = %q want a", got)
	}
	if got := h.Get(3); got != "" {
		t.Errorf("Get(3) = %q want empty", got)
	}
	if got := h.Get(0); got != "" {
		t.Errorf("Get(0) = %q want empty", got)
	}
	h.Push("c")
	h.Push("d") // wraps around: ring now holds c, d (and pre-wrap b => evicted)
	if got := h.Get(1); got != "d" {
		t.Errorf("after wrap Get(1) = %q want d", got)
	}
	if got := h.Get(3); got != "b" {
		t.Errorf("after wrap Get(3) = %q want b", got)
	}
	if got := h.Get(4); got != "" {
		t.Errorf("after wrap Get(4) = %q want empty", got)
	}
	if h.Len() != 3 {
		t.Errorf("Len = %d", h.Len())
	}
}

func TestMemoryContext_Defaults(t *testing.T) {
	c := NewMemoryContext()
	if got := c.Last(1); got != "" {
		t.Errorf("Last(1) on empty = %q", got)
	}
	c.History.Push("hi")
	if got := c.Last(1); got != "hi" {
		t.Errorf("Last(1) = %q", got)
	}
}

func TestRegistry_StubArity(t *testing.T) {
	spec, ok := DefaultRegistry.Lookup("open")
	if !ok {
		t.Fatal("open not registered")
	}
	if spec.Pure {
		t.Errorf("open should be Pure=false")
	}
	if spec.Accepts(0) {
		t.Errorf("open should reject 0 args")
	}
	if !spec.Accepts(1) {
		t.Errorf("open should accept 1 arg")
	}
}
