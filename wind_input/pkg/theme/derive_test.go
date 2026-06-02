package theme

import "testing"

func TestDeriveHSLShift_Light(t *testing.T) {
	d := derivePalette("#4285F4", "hsl-shift", false)
	if d.Bg != "#FFFFFF" {
		t.Errorf("light bg want #FFFFFF, got %s", d.Bg)
	}
	if d.Accent != "#4285F4" {
		t.Errorf("accent should equal primary, got %s", d.Accent)
	}
	if d.OnAccent != "#FFFFFF" {
		t.Errorf("on_accent for bright blue want #FFFFFF, got %s", d.OnAccent)
	}
	if d.Text == "" || d.Border == "" || d.Surface == "" {
		t.Errorf("derive should fill all semantic fields, got %+v", d)
	}
}

func TestDeriveHSLShift_Dark(t *testing.T) {
	d := derivePalette("#4285F4", "hsl-shift", true)
	if d.Bg == "#FFFFFF" {
		t.Errorf("dark bg should not be white, got %s", d.Bg)
	}
	if d.Text == "" {
		t.Errorf("dark text should be filled")
	}
}

func TestDerivePicksContrastOnColor(t *testing.T) {
	// 亮色主题色 → on_accent 应为黑
	d := derivePalette("#FFFF66", "hsl-shift", false)
	if d.OnAccent != "#000000" {
		t.Errorf("bright primary should yield on_accent=#000000, got %s", d.OnAccent)
	}
}

func TestDeriveNone(t *testing.T) {
	d := derivePalette("#4285F4", "none", false)
	if d.Accent != "" || d.Bg != "" {
		t.Errorf("none algorithm should return empty struct, got %+v", d)
	}
}

func TestApplyDerivedToVariant_PreservesUserValues(t *testing.T) {
	v := PaletteVariant{
		Bg:     "#ABCDEF", // 用户显式
		Accent: "",        // 待派生
	}
	d := derivedSemantics{Bg: "#FFFFFF", Accent: "#4285F4"}
	applyDerivedToVariant(&v, d)
	if v.Bg != "#ABCDEF" {
		t.Errorf("user bg should be preserved, got %s", v.Bg)
	}
	if v.Accent != "#4285F4" {
		t.Errorf("empty accent should be filled by derive, got %s", v.Accent)
	}
}

func TestExpandPaletteRefs_SimpleChain(t *testing.T) {
	v := &PaletteVariant{
		Bg:     "#FFFFFF",
		Accent: "${primary}",
		CandidateWindow: CandidateWindowPalette{
			Background: "${bg}",
			IndexBg:    "${accent}",
		},
	}
	if err := expandPaletteRefs(v, "#4285F4"); err != nil {
		t.Fatalf("expand failed: %v", err)
	}
	if v.Accent != "#4285F4" {
		t.Errorf("accent want #4285F4, got %s", v.Accent)
	}
	if v.CandidateWindow.Background != "#FFFFFF" {
		t.Errorf("cw.bg want #FFFFFF, got %s", v.CandidateWindow.Background)
	}
	if v.CandidateWindow.IndexBg != "#4285F4" {
		t.Errorf("cw.index_bg want #4285F4, got %s", v.CandidateWindow.IndexBg)
	}
}

func TestExpandPaletteRefs_TransparentLiteralPreserved(t *testing.T) {
	v := &PaletteVariant{
		Bg:     "#FFFFFF",
		Accent: "#4285F4",
		CandidateWindow: CandidateWindowPalette{
			Background: "transparent",
		},
	}
	if err := expandPaletteRefs(v, "#4285F4"); err != nil {
		t.Fatalf("expand failed: %v", err)
	}
	if v.CandidateWindow.Background != "transparent" {
		t.Errorf("literal transparent should pass through, got %s", v.CandidateWindow.Background)
	}
}

func TestExpandPaletteRefs_UnknownToken(t *testing.T) {
	v := &PaletteVariant{
		Bg:     "#FFFFFF",
		Accent: "${nosuch}",
	}
	if err := expandPaletteRefs(v, "#4285F4"); err == nil {
		t.Errorf("expected error for unknown token")
	}
}

// TestExpandPaletteRefs_OrderIndependent 验证迭代式展开不依赖字段顺序：
// bg → accent → primary 间接两跳引用，bg 在 accent 之前定义也能正常解析
func TestExpandPaletteRefs_OrderIndependent(t *testing.T) {
	v := &PaletteVariant{
		Bg:     "${accent}",  // 指向 accent
		Accent: "${primary}", // 指向 primary
	}
	if err := expandPaletteRefs(v, "#4285F4"); err != nil {
		t.Fatalf("两跳引用应支持: %v", err)
	}
	if v.Bg != "#4285F4" {
		t.Errorf("bg should resolve transitively to primary, got %s", v.Bg)
	}
	if v.Accent != "#4285F4" {
		t.Errorf("accent should resolve to primary, got %s", v.Accent)
	}
}

func TestExpandPaletteRefs_PrimaryRefRejected(t *testing.T) {
	v := &PaletteVariant{}
	if err := expandPaletteRefs(v, "${primary}"); err == nil {
		t.Errorf("primary itself must not contain ${} ref")
	}
}
