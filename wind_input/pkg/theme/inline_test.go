package theme

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestInlineTheme_FromExternal(t *testing.T) {
	tmp, cleanup := setupTestThemes(t)
	defer cleanup()
	m := makeTestManager(tmp)

	src := &Theme{
		Meta:    ThemeMeta{Name: "Src"},
		Layout:  "test-layout",
		Palette: "test-palette",
	}
	inlined, err := m.InlineTheme(src)
	if err != nil {
		t.Fatalf("InlineTheme: %v", err)
	}
	if _, ok := inlined.Layout.(map[string]any); !ok {
		t.Errorf("inlined Layout should be map, got %T", inlined.Layout)
	}
	if _, ok := inlined.Palette.(map[string]any); !ok {
		t.Errorf("inlined Palette should be map, got %T", inlined.Palette)
	}
	if inlined.Overrides != nil {
		t.Errorf("Overrides should be cleared after inline")
	}

	// 内联后 Resolve 应与原外链结果一致
	rExt, _ := m.ResolveV25(src, false, tmp)
	rIn, err := m.ResolveV25(inlined, false, tmp)
	if err != nil {
		t.Fatalf("inlined Resolve: %v", err)
	}
	if !reflect.DeepEqual(rExt.Layout, rIn.Layout) {
		t.Errorf("inlined layout 应与外链等价")
	}
	if ColorToHex(rExt.Palette.Bg) != ColorToHex(rIn.Palette.Bg) {
		t.Errorf("inlined palette.bg 应与外链等价")
	}
}

func TestInlineTheme_AppliesOverrides(t *testing.T) {
	tmp, cleanup := setupTestThemes(t)
	defer cleanup()
	m := makeTestManager(tmp)

	src := &Theme{
		Layout:  "test-layout",
		Palette: "test-palette",
		Overrides: &Overrides{
			Layout: map[string]any{
				"candidate_window": map[string]any{
					"band_gap": 88,
				},
			},
		},
	}
	inlined, err := m.InlineTheme(src)
	if err != nil {
		t.Fatal(err)
	}
	if inlined.Overrides != nil {
		t.Errorf("overrides 应被合并清空")
	}
	r, err := m.ResolveV25(inlined, false, tmp)
	if err != nil {
		t.Fatal(err)
	}
	if r.Layout.CandidateWindow.BandGap != 88 {
		t.Errorf("overrides 未合并到内联，band_gap=%d", r.Layout.CandidateWindow.BandGap)
	}
}

func TestExternalizeTheme_RoundTrip(t *testing.T) {
	// 1. 起始：内联 theme
	var lm, pm map[string]any
	_ = yaml.Unmarshal([]byte(sampleLayoutYAML), &lm)
	_ = yaml.Unmarshal([]byte(samplePaletteYAML), &pm)
	inline := &Theme{
		Meta:    ThemeMeta{Name: "RoundTrip"},
		Layout:  lm,
		Palette: pm,
	}

	// 2. Externalize 写文件
	out, err := os.MkdirTemp("", "externalize")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(out)

	layoutID, paletteID, err := ExternalizeTheme(inline, out, "roundtrip")
	if err != nil {
		t.Fatalf("ExternalizeTheme: %v", err)
	}
	if layoutID == "" || paletteID == "" {
		t.Errorf("ID 应非空: layout=%q palette=%q", layoutID, paletteID)
	}

	// 3. 重新加载外链形态
	themeFile := filepath.Join(out, "roundtrip", "theme.yaml")
	data, err := os.ReadFile(themeFile)
	if err != nil {
		t.Fatal(err)
	}
	reloaded := &Theme{}
	if err := yaml.Unmarshal(data, reloaded); err != nil {
		t.Fatal(err)
	}
	if id, _ := reloaded.Layout.(string); id != layoutID {
		t.Errorf("reloaded layout id want %q, got %v", layoutID, reloaded.Layout)
	}
	if id, _ := reloaded.Palette.(string); id != paletteID {
		t.Errorf("reloaded palette id want %q, got %v", paletteID, reloaded.Palette)
	}

	// 4. Resolve 重载后的外链主题 + 原内联，结果应等价
	m := makeTestManager(out)
	rExt, err := m.ResolveV25(reloaded, false, out)
	if err != nil {
		t.Fatal(err)
	}
	rIn, err := m.ResolveV25(inline, false, out)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rExt.Layout, rIn.Layout) {
		t.Errorf("round-trip layout 不一致")
	}
	if ColorToHex(rExt.Palette.CandidateWindow.SelectedBg) != ColorToHex(rIn.Palette.CandidateWindow.SelectedBg) {
		t.Errorf("round-trip selected_bg 不一致")
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct{ in, fallback, want string }{
		{"clean", "fb", "clean"},
		{"with space", "fb", "with-space"},
		{"中文 mixed-name_1", "fb", "mixed-name_1"},
		{"", "fb", "fb"},
		{"中文", "fb", "fb"},
	}
	for _, tc := range tests {
		got := slugify(tc.in, tc.fallback)
		if got != tc.want {
			t.Errorf("slugify(%q,%q) = %q, want %q", tc.in, tc.fallback, got, tc.want)
		}
	}
}
