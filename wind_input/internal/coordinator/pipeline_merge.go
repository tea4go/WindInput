// pipeline_merge.go — 候选 Provider 分段合并器（第二阶段，Batch 4）。
package coordinator

import (
	"sort"

	"github.com/huanfeng/wind_input/internal/candidate"
)

// mergeProviderCandidates 按 Rank 升序依次查询各 provider，将结果**分段拼接**
// （段位顺序 = 隐式 Rank，设计文档 9.2：不做全局 Weight 拉平），再按 Text 去重。
//
// 不分配 Index/IndexLabel——序号风格随当前活跃 provider 而定（quick_input 基础用
// a/b/c，拼音用数字 1-9/0），由宿主 showUI 负责，故合并只产出裸候选列表。
//
// 去重保留首次出现者，与旧 updateQuickInputCandidates 的 dedup 语义一致（date→
// yearMonth→calc→number 先到先留）。buffer 为空时直接返回 nil（空 buffer 的历史
// 重复候选由宿主单独处理，不经 provider）。
//
// candidate.Candidate 是值类型，append 为值拷贝——合并结果与 provider 内部存储独立，
// 调用方修改返回 slice 不影响 provider；血缘字段（Source/ConsumedLength）随值拷贝保真。
func mergeProviderCandidates(buffer string, providers []CandidateProvider) []candidate.Candidate {
	if buffer == "" || len(providers) == 0 {
		return nil
	}

	ordered := make([]CandidateProvider, len(providers))
	copy(ordered, providers)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Rank() < ordered[j].Rank()
	})

	var merged []candidate.Candidate
	seen := make(map[string]struct{})
	for _, p := range ordered {
		for _, cand := range p.Query(buffer) {
			if _, ok := seen[cand.Text]; ok {
				continue
			}
			seen[cand.Text] = struct{}{}
			merged = append(merged, cand)
		}
	}
	return merged
}
