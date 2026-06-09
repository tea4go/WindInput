package ui

import (
	"image/color"
	"testing"
)

// TestShouldShowModeHintBand 覆盖「嵌入编码下刚进入模式」提示条的判定矩阵（决策层纯函数）。
func TestShouldShowModeHintBand(t *testing.T) {
	cases := []struct {
		name        string
		hidePreedit bool
		modeLabel   string
		input       string
		candCount   int
		want        bool
	}{
		// 情况 1：嵌入编码开 + 处于模式 + 无输入 + 无候选 → 显示提示条
		{"inline_entry_empty", true, "临时拼音", "", 0, true},
		// 情况 2：嵌入编码开 + 已输入（input 非空）→ 不显示（候选窗承载反馈）
		{"inline_typed_with_input", true, "临时拼音", "`pin", 3, false},
		// 情况 3：嵌入编码开 + buffer 非空但无候选（无效编码）→ 保持原样，不显示
		{"inline_typed_no_cand", true, "临时拼音", "`zzz", 0, false},
		// 情况 4：嵌入编码关 → 永远走正常预编辑栏分支，提示条判定为假
		{"top_preedit_off", false, "临时拼音", "", 0, false},
		// 普通模式（无 ModeLabel）→ 不显示
		{"normal_no_mode", true, "", "", 0, false},
		// 防御：input 空但意外有候选 → 不显示
		{"empty_input_with_cand", true, "临时拼音", "", 2, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldShowModeHintBand(tc.hidePreedit, tc.modeLabel, tc.input, tc.candCount)
			if got != tc.want {
				t.Errorf("shouldShowModeHintBand(%v,%q,%q,%d) = %v, want %v",
					tc.hidePreedit, tc.modeLabel, tc.input, tc.candCount, got, tc.want)
			}
		})
	}
}

// treeHasText 在 View 树中递归查找是否存在 Text==target 的节点。
func treeHasText(v *View, target string) bool {
	if v == nil {
		return false
	}
	if v.Text == target {
		return true
	}
	for _, c := range v.Children {
		if treeHasText(c, target) {
			return true
		}
	}
	return false
}

// TestViewEngine_ModeHintBand_Render 验证提示条在「嵌入编码 + 空进入」时出现，
// 在「已输入」「嵌入编码关」等情况下不破坏既有行为（执行层端到端）。
func TestViewEngine_ModeHintBand_Render(t *testing.T) {
	newR := func(hidePreedit bool) *Renderer {
		cfg := parityConfig()
		cfg.ModeLabel = "临时拼音"
		cfg.ModeAccentColor = color.RGBA{0, 150, 136, 255}
		cfg.HidePreedit = hidePreedit
		r := NewRenderer(cfg)
		applyParityThemePath(r)
		return r
	}

	r := newR(true)
	if r.TextDrawer() == nil {
		t.Skip("无可用文本后端")
	}

	// 情况 1：嵌入编码开、无输入、无候选 → 提示条（含徽标）应出现
	t1 := r.buildHorizontalCandidateTree(nil, "", 0, 1, 1, 0, -1, "")
	if !treeHasText(t1.root, "临时拼音") {
		t.Errorf("情况1：嵌入编码空进入应显示模式徽标提示条，实际缺失")
	}

	// 情况 3：嵌入编码开、buffer 非空但无候选 → 不应出现提示条（保持原样）
	t3 := r.buildHorizontalCandidateTree(nil, "`zzz", 4, 1, 1, 0, -1, "")
	if treeHasText(t3.root, "临时拼音") {
		t.Errorf("情况3：嵌入编码下已输入无效编码不应弹出徽标提示条")
	}

	// 情况 4：嵌入编码关、无候选 → 走正常预编辑栏，徽标随预编辑栏正常显示（不受本次改动影响）
	rOff := newR(false)
	t4 := rOff.buildHorizontalCandidateTree(nil, "`", 1, 1, 1, 0, -1, "")
	if !treeHasText(t4.root, "临时拼音") {
		t.Errorf("情况4：嵌入编码关闭时预编辑栏徽标显示不应被破坏")
	}

	// 竖排对称：嵌入编码空进入也应显示提示条
	rv := newR(true)
	tv := rv.buildVerticalCandidateTree(nil, "", 0, 1, 1, 0, -1, "")
	if !treeHasText(tv.root, "临时拼音") {
		t.Errorf("竖排：嵌入编码空进入应显示模式徽标提示条，实际缺失")
	}
}
