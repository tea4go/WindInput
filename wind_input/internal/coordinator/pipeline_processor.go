// pipeline_processor.go — 宿主（Processor）接口。
package coordinator

import (
	"github.com/huanfeng/wind_input/internal/bridge"
	"github.com/huanfeng/wind_input/pkg/config"
)

// Processor 是「宿主」抽象：任一时刻单一活跃，持有模式状态（buffer/preedit），
// 裁决宿主迁移，并贡献自己「特有」的按键处理单元。
//
// 「按键怎么处理」下沉到 KeyHandler 链（见 pipeline_keyhandler.go）：宿主通过
// KeyHandlers() 只贡献特有处理单元，通用候选窗导航交共享 handler。
type Processor interface {
	Name() string

	// Judge 宿主迁移裁决：纯函数，只读 DecisionCtx，判断本按键是否触发切到/切离本宿主。
	// 约束（I2）：极轻量——只做长度检查 / 前缀字符比对，禁止锁竞争、IO、复杂正则。
	Judge(ctx *DecisionCtx, key string, data *bridge.KeyEventData) Decision

	// Activate 成为宿主。residual 为上一宿主 Release 交接的 buffer（可空）。
	// 必须原子（I10）：失败返回 ok=false 时不得遗留任何副作用。
	Activate(triggerKey, residual string) (prefix string, ok bool)

	// Release 卸下宿主身份。引擎资源（Capabilities）由决策器统一 diff（I3），此处不自行卸。
	Release()

	// BufferText 本宿主的活跃 buffer（engine_default=inputBuffer / temp_pinyin=tempPinyinBuffer …）。
	// 供 DecisionCtx 按 host 路由 buffer 访问器，避免固定读 inputBuffer 在别的模式活跃时失真
	// （第 0b 影子实测发现，见 docs/design 第八节）。
	BufferText() string

	// KeyHandlers 本宿主贡献的「特有」按键处理单元。决策器把它们与共享导航 handler 组装成链。
	KeyHandlers() []KeyHandler

	// Capabilities 需对称挂卸的引擎资源（拼音层/英文词库/未来 emoji/url…）。
	Capabilities() Capability

	// UsesExtendedPerPage 是否使用扩展档每页候选数。
	UsesExtendedPerPage() bool

	// PreferredLayout 期望的候选布局（""=不强制）。决策器据此做布局去抖。
	PreferredLayout() config.CandidateLayout

	// AcceptedProviders 接纳哪些 Provider 融合（空=独占）。第一阶段恒空（I9）。
	AcceptedProviders() []ProviderID
}
