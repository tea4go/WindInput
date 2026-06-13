// pipeline_provider.go — 候选 Provider 融合层接口（第二阶段，Batch 4）。
//
// Provider 是「候选来源」抽象：纯查询、无 UI/引擎副作用，给定宿主 buffer 产出候选条目。
// 与 Processor（宿主，单一活跃、持有按键语义）正交——多个 Provider 可同时向同一宿主供
// 候选，经 mergeProviderCandidates 按 Rank 分段合并（设计文档九.2：分段而非全局排序）。
//
// 选中后的 commit 归属靠候选自带的 candidate.Source 区分（拼音候选 Source=SourcePinyin
// 且 ConsumedLength>0 → 分段上屏；date/calc/number 候选 Source 空 → 整条上屏）——故无需
// 在 Candidate 上再加 ProviderID 字段：设计文档 9.1 的「候选血缘」由既有 Source/
// ConsumedLength 承载，不另造平行候选类型。
package coordinator

import "github.com/huanfeng/wind_input/internal/candidate"

// ProviderID 是候选来源标识，用于合并段位排序与准入白名单（Processor.AcceptedProviders）。
type ProviderID string

// Provider 标识。Rank 决定合并段位顺序（见各 provider 的 Rank()）。
const (
	ProviderDate   ProviderID = "date"   // 日期/年月（年月日三段 + 年月两段）
	ProviderCalc   ProviderID = "calc"   // 计算表达式
	ProviderNumber ProviderID = "number" // 数字/小数
	ProviderPinyin ProviderID = "pinyin" // 临时拼音候选
)

// CandidateProvider 是候选来源抽象。Query 是纯查询：给定 buffer 产出候选，
// 不得有 UI / 引擎挂卸副作用（引擎词库层挂卸由宿主按 Capability 统一管，见 I3）。
type CandidateProvider interface {
	// ID 来源标识。
	ID() ProviderID

	// Rank 合并段位：小者在前。结构化候选（日期/计算/数字）占固定低段位，
	// 同质语言候选（拼音）在更高段位（设计文档 9.2）。
	Rank() int

	// Query 给定宿主 buffer 产出候选；不匹配返回 nil/空。
	// 实现须保持纯查询：候选的 Source/ConsumedLength 由实现正确填充，供选中时回 dispatch。
	Query(buffer string) []candidate.Candidate
}
