package theme

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestDefaultThemeViewsParse 验证真实 default/theme.yaml 的 views 块能解析为 Theme.Views，
// 且关键颜色 token 就位（兜底运行时加载，避免 YAML 语法/结构错导致主题加载失败）。
func TestDefaultThemeViewsParse(t *testing.T) {
	data, err := os.ReadFile("../../themes/default/theme.yaml")
	if err != nil {
		t.Skip("default theme.yaml 不可读: " + err.Error())
	}
	var th Theme
	if err := yaml.Unmarshal(data, &th); err != nil {
		t.Fatalf("default theme.yaml 解析失败: %v", err)
	}
	if th.Views == nil {
		t.Fatal("default theme.yaml 应含 views 块")
	}
	if th.Views.Window.Background.Color != "${background}" {
		t.Errorf("window background token, got %q", th.Views.Window.Background.Color)
	}
	if th.Views.Item.Selected == nil || th.Views.Item.Selected.Background.Color != "${selected_bg}" {
		t.Error("item selected bg token 缺失")
	}
	if th.Views.AccentBar.Background.Color != "${accent}" {
		t.Errorf("accent_bar token, got %q", th.Views.AccentBar.Background.Color)
	}
	if th.Views.Index.Color != "${index_text}" {
		t.Errorf("index text token, got %q", th.Views.Index.Color)
	}
}

// TestMsimeThemeViewsParse 验证 msime/theme.yaml 的 views 块解析 + 关键字段（item radius 2 / 颜色 token）。
func TestMsimeThemeViewsParse(t *testing.T) {
	data, err := os.ReadFile("../../themes/msime/theme.yaml")
	if err != nil {
		t.Skip("msime theme.yaml 不可读: " + err.Error())
	}
	var th Theme
	if err := yaml.Unmarshal(data, &th); err != nil {
		t.Fatalf("msime theme.yaml 解析失败: %v", err)
	}
	if th.Views == nil {
		t.Fatal("msime theme.yaml 应含 views 块")
	}
	if th.Views.Item.Border.Radius == nil || *th.Views.Item.Border.Radius != 2 {
		t.Errorf("msime item radius 应为 2, got %v", th.Views.Item.Border.Radius)
	}
	if th.Views.Window.Background.Color != "${background}" {
		t.Errorf("window background token, got %q", th.Views.Window.Background.Color)
	}
	if th.Views.Index.Color != "${index_text}" {
		t.Errorf("index text token, got %q", th.Views.Index.Color)
	}
}

// TestViews_YAMLUnmarshal 验证 views 块 YAML 解析为 *int 显式语义（未写=nil）。
func TestViews_YAMLUnmarshal(t *testing.T) {
	data := `
window:
  padding: {top: 6, right: 8, bottom: 6, left: 8}
  border: {radius: 8}
item:
  padding: {left: 8, right: 10}
  border: {radius: 4}
text:
  margin: {left: 4}
comment:
  margin: {left: 6}
`
	var v Views
	if err := yaml.Unmarshal([]byte(data), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v.Window.Padding.Top == nil || *v.Window.Padding.Top != 6 {
		t.Errorf("window padding top: %v", v.Window.Padding.Top)
	}
	if v.Window.Border.Radius == nil || *v.Window.Border.Radius != 8 {
		t.Errorf("window border radius: %v", v.Window.Border.Radius)
	}
	if v.Item.Padding.Right == nil || *v.Item.Padding.Right != 10 {
		t.Errorf("item padding right: %v", v.Item.Padding.Right)
	}
	if v.Text.Margin.Left == nil || *v.Text.Margin.Left != 4 {
		t.Errorf("text margin left: %v", v.Text.Margin.Left)
	}
	// 未写字段应为 nil（显式语义）
	if v.Item.Margin.Left != nil {
		t.Errorf("item margin left 未写应为 nil, got %v", *v.Item.Margin.Left)
	}
	if v.Window.Padding.Right == nil || *v.Window.Padding.Right != 8 {
		t.Errorf("window padding right: %v", v.Window.Padding.Right)
	}
}
