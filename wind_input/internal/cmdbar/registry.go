package cmdbar

import (
	"errors"
	"fmt"
	"sync"
)

// ErrNotImplemented is returned by stub side-effect functions whose
// real implementation lives in later phases (P3+). Resolving a stub
// during evaluation surfaces this error so callers can degrade to the
// raw phrase.
var ErrNotImplemented = errors.New("function not implemented in this phase")

// EvalFunc is the signature of a registered command-bar function.
type EvalFunc func(ctx EvalContext, args []string) (string, error)

// FuncSpec is the metadata + entry point for a single registered
// function. MinArgs and MaxArgs are arity bounds (inclusive). MaxArgs
// may be -1 for variadic functions. Pure marks side-effect-free
// functions; only pure functions are permitted inside `$CC` display
// expressions (§5).
type FuncSpec struct {
	Name    string
	MinArgs int
	MaxArgs int // -1 for variadic
	Pure    bool
	Eval    EvalFunc
}

// Accepts reports whether n is within the spec's arity bounds.
func (f FuncSpec) Accepts(n int) bool {
	if n < f.MinArgs {
		return false
	}
	if f.MaxArgs >= 0 && n > f.MaxArgs {
		return false
	}
	return true
}

// Registry is a thread-safe map of function specs keyed by name. The
// default registry is populated at package init with all §3.1-§3.3
// functions plus stub entries for §3.4-§3.5 (Pure=false, Eval returns
// ErrNotImplemented) so phrase parsing's arity check passes.
type Registry struct {
	mu    sync.RWMutex
	specs map[string]FuncSpec
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{specs: make(map[string]FuncSpec)}
}

// Register inserts spec, overwriting any prior entry with the same
// name.
func (r *Registry) Register(spec FuncSpec) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.specs[spec.Name] = spec
}

// Lookup returns the spec for name and whether it was found.
func (r *Registry) Lookup(name string) (FuncSpec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.specs[name]
	return s, ok
}

// Names returns a snapshot of all registered function names. Mainly
// useful for diagnostics.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.specs))
	for k := range r.specs {
		out = append(out, k)
	}
	return out
}

// DefaultRegistry holds the package's default function set. The funcs
// subpackage populates it during init.
var DefaultRegistry = NewRegistry()

// stub returns an EvalFunc that always reports ErrNotImplemented. It is
// used to register side-effect placeholders that real P3+ wiring will
// later override.
func stub(name string) EvalFunc {
	return func(ctx EvalContext, args []string) (string, error) {
		return "", fmt.Errorf("%w: %s", ErrNotImplemented, name)
	}
}

// registerSideEffectStubs registers P3+ side-effecting functions as
// Pure=false stubs so the display-name purity check and the parse-time
// arity check work today.
func registerSideEffectStubs(r *Registry) {
	stubs := []FuncSpec{
		{Name: "type", MinArgs: 1, MaxArgs: 1, Pure: false, Eval: stub("type")},
		{Name: "open", MinArgs: 1, MaxArgs: 1, Pure: false, Eval: stub("open")},
		{Name: "run", MinArgs: 1, MaxArgs: -1, Pure: false, Eval: stub("run")},
		{Name: "shell", MinArgs: 1, MaxArgs: 1, Pure: false, Eval: stub("shell")},
		{Name: "key.tap", MinArgs: 1, MaxArgs: 1, Pure: false, Eval: stub("key.tap")},
		{Name: "key.seq", MinArgs: 1, MaxArgs: -1, Pure: false, Eval: stub("key.seq")},
		{Name: "clip.copy", MinArgs: 1, MaxArgs: 1, Pure: false, Eval: stub("clip.copy")},
		{Name: "clip.paste", MinArgs: 0, MaxArgs: 0, Pure: false, Eval: stub("clip.paste")},
		{Name: "dict.addword", MinArgs: 1, MaxArgs: 2, Pure: false, Eval: stub("dict.addword")},
		{Name: "ime.toggle", MinArgs: 1, MaxArgs: 1, Pure: false, Eval: stub("ime.toggle")},
		{Name: "ime.setting", MinArgs: 1, MaxArgs: 1, Pure: false, Eval: stub("ime.setting")},
		{Name: "search", MinArgs: 2, MaxArgs: 2, Pure: false, Eval: stub("search")},
		{Name: "ask", MinArgs: 1, MaxArgs: 1, Pure: false, Eval: stub("ask")},
		{Name: "pick", MinArgs: 1, MaxArgs: -1, Pure: false, Eval: stub("pick")},
	}
	for _, s := range stubs {
		r.Register(s)
	}
}

func init() {
	registerSideEffectStubs(DefaultRegistry)
}
