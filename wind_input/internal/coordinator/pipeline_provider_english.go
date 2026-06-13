// pipeline_provider_english.go — 英文词库候选 Provider（第二阶段融合，快捷模式可选源）。
//
// 复用「临时英文带候选」的英文词库查询（engineMgr.SearchEnglish），按字母前缀产出英文词
// 候选。Source=SourceEnglish（整词上屏），供选中时按 Source 分派 commit。
//
// 大小写适配（按输入大小写形态调整候选首字母/全大写）是显示层精修，留待接入切片处理；
// 本 provider 只做纯查询。英文词库加载（EnsureEnglishLoaded）是副作用，不在 Query 内（I3）。
package coordinator

import (
	"strings"

	"github.com/huanfeng/wind_input/internal/candidate"
)

// englishProviderLimit 单次前缀查询上限。
const englishProviderLimit = 50

type englishProvider struct{ c *Coordinator }

func (englishProvider) ID() ProviderID { return ProviderEnglish }
func (englishProvider) Rank() int      { return 60 }

func (p englishProvider) Query(buffer string) []candidate.Candidate {
	if p.c.engineMgr == nil || buffer == "" {
		return nil
	}
	cands := p.c.engineMgr.SearchEnglish(strings.ToLower(buffer), englishProviderLimit)
	for i := range cands {
		cands[i].Source = candidate.SourceEnglish
	}
	return cands
}
