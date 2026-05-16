package funcs

import (
	"testing"

	"github.com/huanfeng/wind_input/internal/cmdbar"
)

// TestDefaultRegistry_AliasMapping 验证 RegisterActions 后, 命名宪法的 5 个
// 旧名 (run/shell/search/dict.addword/ime.setting) 都在 DefaultRegistry 中、
// 标 Deprecated=true、AliasOf 指向新名、arity 与 canonical 一致。
//
// 设计 docs/design/2026-05-16-cmdbar-followup.md §1.2。
func TestDefaultRegistry_AliasMapping(t *testing.T) {
	// 调用 RegisterActions 把 stubs 替换为真实 (alias 也被覆盖)。
	RegisterActions(cmdbar.DefaultRegistry)

	want := map[string]string{
		"run":          "proc.run",
		"shell":        "proc.shell",
		"search":       "web.search",
		"dict.addword": "dict.add",
		"ime.setting":  "setting.open",
	}
	for oldName, newName := range want {
		spec, ok := cmdbar.DefaultRegistry.Lookup(oldName)
		if !ok {
			t.Errorf("alias %q not in registry", oldName)
			continue
		}
		if !spec.Deprecated {
			t.Errorf("alias %q should be Deprecated=true", oldName)
		}
		if spec.AliasOf != newName {
			t.Errorf("alias %q AliasOf = %q, want %q", oldName, spec.AliasOf, newName)
		}
		// arity / Pure 应与 canonical 一致 (确保行为完全等价)
		canonical, ok := cmdbar.DefaultRegistry.Lookup(newName)
		if !ok {
			t.Errorf("canonical %q not in registry", newName)
			continue
		}
		if spec.MinArgs != canonical.MinArgs || spec.MaxArgs != canonical.MaxArgs {
			t.Errorf("alias %q arity (%d~%d) differs from canonical %q (%d~%d)",
				oldName, spec.MinArgs, spec.MaxArgs, newName, canonical.MinArgs, canonical.MaxArgs)
		}
	}
}
