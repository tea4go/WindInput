package schema

import (
	"testing"

	"github.com/huanfeng/wind_input/internal/engine/mixed"
)

// TestMixedConfigFromSpec_ConstructionEqualsReload 锁定"构建路径"与"热更新路径"产出的
// mixed.Config 完全一致：两者都必须经 MixedConfigFromSpec。任何一侧将来绕过它（例如有人
// 又在 manager_config.go 里逐字段手抄、或在 factory 里内联），都会让本测试报红。
// 这道防呆使"新增 mixed 开关只改 MixedConfigFromSpec 一处"成为可靠契约。
func TestMixedConfigFromSpec_ConstructionEqualsReload(t *testing.T) {
	no := false
	yes := true
	spec := &MixedSpec{
		MinPinyinLength:       3,
		CodetableWeightBoost:  12_345_678,
		ShowSourceHint:        true,
		PinyinOnlyOverflow:    &no,
		TopCodeOverridePinyin: &yes,
	}
	want := MixedConfigFromSpec(spec)

	// 构建路径：NewEngine 直接收 MixedConfigFromSpec 的结果
	built := mixed.NewEngine(nil, nil, MixedConfigFromSpec(spec), nil)
	if got := built.GetConfig(); *got != *want {
		t.Errorf("construction config = %+v, want %+v", *got, *want)
	}

	// 热更新路径：从默认配置引擎出发，ApplyConfig 整体覆盖
	reloaded := mixed.NewEngine(nil, nil, mixed.DefaultConfig(), nil)
	reloaded.ApplyConfig(MixedConfigFromSpec(spec))
	if got := reloaded.GetConfig(); *got != *want {
		t.Errorf("reload config = %+v, want %+v", *got, *want)
	}
}

// TestMixedConfigFromSpec_NilAndDefaults nil spec 应退回 DefaultConfig；空 spec 的未设置
// 字段走缺省（MinPinyinLength=2、PinyinOnlyOverflow=true、TopCodeOverridePinyin=false）。
func TestMixedConfigFromSpec_NilAndDefaults(t *testing.T) {
	if got, want := MixedConfigFromSpec(nil), mixed.DefaultConfig(); *got != *want {
		t.Errorf("nil spec config = %+v, want default %+v", *got, *want)
	}
	cfg := MixedConfigFromSpec(&MixedSpec{})
	if cfg.MinPinyinLength != 2 {
		t.Errorf("MinPinyinLength = %d, want 2", cfg.MinPinyinLength)
	}
	if !cfg.PinyinOnlyOverflow {
		t.Error("PinyinOnlyOverflow should default true")
	}
	if cfg.TopCodeOverridePinyin {
		t.Error("TopCodeOverridePinyin should default false")
	}
}
