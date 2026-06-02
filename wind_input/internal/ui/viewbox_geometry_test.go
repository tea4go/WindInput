package ui

import (
	"fmt"
	"image/color"
	"reflect"
	"testing"

	"github.com/huanfeng/wind_input/pkg/config"
)

// colorHex 把 color.Color 转为 RGBA 十六进制；nil 返回 "-"。
func colorHex(c color.Color) string {
	if c == nil {
		return "-"
	}
	r, g, b, a := c.RGBA()
	return fmt.Sprintf("%02x%02x%02x%02x", uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8))
}

// flattenNodes 深度优先收集 View 树所有节点的「几何 + 颜色」指纹（P2 切片-1 零回归网）。
// 每节点记录 Rect + Background/Border/TextStyle 颜色；重构前后一致即视为几何+颜色对齐。
func flattenNodes(v *View) []string {
	if v == nil {
		return nil
	}
	r := v.Rect()
	out := []string{fmt.Sprintf("%d,%d,%d,%d|bg=%s|bd=%s|tx=%s",
		r.Min.X, r.Min.Y, r.Dx(), r.Dy(),
		colorHex(v.Background.Color), colorHex(v.Border.Color), colorHex(v.TextStyle.Color))}
	for _, c := range v.Children {
		out = append(out, flattenNodes(c)...)
	}
	return out
}

// geometryFingerprint 构建候选窗 View 树、布局，返回所有节点的几何+颜色指纹序列。
func geometryFingerprint(t *testing.T, layout config.CandidateLayout) []string {
	t.Helper()
	cfg := parityConfig()
	cfg.Layout = layout
	r := NewRenderer(cfg)
	if r.TextDrawer() == nil {
		t.Skip("无可用文本后端")
	}
	// 模拟 RenderCandidates 的合成桥填充（直接调 build 绕过了入口）。
	r.resolvedViews = r.buildResolvedViews()
	cands := []Candidate{
		{Text: "中文", Index: 1},
		{Text: "中", Index: 2, Comment: "zhōng"},
		{Text: "众", Index: 3},
		{Text: "种", Index: 4},
		{Text: "重", Index: 5},
	}
	var tree *candWindowTree
	if layout == config.LayoutHorizontal {
		tree = r.buildHorizontalCandidateTree(cands, "zhong", 5, 2, 3, 0, 1, "")
	} else {
		tree = r.buildVerticalCandidateTree(cands, "zhong", 5, 2, 3, 0, 1, "")
	}
	Layout(tree.root, 0, 0, r.textDrawer)
	return flattenNodes(tree.root)
}

// 几何+颜色基准（P2 切片-1 重捕，DPI scale=1）。后续重构 task 必须保持不变。
var (
	wantHGeometry = []string{"0,0,442,76|bg=ffffffff|bd=c2c6cbff|tx=-", "8,8,426,24|bg=f0f0f0ff|bd=-|tx=-", "16,11,45,18|bg=-|bd=-|tx=646464ff", "8,36,426,32|bg=-|bd=-|tx=-", "8,36,74,32|bg=d2e4ffff|bd=-|tx=-", "16,43,18,18|bg=4285f4ff|bd=-|tx=-", "16,43,18,18|bg=-|bd=-|tx=ffffffff", "38,43,36,18|bg=-|bd=-|tx=1f1f1fff", "82,36,99,32|bg=e6f0ffff|bd=-|tx=-", "90,43,18,18|bg=4285f4ff|bd=-|tx=-", "90,43,18,18|bg=-|bd=-|tx=ffffffff", "112,43,18,18|bg=-|bd=-|tx=1f1f1fff", "138,45,35,14|bg=-|bd=-|tx=969696ff", "181,36,56,32|bg=-|bd=-|tx=-", "189,43,18,18|bg=4285f4ff|bd=-|tx=-", "189,43,18,18|bg=-|bd=-|tx=ffffffff", "211,43,18,18|bg=-|bd=-|tx=1f1f1fff", "237,36,56,32|bg=-|bd=-|tx=-", "245,43,18,18|bg=4285f4ff|bd=-|tx=-", "245,43,18,18|bg=-|bd=-|tx=ffffffff", "267,43,18,18|bg=-|bd=-|tx=1f1f1fff", "293,36,56,32|bg=-|bd=-|tx=-", "301,43,18,18|bg=4285f4ff|bd=-|tx=-", "301,43,18,18|bg=-|bd=-|tx=ffffffff", "323,43,18,18|bg=-|bd=-|tx=1f1f1fff", "357,36,21,32|bg=-|bd=-|tx=-", "378,45,35,14|bg=-|bd=-|tx=646464ff", "413,36,21,32|bg=-|bd=-|tx=-"}
	wantVGeometry = []string{"0,0,117,246|bg=ffffffff|bd=c2c6cbff|tx=-", "8,8,101,30|bg=f0f0f0ff|bd=-|tx=-", "16,14,45,18|bg=-|bd=-|tx=646464ff", "8,42,101,160|bg=-|bd=-|tx=-", "8,42,101,32|bg=d2e4ffff|bd=-|tx=-", "11,47,22,22|bg=4285f4ff|bd=-|tx=-", "11,47,22,22|bg=-|bd=-|tx=ffffffff", "40,49,36,18|bg=-|bd=-|tx=1f1f1fff", "8,74,101,32|bg=e6f0ffff|bd=-|tx=-", "11,79,22,22|bg=4285f4ff|bd=-|tx=-", "11,79,22,22|bg=-|bd=-|tx=ffffffff", "40,81,18,18|bg=-|bd=-|tx=1f1f1fff", "66,83,35,14|bg=-|bd=-|tx=969696ff", "8,106,101,32|bg=-|bd=-|tx=-", "11,111,22,22|bg=4285f4ff|bd=-|tx=-", "11,111,22,22|bg=-|bd=-|tx=ffffffff", "40,113,18,18|bg=-|bd=-|tx=1f1f1fff", "8,138,101,32|bg=-|bd=-|tx=-", "11,143,22,22|bg=4285f4ff|bd=-|tx=-", "11,143,22,22|bg=-|bd=-|tx=ffffffff", "40,145,18,18|bg=-|bd=-|tx=1f1f1fff", "8,170,101,32|bg=-|bd=-|tx=-", "11,175,22,22|bg=4285f4ff|bd=-|tx=-", "11,175,22,22|bg=-|bd=-|tx=ffffffff", "40,177,18,18|bg=-|bd=-|tx=1f1f1fff", "20,206,77,32|bg=-|bd=-|tx=-", "20,206,21,32|bg=-|bd=-|tx=-", "41,215,35,14|bg=-|bd=-|tx=646464ff", "76,206,21,32|bg=-|bd=-|tx=-"}
)

// TestGeometryFingerprint_Horizontal 横排几何+颜色零回归：指纹须与基准逐项一致。
func TestGeometryFingerprint_Horizontal(t *testing.T) {
	got := geometryFingerprint(t, config.LayoutHorizontal)
	if !reflect.DeepEqual(got, wantHGeometry) {
		t.Errorf("横排几何+颜色漂移:\n got  (%d): %#v", len(got), got)
	}
}

// TestGeometryFingerprint_Vertical 竖排几何+颜色零回归：指纹须与基准逐项一致。
func TestGeometryFingerprint_Vertical(t *testing.T) {
	got := geometryFingerprint(t, config.LayoutVertical)
	if !reflect.DeepEqual(got, wantVGeometry) {
		t.Errorf("竖排几何+颜色漂移:\n got  (%d): %#v", len(got), got)
	}
}
