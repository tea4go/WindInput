// pipeline_types.go — 输入处理器流水线核心类型（第 0 批地基）。
// 设计见 docs/design/input-processor-pipeline.md。
//
// 本批次只引入类型与接口骨架，不接入 HandleKeyEvent 主路径（feature flag 默认关闭，
// 影子运行待后续接入）。所有行为与现状逐条等价。
package coordinator

// Verdict 是决策器/处理器/按键处理单元对单个按键的裁决类型。
type Verdict int

const (
	// VerdictPass 不认领此按键，决策器继续问下一个处理单元。
	VerdictPass Verdict = iota
	// VerdictHandle 在当前宿主状态下处理此按键（不切换宿主）。
	VerdictHandle
	// VerdictActivate 切换宿主为本处理器后处理（可带顶码上屏）。
	VerdictActivate
	// VerdictRelease 当前宿主放弃这段 buffer，触发整链重判（带 Residual 交接）。
	VerdictRelease
)

func (v Verdict) String() string {
	switch v {
	case VerdictPass:
		return "Pass"
	case VerdictHandle:
		return "Handle"
	case VerdictActivate:
		return "Activate"
	case VerdictRelease:
		return "Release"
	default:
		return "Unknown"
	}
}

// Decision 是一次裁决的完整结果（纯数据，无副作用，便于表驱动单测）。
type Decision struct {
	Verdict    Verdict
	CommitIdx  int    // VerdictActivate：顶码上屏候选索引，-1=不顶
	TriggerKey string // VerdictActivate：触发键标识
	ActivateID string // VerdictActivate：宿主实例标识（special 引导键多实例的 id；其余空）
	Residual   string // VerdictRelease：放弃宿主时残留待重判的 buffer
}

// 便捷构造（包内使用，避免到处写结构体字面量）。
func decPass() Decision   { return Decision{Verdict: VerdictPass} }
func decHandle() Decision { return Decision{Verdict: VerdictHandle} }

func decActivate(triggerKey string, commitIdx int) Decision {
	return Decision{Verdict: VerdictActivate, TriggerKey: triggerKey, CommitIdx: commitIdx}
}

// decActivateID 带实例 id 的激活裁决（special 引导键：id 区分多个码表实例）。
func decActivateID(triggerKey, id string, commitIdx int) Decision {
	return Decision{Verdict: VerdictActivate, TriggerKey: triggerKey, ActivateID: id, CommitIdx: commitIdx}
}

func decRelease(residual string) Decision {
	return Decision{Verdict: VerdictRelease, Residual: residual}
}

// Capability 是需要对称挂卸的引擎资源位掩码。宿主切换时由决策器统一 diff，
// 避免散落各处的 ActivateTempPinyin/DeactivateTempPinyin 调用不对称。
type Capability uint64

const (
	// CapPinyinLayer 拼音词库层挂载（会污染五笔查询，必须对称卸载）。
	CapPinyinLayer Capability = 1 << iota
	// CapEnglishDict 英文词库加载。
	CapEnglishDict
	// 预留：CapEmoji / CapUrl / CapTranslate / CapCloudDict ...
)

// CompositionPhase 把 composition 边界与宿主切换解耦为正交事件。
// 现有焦点/composition 兼容补丁挂在 Cold/Commit/End 上；Hot 是新增的「窗口不变」路径。
type CompositionPhase int

const (
	// CompCold 无 composition → 有：StartComposition + armPendingFirstShow。
	CompCold CompositionPhase = iota
	// CompHot 有 → 有，仅宿主切换：原地换内容，不重启 composition、不重 arm。
	CompHot
	// CompCommit 有 → 上屏后开新 composition：HasNewComposition + resetCompositionAnchorAfterCommit。
	CompCommit
	// CompEnd 有 → 无：clearState + hideUI。
	CompEnd
)

func (p CompositionPhase) String() string {
	switch p {
	case CompCold:
		return "Cold"
	case CompHot:
		return "Hot"
	case CompCommit:
		return "Commit"
	case CompEnd:
		return "End"
	default:
		return "Unknown"
	}
}

// ProviderID 是候选来源标识（第二阶段融合用，此处预留）。
type ProviderID string
