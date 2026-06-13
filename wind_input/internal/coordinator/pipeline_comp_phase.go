// pipeline_comp_phase.go — CompositionPhase 推导与 Capability diff 的纯函数（可表驱动单测）。
//
// 本批次（temp_pinyin 全接管）只用这两个纯函数做**只读遥测**（decider.logSwitch），不驱动
// 真实引擎/composition 副作用——后者仍由既有 setup/exit/clearState 路径管。第 2 批
// applyEngineDiff 落地时，computeCapabilityDiff 升格为真实挂卸的唯一计算点（设计文档七·2）。
package coordinator

// computeCapabilityDiff 计算宿主切换 old→new 时需新增挂载与卸载的引擎资源位集。
//
//	added   = new &^ old  （new 有而 old 无 → 需挂载，如进入拼音层）
//	removed = old &^ new  （old 有而 new 无 → 需卸载，如离开拼音层）
//
// 去抖关键：old、new 都含某 cap → 既不在 added 也不在 removed（解决 z fallback 反复横跳的
// 拼音层抖动）。纯函数，无副作用。
func computeCapabilityDiff(old, new Capability) (added, removed Capability) {
	return new &^ old, old &^ new
}

// deriveCompositionPhase 据「宿主切换」推导 composition 生命周期阶段（设计文档六·推导规则）。
//
// host 永不为空下「无 composition」== host 是 engine_default（且 buffer 空，但 phase 推导只需
// 宿主身份即可区分冷/热/结束三类切换；是否上屏由调用方据 CommitIdx 另行判定 CompCommit）：
//
//	engineDefault → 模式      ：CompCold（无→有，StartComposition + armPendingFirstShow）
//	模式A         → 模式B      ：CompHot （有→有，仅换宿主，窗口不变，不重 arm）
//	模式         → engineDefault：CompEnd （有→无，clearState + hideUI）
//	engineDefault → engineDefault：CompCold 退化（无变化，调用方一般不触发切换）
//
// 顶码上屏（CompCommit）需 CommitIdx≥0 的额外信息，不在本纯函数职责内——本批 temp_pinyin
// 触发/回退均不顶码，故只产出 Cold/Hot/End。
func deriveCompositionPhase(from, to, engineDefault Processor) CompositionPhase {
	fromDefault := from == engineDefault
	toDefault := to == engineDefault
	switch {
	case fromDefault && !toDefault:
		return CompCold
	case !fromDefault && toDefault:
		return CompEnd
	case !fromDefault && !toDefault:
		return CompHot
	default:
		return CompCold
	}
}
