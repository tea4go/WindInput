// pipeline_provider_pinyin.go — 临时拼音候选 Provider（第二阶段，Batch 4）。
//
// 把快捷输入下的临时拼音候选降为 Provider（Rank 40，语言类候选段位高于结构化的
// date/calc/number）。Query 委托引擎 ConvertWithPinyin——候选自带 Source=SourcePinyin
// 且 ConsumedLength（拼音切分长度），供选中时走分段上屏 commit。
//
// PreeditDisplay 注意：拼音引擎除候选外还返回分段显示串（如 "zhang guo"），是宿主渲染
// preedit 必需的，但 CandidateProvider.Query 接口只产候选。故另开具体方法 query() 同时
// 返回候选 + preedit；宿主融合时持有具体 pinyinProvider 即可调 query() 取 preedit，
// Query() 仅暴露候选以满足接口契约。
//
// 引擎词库层挂卸（码表引擎下 ActivateTempPinyin/DeactivateTempPinyin）是副作用，由宿主
// 按 Capability 统一管（I3），不在 Query 内——Query 假定拼音层已就绪。
package coordinator

import "github.com/huanfeng/wind_input/internal/candidate"

// pinyinProviderMaxCandidates 单次拉取上限，与旧 updatePinyinModeCandidates 一致。
const pinyinProviderMaxCandidates = 100

type pinyinProvider struct{ c *Coordinator }

func (pinyinProvider) ID() ProviderID { return ProviderPinyin }
func (pinyinProvider) Rank() int      { return 40 }

func (p pinyinProvider) Query(buffer string) []candidate.Candidate {
	cands, _ := p.query(buffer)
	return cands
}

// query 返回候选 + 拼音分段显示串（PreeditDisplay）。引擎缺失 / 空 buffer / 空结果返回 nil,""。
func (p pinyinProvider) query(buffer string) ([]candidate.Candidate, string) {
	if p.c.engineMgr == nil || buffer == "" {
		return nil, ""
	}
	// ConvertWithPinyin 契约保证返回非 nil（各分支均 return &ConvertResult{...}），与旧
	// updatePinyinModeCandidates 一致，不另做 nil 防御。
	result := p.c.engineMgr.ConvertWithPinyin(buffer, pinyinProviderMaxCandidates)
	return result.Candidates, result.PreeditDisplay
}
