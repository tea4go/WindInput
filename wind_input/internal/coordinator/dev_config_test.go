package coordinator

import (
	"testing"

	toml "github.com/pelletier/go-toml/v2"
)

// TestDevConfigDeciderDefault 锁定「决策器默认接管 + 显式回退」语义（P4 翻默认的依据）：
// loadDevConfig 预填 DeciderEnabled=true，wind_dev.toml 缺该键时保持 true，显式 false 时回退。
// 这里直接验证 toml.Unmarshal 入预填 struct 的覆盖语义（loadDevConfig 的核心）。
func TestDevConfigDeciderDefault(t *testing.T) {
	cases := []struct {
		name string
		toml string
		want bool
	}{
		{"空内容→默认开", "", true},
		{"只有 shadow、缺 decider_enabled→保持默认开", "decider_shadow = true\n", true},
		{"显式 true", "decider_enabled = true\n", true},
		{"显式 false→回退旧逻辑", "decider_enabled = false\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dc := devConfig{DeciderEnabled: true} // 同 loadDevConfig 的预填默认
			if err := toml.Unmarshal([]byte(tc.toml), &dc); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if dc.DeciderEnabled != tc.want {
				t.Errorf("%s: DeciderEnabled=%v, want %v", tc.name, dc.DeciderEnabled, tc.want)
			}
		})
	}
}
