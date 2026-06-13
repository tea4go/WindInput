// dev_config.go — 独立开发/调试配置。
//
// 与主配置（pkg/config）完全隔离：不进 const-gen、不进版本迁移桥接、不暴露前端 UI。
// 用途是开发期临时开关（如输入处理器流水线影子运行），避免污染用户主配置文件与配置流程。
// 文件位置：配置目录/wind_dev.toml；文件不存在或解析失败时全部回落默认（全关闭）。
// 启动时（NewCoordinator）加载一次；改动后重启服务生效（重启/重连均会重新加载，比环境变量可靠）。
package coordinator

import (
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/huanfeng/wind_input/pkg/config"
)

// devConfigFileName 开发配置文件名（置于配置目录下）。
const devConfigFileName = "wind_dev.toml"

// devConfig 开发/调试开关集合。新增开关时只在此处加字段 + toml tag，无需任何代码生成。
type devConfig struct {
	// DeciderShadow 输入处理器流水线第 0b 影子运行：只读地并行运行新决策器裁决并记 DEBUG
	// 日志，零行为影响。用于观测新旧裁决一致性。详见 docs/design/input-processor-pipeline.md。
	DeciderShadow bool `toml:"decider_shadow"`

	// DeciderEnabled 第 1 批 1c：决策器真正接管 z 键混合回退判定（执行复用现有
	// enterTempPinyinFromZBuffer = CompHot 原地切换）。默认 false 走旧 zHybridFallback。
	// 与 DeciderShadow 独立：可单开 shadow 观测、或开 enabled 真实接管对比。
	DeciderEnabled bool `toml:"decider_enabled"`
}

// loadDevConfig 从配置目录读取 wind_dev.toml；不存在/解析失败时返回零值（全关）。
func loadDevConfig() devConfig {
	var dc devConfig
	dir, err := config.GetConfigDir()
	if err != nil {
		return dc
	}
	data, err := os.ReadFile(filepath.Join(dir, devConfigFileName))
	if err != nil {
		return dc // 文件不存在（最常见）→ 全默认
	}
	_ = toml.Unmarshal(data, &dc) // 解析失败 → 保持零值，不阻断启动
	return dc
}
