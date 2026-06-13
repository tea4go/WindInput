package coordinator

import (
	"testing"

	"github.com/huanfeng/wind_input/pkg/config"
)

// TestQuickInputAlphaExtraProviders 验证配置开关驱动的「拼音以外」融合源选取：
// 各源独立开关，生僻字额外要求有效 id + registry 就绪；顺序生僻字在英文前。
func TestQuickInputAlphaExtraProviders(t *testing.T) {
	mk := func(rareChar bool, id string, english bool, reg bool) *Coordinator {
		c := &Coordinator{config: &config.Config{}}
		c.config.Features.QuickInput.AlphaProviders.RareChar = rareChar
		c.config.Features.QuickInput.AlphaProviders.RareCharID = id
		c.config.Features.QuickInput.AlphaProviders.English = english
		if reg {
			c.specialModeReg = &specialModeRegistry{}
		}
		return c
	}
	ids := func(ps []CandidateProvider) []ProviderID {
		out := make([]ProviderID, len(ps))
		for i, p := range ps {
			out[i] = p.ID()
		}
		return out
	}

	cases := []struct {
		name         string
		rareChar     bool
		id           string
		english, reg bool
		want         []ProviderID
	}{
		{"全关", false, "", false, false, nil},
		{"仅英文", false, "", true, false, []ProviderID{ProviderEnglish}},
		{"仅生僻字", true, "rare", false, true, []ProviderID{ProviderRareChar}},
		{"生僻字缺id不启用", true, "", false, true, nil},
		{"生僻字缺registry不启用", true, "rare", false, false, nil},
		{"生僻字+英文有序", true, "rare", true, true, []ProviderID{ProviderRareChar, ProviderEnglish}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ids(mk(tc.rareChar, tc.id, tc.english, tc.reg).quickInputAlphaExtraProviders())
			if len(got) != len(tc.want) {
				t.Fatalf("%s: got %v, want %v", tc.name, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
					break
				}
			}
		})
	}
}

// TestQuickInputPinyinActive 锁定「拼音上下文由 buffer 内容派生」这一取代 quickInputPinyinMode
// 布尔的核心不变量：quick_input 激活 且 buffer 以小写字母打头 ⟺ 拼音上下文。
func TestQuickInputPinyinActive(t *testing.T) {
	cases := []struct {
		name       string
		quickInput bool
		buffer     string
		want       bool
	}{
		{"未进快捷输入", false, "ni", false},
		{"快捷输入空 buffer", true, "", false},
		{"拼音上下文-字母打头", true, "ni", true},
		{"拼音上下文-含分隔符", true, "xi'an", true},
		{"分段上屏后残留拼音", true, "hao", true},
		// 首字节大写不视为拼音上下文——engageQuickInputPinyin 调用方保证首字母已小写化，
		// 此用例锁定该隐含不变量（万一某入口忘记小写，active 应返回 false 而非误判）。
		{"大写字母打头", true, "Ni", false},
		{"小写打头大写后续", true, "nI", true},
		{"结构化-纯数字", true, "123", false},
		{"结构化-运算符开头", true, "(1+2)", false},
		{"结构化-日期", true, "2024.1.1", false},
	}
	for _, tc := range cases {
		c := &Coordinator{}
		c.quickInputMode = tc.quickInput
		c.quickInputBuffer = tc.buffer
		if got := c.quickInputPinyinActive(); got != tc.want {
			t.Errorf("%s: quickInputPinyinActive()=%v, want %v", tc.name, got, tc.want)
		}
	}
}
