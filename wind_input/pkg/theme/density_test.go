package theme

import "testing"

func TestDensityBaselines(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string // density 字段回填值
	}{
		{"compact", "compact", "compact"},
		{"cozy", "cozy", "cozy"},
		{"comfortable", "comfortable", "comfortable"},
		{"unknown falls back to compact", "weird", "compact"},
		{"empty falls back to compact", "", "compact"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := densityBaseline(tc.input)
			if b.Density != tc.expect {
				t.Errorf("Density=%q want %q", b.Density, tc.expect)
			}
			if b.CandidateWindow.WindowPadding.Top == 0 {
				t.Errorf("baseline WindowPadding.Top should be non-zero")
			}
			if len(b.CandidateWindow.CandidateList.Index.Labels) != 10 {
				t.Errorf("baseline index.labels should have 10 slots, got %d", len(b.CandidateWindow.CandidateList.Index.Labels))
			}
		})
	}
}

func TestDensityCozyIsBiggerThanCompact(t *testing.T) {
	c := densityBaseline("compact")
	z := densityBaseline("cozy")
	cm := densityBaseline("comfortable")

	if z.CandidateWindow.WindowPadding.Top <= c.CandidateWindow.WindowPadding.Top {
		t.Errorf("cozy WindowPadding.Top %d should > compact %d",
			z.CandidateWindow.WindowPadding.Top, c.CandidateWindow.WindowPadding.Top)
	}
	if cm.CandidateWindow.WindowPadding.Top <= z.CandidateWindow.WindowPadding.Top {
		t.Errorf("comfortable WindowPadding.Top %d should > cozy %d",
			cm.CandidateWindow.WindowPadding.Top, z.CandidateWindow.WindowPadding.Top)
	}
	if z.CandidateWindow.CandidateList.Text.FontSize <= c.CandidateWindow.CandidateList.Text.FontSize {
		t.Errorf("cozy text font_size should > compact")
	}
}

func TestMergeWithDensityBaseline_PreservesUserFields(t *testing.T) {
	user := LayoutSchema{
		Density: "compact",
		CandidateWindow: RawCandidateWindowLayout{
			BandGap: intPtr(99), // 用户覆盖
			CandidateList: RawCandidateListLayout{
				ItemGap: intPtr(7), // 用户覆盖
				Index: RawIndexLayout{
					Labels: []string{"①", "②", "③"}, // 用户整体替换 labels
					Circle: true,                    // 用户开启圆圈
				},
			},
		},
	}
	merged := mergeWithDensityBaseline(user)

	if merged.CandidateWindow.BandGap != 99 {
		t.Errorf("BandGap want 99, got %d", merged.CandidateWindow.BandGap)
	}
	if merged.CandidateWindow.CandidateList.ItemGap != 7 {
		t.Errorf("ItemGap want 7, got %d", merged.CandidateWindow.CandidateList.ItemGap)
	}
	gotLabels := merged.CandidateWindow.CandidateList.Index.Labels
	if len(gotLabels) != 3 || gotLabels[0] != "①" || gotLabels[2] != "③" {
		t.Errorf("Index.Labels want [①②③], got %v", gotLabels)
	}
	if !merged.CandidateWindow.CandidateList.Index.Circle {
		t.Errorf("Index.Circle want true (user override)")
	}
	// 未覆盖字段应保留基线值
	if merged.CandidateWindow.WindowPadding.Top == 0 {
		t.Errorf("WindowPadding.Top should retain baseline")
	}
	if merged.CandidateWindow.CandidateList.Index.MinWidth == 0 {
		t.Errorf("Index.MinWidth should retain baseline")
	}
}

func TestMergeWithDensityBaseline_DefaultDensityIsCompact(t *testing.T) {
	user := LayoutSchema{} // 完全空
	merged := mergeWithDensityBaseline(user)
	if merged.Density != "compact" {
		t.Errorf("default density want compact, got %q", merged.Density)
	}
	c := compactBaseline()
	if merged.CandidateWindow.WindowPadding != c.CandidateWindow.WindowPadding {
		t.Errorf("empty user should equal compact baseline")
	}
}

func TestMergeWithDensityBaseline_ExplicitZero(t *testing.T) {
	// 显式写 0 的距离/圆角字段应被保留，而非回退基线
	user := LayoutSchema{
		Density: "compact",
		CandidateWindow: RawCandidateWindowLayout{
			BorderRadius:  intPtr(0),
			WindowPadding: RawPadding{Top: intPtr(0)},
		},
	}
	merged := mergeWithDensityBaseline(user)
	if merged.CandidateWindow.BorderRadius != 0 {
		t.Errorf("explicit BorderRadius=0 should be preserved, got %d", merged.CandidateWindow.BorderRadius)
	}
	if merged.CandidateWindow.WindowPadding.Top != 0 {
		t.Errorf("explicit WindowPadding.Top=0 should be preserved, got %d", merged.CandidateWindow.WindowPadding.Top)
	}
	// 未写的边（nil）应回退基线非零值
	if merged.CandidateWindow.WindowPadding.Left == 0 {
		t.Errorf("unset WindowPadding.Left should retain baseline (non-zero)")
	}

	// nil 字段应回退到基线非零值
	userNil := LayoutSchema{Density: "compact"}
	mergedNil := mergeWithDensityBaseline(userNil)
	if mergedNil.CandidateWindow.BorderRadius == 0 {
		t.Errorf("nil BorderRadius should fall back to non-zero baseline")
	}
	if mergedNil.CandidateWindow.WindowPadding.Top == 0 {
		t.Errorf("nil WindowPadding.Top should fall back to non-zero baseline")
	}
}
