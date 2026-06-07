package ui

import (
	"testing"

	"github.com/huanfeng/wind_input/pkg/theme"
)

// TestSetTheme_PagerFromBehavior 验证 SetTheme 的 page 策略默认改读 rv.Behavior（P6 阶段2d）。
// 用户 PagerBarDisplay/PageNumberDisplay 覆盖在 manager.applyPagerOverride 注入，不在 renderer 层。
func TestSetTheme_PagerFromBehavior(t *testing.T) {
	cases := []struct {
		name            string
		behavior        theme.ResolvedBehavior
		wantAlwaysShow  bool
		wantShowPageNum bool
	}{
		{"defaultBehavior 零回归", theme.ResolvedBehavior{AlwaysShowPager: false, ShowPageNumber: true}, false, true},
		{"主题覆盖 always+隐藏页码", theme.ResolvedBehavior{AlwaysShowPager: true, ShowPageNumber: false}, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := NewRenderer(parityConfig())
			rv := &theme.ResolvedV3{Palette: themePathPalette(), Behavior: c.behavior}
			r.SetTheme(rv)
			if r.config.AlwaysShowPager != c.wantAlwaysShow {
				t.Errorf("AlwaysShowPager=%v, 期望 %v", r.config.AlwaysShowPager, c.wantAlwaysShow)
			}
			if r.config.ShowPageNumber != c.wantShowPageNum {
				t.Errorf("ShowPageNumber=%v, 期望 %v", r.config.ShowPageNumber, c.wantShowPageNum)
			}
		})
	}
}
