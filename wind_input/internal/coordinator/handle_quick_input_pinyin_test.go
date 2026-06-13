package coordinator

import "testing"

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
