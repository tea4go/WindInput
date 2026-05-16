package funcs

import "github.com/huanfeng/wind_input/internal/cmdbar"

// init registers all pure §3.1-§3.3 functions onto
// cmdbar.DefaultRegistry. Side-effect stubs are registered separately
// inside the cmdbar package itself.
func init() {
	for _, spec := range valueFuncs() {
		cmdbar.DefaultRegistry.Register(spec)
	}
	for _, spec := range textFuncs() {
		cmdbar.DefaultRegistry.Register(spec)
	}
	for _, spec := range calcFuncs() {
		cmdbar.DefaultRegistry.Register(spec)
	}
	// help / 内省函数 (Category=meta), 注册到 default registry。
	for _, spec := range helpFuncs() {
		cmdbar.DefaultRegistry.Register(spec)
	}
}

// aliasOf 把 canonical 拷贝一份, 改名为 oldName 并标 Deprecated, AliasOf
// 指向 canonical.Name; Eval / arity / Pure 等运行时字段保持不变, 保证行为
// 完全等价 (用户已存的 yaml 短语调旧名时仍能正确执行)。
//
// 设计 docs/design/2026-05-16-cmdbar-followup.md §1.2 命名宪法迁移表。
func aliasOf(canonical cmdbar.FuncSpec, oldName string) cmdbar.FuncSpec {
	aliased := canonical
	aliased.Name = oldName
	aliased.AliasOf = canonical.Name
	aliased.Deprecated = true
	aliased.Description = "(deprecated) 改用 " + canonical.Name
	aliased.ExampleSrc = "" // 不引导新用户用旧名
	return aliased
}

// RegisterActions installs the P3 action MVP (type / open / run /
// shell / key.tap / key.seq / clip.copy / clip.paste / search) onto
// reg, overwriting the package's pre-registered stubs (Pure=false,
// Eval returns ErrNotImplemented). After this call, evaluating the
// action expressions will dispatch to the real implementations, which
// resolve their Services from EvalContext.Services() at runtime.
//
// Pass cmdbar.DefaultRegistry for the package-level registry; tests
// can pass an isolated *Registry for hermetic verification.
func RegisterActions(reg *cmdbar.Registry) {
	for _, spec := range actionFuncs() {
		reg.Register(spec)
	}
	// P4 新增 dict.addword / ime.toggle / ime.setting 真实实现,
	// 覆盖 registerSideEffectStubs 的 ErrNotImplemented 占位。
	for _, spec := range dictIMEActionFuncs() {
		reg.Register(spec)
	}
}
