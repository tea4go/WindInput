package theme

import "testing"

// P7-5：候选窗几何已迁 views/behavior，density 仅服务其它窗口（toolbar/status/tooltip/popup_menu/toast）。
// 本文件断言均针对这些保留窗口。

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
			if b.Toolbar.Padding.Top == 0 {
				t.Errorf("baseline Toolbar.Padding.Top should be non-zero")
			}
			if b.Tooltip.MaxWidth == 0 {
				t.Errorf("baseline Tooltip.MaxWidth should be non-zero")
			}
		})
	}
}

func TestDensityCozyIsBiggerThanCompact(t *testing.T) {
	c := densityBaseline("compact")
	z := densityBaseline("cozy")
	cm := densityBaseline("comfortable")

	if z.Tooltip.Padding.Top <= c.Tooltip.Padding.Top {
		t.Errorf("cozy Tooltip.Padding.Top %d should > compact %d",
			z.Tooltip.Padding.Top, c.Tooltip.Padding.Top)
	}
	if cm.Tooltip.Padding.Top <= z.Tooltip.Padding.Top {
		t.Errorf("comfortable Tooltip.Padding.Top %d should > cozy %d",
			cm.Tooltip.Padding.Top, z.Tooltip.Padding.Top)
	}
	if z.Tooltip.FontSize <= c.Tooltip.FontSize {
		t.Errorf("cozy tooltip font_size should > compact")
	}
}

func TestMergeWithDensityBaseline_PreservesUserFields(t *testing.T) {
	user := LayoutSchema{
		Density: "compact",
		Toolbar: RawToolbarLayout{
			ItemGap: intPtr(7), // 用户覆盖
		},
		Tooltip: RawTooltipLayout{
			MaxWidth: 999, // 用户覆盖（plain int，非零即覆盖）
		},
	}
	merged := mergeWithDensityBaseline(user)

	if merged.Toolbar.ItemGap != 7 {
		t.Errorf("Toolbar.ItemGap want 7, got %d", merged.Toolbar.ItemGap)
	}
	if merged.Tooltip.MaxWidth != 999 {
		t.Errorf("Tooltip.MaxWidth want 999, got %d", merged.Tooltip.MaxWidth)
	}
	// 未覆盖字段应保留基线值
	if merged.Toolbar.Padding.Top == 0 {
		t.Errorf("Toolbar.Padding.Top should retain baseline")
	}
	if merged.Tooltip.FontSize == 0 {
		t.Errorf("Tooltip.FontSize should retain baseline")
	}
}

func TestMergeWithDensityBaseline_DefaultDensityIsCompact(t *testing.T) {
	user := LayoutSchema{} // 完全空
	merged := mergeWithDensityBaseline(user)
	if merged.Density != "compact" {
		t.Errorf("default density want compact, got %q", merged.Density)
	}
	c := compactBaseline()
	if merged.Toolbar.Padding != c.Toolbar.Padding {
		t.Errorf("empty user should equal compact baseline")
	}
}

func TestMergeWithDensityBaseline_ExplicitZero(t *testing.T) {
	// 显式写 0 的指针型字段（*int）应被保留，而非回退基线
	user := LayoutSchema{
		Density: "compact",
		Tooltip: RawTooltipLayout{
			BorderRadius: intPtr(0),
		},
		Status: RawStatusLayout{
			Padding: RawPadding{Top: intPtr(0)},
		},
	}
	merged := mergeWithDensityBaseline(user)
	if merged.Tooltip.BorderRadius != 0 {
		t.Errorf("explicit Tooltip.BorderRadius=0 should be preserved, got %d", merged.Tooltip.BorderRadius)
	}
	if merged.Status.Padding.Top != 0 {
		t.Errorf("explicit Status.Padding.Top=0 should be preserved, got %d", merged.Status.Padding.Top)
	}
	// 未写的边（nil）应回退基线非零值
	if merged.Status.Padding.Left == 0 {
		t.Errorf("unset Status.Padding.Left should retain baseline (non-zero)")
	}

	// nil 字段应回退到基线非零值
	userNil := LayoutSchema{Density: "compact"}
	mergedNil := mergeWithDensityBaseline(userNil)
	if mergedNil.Tooltip.BorderRadius == 0 {
		t.Errorf("nil Tooltip.BorderRadius should fall back to non-zero baseline")
	}
	if mergedNil.Tooltip.Padding.Top == 0 {
		t.Errorf("nil Tooltip.Padding.Top should fall back to non-zero baseline")
	}
}
