package theme

import "testing"

func TestResolvedToLegacy_Smoke(t *testing.T) {
	tmp, cleanup := setupTestThemes(t)
	defer cleanup()
	m := makeTestManager(tmp)

	theme := &Theme{Layout: "test-layout", Palette: "test-palette"}
	rv, err := m.ResolveV25(theme, false, tmp)
	if err != nil {
		t.Fatal(err)
	}
	legacy := ResolvedToLegacy(rv)
	if legacy == nil {
		t.Fatal("legacy nil")
	}
	// 验证 index 模板传导
	if legacy.Style.IndexStyle != "text" {
		t.Errorf("IndexStyle want text (from \"1.\"), got %q", legacy.Style.IndexStyle)
	}
	if legacy.Style.IndexLabels != "1./2./3./4./5./6./7./8./9./0." {
		t.Errorf("IndexLabels mismatch: %q", legacy.Style.IndexLabels)
	}
	// 颜色映射
	if ColorToHexRGB(legacy.CandidateWindow.SelectedBgColor) != "#4285F4" {
		t.Errorf("SelectedBgColor want #4285F4, got %s", ColorToHexRGB(legacy.CandidateWindow.SelectedBgColor))
	}
	// 尺寸映射
	if legacy.Style.CornerRadius == 0 {
		t.Errorf("CornerRadius should be filled from layout")
	}
}

func TestBuildIndexLabelsFromSlots(t *testing.T) {
	cases := []struct {
		name   string
		labels []string
		want   string
	}{
		{
			name:   "full emoji set",
			labels: []string{"🍎", "🍊", "🍇", "🍉", "🍓", "🍑", "🍒", "🥝", "🍍", "🥥"},
			want:   "🍎/🍊/🍇/🍉/🍓/🍑/🍒/🥝/🍍/🥥",
		},
		{
			name:   "partial fills rest with default digits",
			labels: []string{"壹", "贰", "叁"},
			want:   "壹/贰/叁/4/5/6/7/8/9/0",
		},
		{
			name:   "empty slot falls back to default digit",
			labels: []string{"A", "", "C"},
			want:   "A/2/C/4/5/6/7/8/9/0",
		},
		{
			name:   "more than 10 ignores extras",
			labels: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "0", "x", "y"},
			want:   "1/2/3/4/5/6/7/8/9/0",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := buildIndexLabelsFromSlots(c.labels); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// ResolvedToLegacy：labels 是序号唯一来源，空槽回退默认数字；circle 仅控制 IndexStyle，与 labels 内容正交
func TestResolvedToLegacy_LabelsDriveIndex(t *testing.T) {
	rv := &ResolvedV25{}
	rv.Layout.CandidateWindow.CandidateList.Index.Labels = []string{"甲", "乙", "丙"}

	out := ResolvedToLegacy(rv)
	if out.Style.IndexStyle != "text" {
		t.Errorf("IndexStyle want text (circle off), got %q", out.Style.IndexStyle)
	}
	if out.Style.IndexLabels != "甲/乙/丙/4/5/6/7/8/9/0" {
		t.Errorf("IndexLabels mismatch, got %q", out.Style.IndexLabels)
	}
}

// circle=true → IndexStyle "circle"，且 labels 内容照样透传（圆里放自定义字符）
func TestResolvedToLegacy_CircleIsOrthogonalToLabels(t *testing.T) {
	rv := &ResolvedV25{}
	rv.Layout.CandidateWindow.CandidateList.Index.Circle = true
	rv.Layout.CandidateWindow.CandidateList.Index.Labels = []string{"壹", "贰"}

	out := ResolvedToLegacy(rv)
	if out.Style.IndexStyle != "circle" {
		t.Errorf("IndexStyle want circle, got %q", out.Style.IndexStyle)
	}
	if out.Style.IndexLabels != "壹/贰/3/4/5/6/7/8/9/0" {
		t.Errorf("IndexLabels mismatch, got %q", out.Style.IndexLabels)
	}
}

// 无 labels（空数组）→ 全部回退默认数字
func TestResolvedToLegacy_EmptyLabelsAllDigits(t *testing.T) {
	rv := &ResolvedV25{}
	out := ResolvedToLegacy(rv)
	if out.Style.IndexStyle != "text" || out.Style.IndexLabels != "1/2/3/4/5/6/7/8/9/0" {
		t.Errorf("want text + default digits, got %q/%q", out.Style.IndexStyle, out.Style.IndexLabels)
	}
}
