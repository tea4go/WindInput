// pipeline_provider_rare_char.go — 生僻字候选 Provider（第二阶段融合，快捷模式可选源）。
//
// 生僻字本质是「一个特殊模式的编码表」：复用 specialModeRegistry 加载/查询基础设施
// （inst.table.LookupPrefix），产出整字候选。Source=SourceCodetable（整字上屏、无分段，
// 与 specialMode 选词语义一致），供选中时按 Source 分派 commit。
//
// 词库层加载（specialModeRegistry.ensureLoaded）是磁盘 I/O 副作用，**不在 Query 内**
// （I3）：由宿主在启用本源时 eager 加载，Query 假定 inst.table 已就绪、未就绪返回 nil。
package coordinator

import (
	"strings"

	"github.com/huanfeng/wind_input/internal/candidate"
)

// rareCharProviderLimit 单次前缀查询上限。
const rareCharProviderLimit = 50

type rareCharProvider struct {
	c  *Coordinator
	id string // 引用的特殊模式实例 id（配置 alpha_providers.rare_char_id）
}

func (rareCharProvider) ID() ProviderID { return ProviderRareChar }
func (rareCharProvider) Rank() int      { return 50 }

func (p rareCharProvider) Query(buffer string) []candidate.Candidate {
	if p.c.specialModeReg == nil || p.id == "" || buffer == "" {
		return nil
	}
	inst := p.c.specialModeReg.get(p.id)
	if inst == nil || inst.table == nil {
		return nil // 未加载（eager 加载在启用时由宿主触发，非此处）
	}
	// 与 englishProvider 一致在 provider 层统一转小写（LookupPrefix 内部虽也转，此处防御其未来变更）
	cands := inst.table.LookupPrefix(strings.ToLower(buffer), rareCharProviderLimit)
	for i := range cands {
		cands[i].Source = candidate.SourceCodetable
	}
	return cands
}
