package ui

import (
	"testing"

	"github.com/huanfeng/wind_input/pkg/theme"
)

// horizontalListGap 走真实 theme 路径（ResolveCandidateViews）构建横排树，返回候选列表行的 Gap
// （scale=1，等于 boxGap 逻辑像素）。用小 item padding(2) 使 boxGap=max(spacing-4,0)>0，可区分 12 vs 16。
func horizontalListGap(t *testing.T, indexStyle string) int {
	t.Helper()
	cfg := parityConfig()
	cfg.IndexStyle = indexStyle
	r := NewRenderer(cfg)
	if r.TextDrawer() == nil {
		t.Skip("无可用文本后端")
	}
	views := themePathViews(8, 2) // item padding=2
	r.resolvedV25 = &theme.ResolvedV25{Palette: themePathPalette()}
	r.themeViews = &views
	r.refreshResolvedViews()
	cands := []Candidate{{Text: "中", Index: 1}, {Text: "文", Index: 2}}
	tree := r.buildHorizontalCandidateTree(cands, "", 0, 1, 1, 0, -1, "")
	list := tree.root.Children[len(tree.root.Children)-1] // 无 preedit 时列表行=最后一个 band
	return list.Gap
}

// TestBuildHorizontal_TextIndexSpacing 验证 build 对文本序号施加 +4（圆点 boxGap 8 vs 文本 12）。
// P6 阶段2c 把间距 12/16 的 isTextIndex 选择从 resolve 层下沉到 build；本测试守护该下沉。
func TestBuildHorizontal_TextIndexSpacing(t *testing.T) {
	circleGap := horizontalListGap(t, "circle") // spacing 12 → boxGap max(12-4,0)=8
	textGap := horizontalListGap(t, "text")     // spacing 16 → boxGap max(16-4,0)=12
	if circleGap != 8 {
		t.Errorf("圆点序号 boxGap 应为 8（spacing 12），got %d", circleGap)
	}
	if textGap != 12 {
		t.Errorf("文本序号 boxGap 应为 12（spacing 16，+4 已下沉 build），got %d", textGap)
	}
}
