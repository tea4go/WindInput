package theme

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

// 完整 layout/palette yaml 文本 — 内联与外链测试共用
// P7-5：候选窗已不在 layout，fixture 改用其它窗口字段（toolbar/tooltip）验证 layout 合并。
const sampleLayoutYAML = `
meta: {name: "test-layout", version: "1.0"}
density: compact
scale: 1.0
toolbar:
  item_gap: 3
tooltip:
  max_width: 500
`

const samplePaletteYAML = `
meta: {name: "test-palette", version: "1.0"}
primary: "#4285F4"
derive: {enabled: true, algorithm: hsl-shift}
light:
  bg: "#FFFFFF"
  text: "#1E1E1E"
  candidate_window:
    background: "${bg}"
    text: "${text}"
    selected_bg: "${primary}"
dark:
  bg: "#2D2D2D"
  text: "#E0E0E0"
  candidate_window:
    background: "${bg}"
    text: "${text}"
    selected_bg: "${primary}"
`

// 创建一个临时主题目录结构
func setupTestThemes(t *testing.T) (themesDir string, cleanup func()) {
	t.Helper()
	tmp, err := os.MkdirTemp("", "themetest")
	if err != nil {
		t.Fatal(err)
	}
	cleanup = func() { os.RemoveAll(tmp) }

	mustWrite := func(p, content string) {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(filepath.Join(tmp, "_layouts", "test-layout.yaml"), sampleLayoutYAML)
	mustWrite(filepath.Join(tmp, "_palettes", "test-palette.yaml"), samplePaletteYAML)

	return tmp, cleanup
}

// 直接构造一个手工 Manager 指向临时 themesDir
func makeTestManager(themesDir string) *Manager {
	return &Manager{themeDirs: []string{themesDir}}
}

func TestResolveV25_External(t *testing.T) {
	tmp, cleanup := setupTestThemes(t)
	defer cleanup()
	m := makeTestManager(tmp)

	theme := &Theme{
		Meta:    ThemeMeta{Name: "External"},
		Layout:  "test-layout",
		Palette: "test-palette",
	}
	r, err := m.ResolveV25(theme, false, tmp)
	if err != nil {
		t.Fatalf("ResolveV25 external: %v", err)
	}
	if r.Layout.Toolbar.ItemGap != 3 {
		t.Errorf("toolbar.item_gap want 3, got %d", r.Layout.Toolbar.ItemGap)
	}
	if r.Layout.Tooltip.MaxWidth != 500 {
		t.Errorf("tooltip.max_width want 500, got %d", r.Layout.Tooltip.MaxWidth)
	}
	// density 基线填充
	if r.Layout.Toolbar.Padding.Top == 0 {
		t.Errorf("toolbar padding should be baseline-filled")
	}
	// palette 解析 + ${} 展开
	if ColorToHexRGB(r.Palette.CandidateWindow.SelectedBg) != "#4285F4" {
		t.Errorf("selected_bg should be primary, got %s", ColorToHexRGB(r.Palette.CandidateWindow.SelectedBg))
	}
	if ColorToHexRGB(r.Palette.CandidateWindow.Background) != "#FFFFFF" {
		t.Errorf("background should be #FFFFFF, got %s", ColorToHexRGB(r.Palette.CandidateWindow.Background))
	}
}

func TestResolveV25_Inline(t *testing.T) {
	tmp, cleanup := setupTestThemes(t)
	defer cleanup()
	m := makeTestManager(tmp)

	// 内联：把 layout/palette 内容作为对象直接挂在 Theme 上
	var layoutMap, paletteMap map[string]any
	if err := yaml.Unmarshal([]byte(sampleLayoutYAML), &layoutMap); err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal([]byte(samplePaletteYAML), &paletteMap); err != nil {
		t.Fatal(err)
	}
	theme := &Theme{
		Meta:    ThemeMeta{Name: "Inline"},
		Layout:  layoutMap,
		Palette: paletteMap,
	}
	r, err := m.ResolveV25(theme, false, tmp)
	if err != nil {
		t.Fatalf("ResolveV25 inline: %v", err)
	}
	if r.Layout.Toolbar.ItemGap != 3 {
		t.Errorf("inline toolbar.item_gap want 3, got %d", r.Layout.Toolbar.ItemGap)
	}
	if ColorToHexRGB(r.Palette.CandidateWindow.SelectedBg) != "#4285F4" {
		t.Errorf("inline selected_bg want #4285F4, got %s", ColorToHexRGB(r.Palette.CandidateWindow.SelectedBg))
	}
}

func TestResolveV25_ExternalAndInlineEquivalent(t *testing.T) {
	tmp, cleanup := setupTestThemes(t)
	defer cleanup()
	m := makeTestManager(tmp)

	external := &Theme{Layout: "test-layout", Palette: "test-palette"}
	rExt, err := m.ResolveV25(external, false, tmp)
	if err != nil {
		t.Fatal(err)
	}

	var lm, pm map[string]any
	_ = yaml.Unmarshal([]byte(sampleLayoutYAML), &lm)
	_ = yaml.Unmarshal([]byte(samplePaletteYAML), &pm)
	inline := &Theme{Layout: lm, Palette: pm}
	rIn, err := m.ResolveV25(inline, false, tmp)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(rExt.Layout, rIn.Layout) {
		t.Errorf("layout 不等价:\nExt:%+v\nIn :%+v", rExt.Layout, rIn.Layout)
	}
	if ColorToHex(rExt.Palette.Bg) != ColorToHex(rIn.Palette.Bg) {
		t.Errorf("palette.bg 不等价")
	}
	if ColorToHex(rExt.Palette.CandidateWindow.SelectedBg) != ColorToHex(rIn.Palette.CandidateWindow.SelectedBg) {
		t.Errorf("palette.candidate_window.selected_bg 不等价")
	}
}

func TestResolveV25_DarkMode(t *testing.T) {
	tmp, cleanup := setupTestThemes(t)
	defer cleanup()
	m := makeTestManager(tmp)

	theme := &Theme{Layout: "test-layout", Palette: "test-palette"}
	r, err := m.ResolveV25(theme, true, tmp)
	if err != nil {
		t.Fatal(err)
	}
	if ColorToHexRGB(r.Palette.Bg) != "#2D2D2D" {
		t.Errorf("dark bg want #2D2D2D, got %s", ColorToHexRGB(r.Palette.Bg))
	}
}

func TestResolveV25_Overrides(t *testing.T) {
	tmp, cleanup := setupTestThemes(t)
	defer cleanup()
	m := makeTestManager(tmp)

	theme := &Theme{
		Layout:  "test-layout",
		Palette: "test-palette",
		Overrides: &Overrides{
			Layout: map[string]any{
				"toolbar": map[string]any{
					"item_gap": 99,
				},
			},
			Palette: map[string]any{
				"light": map[string]any{
					"candidate_window": map[string]any{
						"selected_bg": "#FF0000",
					},
				},
			},
		},
	}
	r, err := m.ResolveV25(theme, false, tmp)
	if err != nil {
		t.Fatal(err)
	}
	if r.Layout.Toolbar.ItemGap != 99 {
		t.Errorf("overrides toolbar.item_gap want 99, got %d", r.Layout.Toolbar.ItemGap)
	}
	if ColorToHexRGB(r.Palette.CandidateWindow.SelectedBg) != "#FF0000" {
		t.Errorf("overrides selected_bg want #FF0000, got %s", ColorToHexRGB(r.Palette.CandidateWindow.SelectedBg))
	}
}

func TestResolveV25_UnknownLayoutID(t *testing.T) {
	tmp, cleanup := setupTestThemes(t)
	defer cleanup()
	m := makeTestManager(tmp)

	theme := &Theme{Layout: "nosuch", Palette: "test-palette"}
	if _, err := m.ResolveV25(theme, false, tmp); err == nil {
		t.Errorf("expected error for unknown layout id")
	}
}
