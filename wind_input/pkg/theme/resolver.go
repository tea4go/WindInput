package theme

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ResolveV25 把 v2.5 schema 的 Theme 解析为 ResolvedV25。
// 仅处理 v2.5 字段；调用方应先用 HasV25Schema() 判定。
// themeFileDir 用于解析背景图相对路径（外链形态时为 theme.yaml 所在目录，内联时为同左）。
func (m *Manager) ResolveV25(t *Theme, isDark bool, themeFileDir string) (*ResolvedV25, error) {
	// 1. 取出 layout / palette schema（区分外链字符串 ID 与内联对象）
	layout, err := m.resolveLayoutField(t.Layout)
	if err != nil {
		return nil, fmt.Errorf("layout 解析失败: %w", err)
	}
	if layout == nil {
		return nil, fmt.Errorf("layout 解析返回空指针")
	}
	palette, paletteFileDir, err := m.resolvePaletteField(t.Palette, themeFileDir)
	if err != nil {
		return nil, fmt.Errorf("palette 解析失败: %w", err)
	}
	if palette == nil {
		return nil, fmt.Errorf("palette 解析返回空指针")
	}

	// 2. 应用 overrides（仅外链形态会带 overrides；内联形态 t.Overrides 为 nil）
	if t.Overrides != nil {
		if len(t.Overrides.Layout) > 0 {
			if err := applyOverridesYAML(layout, t.Overrides.Layout); err != nil {
				return nil, fmt.Errorf("overrides.layout 应用失败: %w", err)
			}
		}
		if len(t.Overrides.Palette) > 0 {
			if err := applyOverridesYAML(palette, t.Overrides.Palette); err != nil {
				return nil, fmt.Errorf("overrides.palette 应用失败: %w", err)
			}
		}
	}

	// 3. layout 走 density 基线补全
	fullLayout := mergeWithDensityBaseline(*layout)

	// 4. palette 派生并展开引用
	fullPalette, err := finalizePalette(palette, isDark, paletteFileDir)
	if err != nil {
		return nil, fmt.Errorf("palette 派生失败: %w", err)
	}

	rv := &ResolvedV25{
		Meta:    t.Meta,
		Layout:  fullLayout,
		Palette: fullPalette,
	}
	// 5. views（v2.6 P2）：主题提供 views: 块时原样透传（仅显式写出的字段非 nil）。
	// 不在此 merge defaultViews 基线——渲染器以合成桥（layout 现状）为基线，仅用主题
	// 显式字段覆盖，保证未写字段沿用现状（零回归）。defaultViews/mergeViews 留待
	// 「无合成桥」的纯 views 主题（后续切片）。
	rv.Views = t.Views
	if t.Views != nil && m.logger != nil {
		m.logger.Info("主题提供盒模型 views，候选窗外观经 YAML 驱动 (P2)", "theme", t.Meta.Name)
	}
	return rv, nil
}

// resolveLayoutField 把 Theme.Layout 字段（string 或 map）规范化为 *LayoutSchema。
func (m *Manager) resolveLayoutField(v any) (*LayoutSchema, error) {
	switch x := v.(type) {
	case nil:
		return nil, fmt.Errorf("layout 字段为空")
	case string:
		return m.loadLayoutByID(x)
	case map[string]any, map[any]any:
		return decodeViaYAML[LayoutSchema](x)
	default:
		return nil, fmt.Errorf("layout 字段类型不支持: %T", v)
	}
}

// resolvePaletteField 同 resolveLayoutField，但额外返回 palette 文件所在目录，用于背景图相对路径。
// 内联形态返回 themeFileDir（背景图相对 theme.yaml）。
func (m *Manager) resolvePaletteField(v any, themeFileDir string) (*PaletteSchema, string, error) {
	switch x := v.(type) {
	case nil:
		return nil, "", fmt.Errorf("palette 字段为空")
	case string:
		ps, dir, err := m.loadPaletteByID(x)
		return ps, dir, err
	case map[string]any, map[any]any:
		ps, err := decodeViaYAML[PaletteSchema](x)
		return ps, themeFileDir, err
	default:
		return nil, "", fmt.Errorf("palette 字段类型不支持: %T", v)
	}
}

// decodeViaYAML 把 map[string]any (yaml 解析的中间态) 二次 marshal/unmarshal 为目标 struct。
func decodeViaYAML[T any](raw any) (*T, error) {
	data, err := yaml.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var out T
	if err := yaml.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// loadLayoutByID 在 themes/_layouts/<id>.yaml 中查找
func (m *Manager) loadLayoutByID(id string) (*LayoutSchema, error) {
	for _, dir := range m.themeDirs {
		p := filepath.Join(dir, "_layouts", id+".yaml")
		if _, err := os.Stat(p); err == nil {
			return loadYAMLFile[LayoutSchema](p)
		}
	}
	return nil, fmt.Errorf("layout 零件未找到: %s", id)
}

// loadPaletteByID 同 loadLayoutByID
func (m *Manager) loadPaletteByID(id string) (*PaletteSchema, string, error) {
	for _, dir := range m.themeDirs {
		p := filepath.Join(dir, "_palettes", id+".yaml")
		if _, err := os.Stat(p); err == nil {
			ps, err := loadYAMLFile[PaletteSchema](p)
			return ps, filepath.Dir(p), err
		}
	}
	return nil, "", fmt.Errorf("palette 零件未找到: %s", id)
}

func loadYAMLFile[T any](path string) (*T, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out T
	if err := yaml.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// applyOverridesYAML 把 overrides map 深度合并到 target struct 上。
// 实现：把 target marshal 为 map，深度合并 overrides，再 unmarshal 回 target。
func applyOverridesYAML[T any](target *T, overrides map[string]any) error {
	data, err := yaml.Marshal(target)
	if err != nil {
		return err
	}
	var base map[string]any
	if err := yaml.Unmarshal(data, &base); err != nil {
		return err
	}
	merged := deepMergeMaps(base, overrides)
	out, err := yaml.Marshal(merged)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(out, target)
}

// deepMergeMaps src 覆盖 dst；嵌套 map 递归合并；其它类型（含 list）整体替换
func deepMergeMaps(dst, src map[string]any) map[string]any {
	out := make(map[string]any, len(dst))
	for k, v := range dst {
		out[k] = v
	}
	for k, v := range src {
		if existing, ok := out[k]; ok {
			em, eOK := existing.(map[string]any)
			vm, vOK := v.(map[string]any)
			if eOK && vOK {
				out[k] = deepMergeMaps(em, vm)
				continue
			}
		}
		out[k] = v
	}
	return out
}

// finalizePalette 派生缺省语义色 → 展开 ${} 引用 → 解析背景图路径
func finalizePalette(p *PaletteSchema, isDark bool, paletteFileDir string) (ResolvedPalette, error) {
	var variant *PaletteVariant
	if isDark {
		variant = &p.Dark
	} else {
		variant = &p.Light
	}

	// 派生填充未给字段
	if p.Derive.Enabled || p.Derive.Algorithm != "none" {
		algo := p.Derive.Algorithm
		if algo == "" {
			algo = "hct"
		}
		d := derivePalette(p.Primary, algo, isDark)
		applyDerivedToVariant(variant, d)
	}

	// 展开 ${} 引用
	if err := expandPaletteRefs(variant, p.Primary); err != nil {
		return ResolvedPalette{}, err
	}

	out := ResolvedPalette{IsDark: isDark}
	out.Primary = parseColorOrTransparent(p.Primary)
	out.Bg = parseColorOrTransparent(variant.Bg)
	out.Surface = parseColorOrTransparent(variant.Surface)
	out.Border = parseColorOrTransparent(variant.Border)
	out.Text = parseColorOrTransparent(variant.Text)
	out.TextDim = parseColorOrTransparent(variant.TextDim)
	out.TextHint = parseColorOrTransparent(variant.TextHint)
	out.Accent = parseColorOrTransparent(variant.Accent)
	out.OnAccent = parseColorOrTransparent(variant.OnAccent)
	out.Shadow = parseColorOrTransparent(variant.Shadow)

	out.CandidateWindow = ResolvedCandidateWindowPalette{
		Background:   resolveColorWithFallback(variant.CandidateWindow.Background, out.Bg),
		Border:       resolveColorWithFallback(variant.CandidateWindow.Border, out.Border),
		Text:         resolveColorWithFallback(variant.CandidateWindow.Text, out.Text),
		Comment:      resolveColorWithFallback(variant.CandidateWindow.Comment, out.TextHint),
		IndexBg:      resolveColorWithFallback(variant.CandidateWindow.IndexBg, out.Accent),
		IndexText:    resolveColorWithFallback(variant.CandidateWindow.IndexText, out.OnAccent),
		HoverBg:      resolveColorWithFallback(variant.CandidateWindow.HoverBg, out.Surface),
		SelectedBg:   resolveColorWithFallback(variant.CandidateWindow.SelectedBg, out.Accent),
		SelectedText: resolveColorWithFallback(variant.CandidateWindow.SelectedText, out.OnAccent),
		PreeditBg:    resolveColorWithFallback(variant.CandidateWindow.PreeditBg, out.Surface),
		PreeditText:  resolveColorWithFallback(variant.CandidateWindow.PreeditText, out.TextDim),
		AccentBar:    resolveColorWithFallback(variant.CandidateWindow.AccentBar, out.Accent),
	}
	out.Toolbar = ResolvedToolbarPalette{
		Background:       resolveColorWithFallback(variant.Toolbar.Background, out.Bg),
		Border:           resolveColorWithFallback(variant.Toolbar.Border, out.Border),
		Grip:             resolveColorWithFallback(variant.Toolbar.Grip, out.TextHint),
		ModeChineseBg:    resolveColorWithFallback(variant.Toolbar.ModeChineseBg, out.Accent),
		ModeEnglishBg:    resolveColorWithFallback(variant.Toolbar.ModeEnglishBg, out.TextDim),
		ModeText:         resolveColorWithFallback(variant.Toolbar.ModeText, out.OnAccent),
		FullWidthOffBg:   resolveColorWithFallback(variant.Toolbar.FullWidthOffBg, out.Surface),
		FullWidthOffText: resolveColorWithFallback(variant.Toolbar.FullWidthOffText, out.TextDim),
		PunctEnglishBg:   resolveColorWithFallback(variant.Toolbar.PunctEnglishBg, out.Surface),
		PunctEnglishText: resolveColorWithFallback(variant.Toolbar.PunctEnglishText, out.TextDim),
		SettingsBg:       resolveColorWithFallback(variant.Toolbar.SettingsBg, out.Surface),
		SettingsIcon:     resolveColorWithFallback(variant.Toolbar.SettingsIcon, out.Accent),
		SettingsHole:     resolveColorWithFallback(variant.Toolbar.SettingsHole, out.Surface),
	}
	out.PopupMenu = ResolvedPopupMenuPalette{
		Background: resolveColorWithFallback(variant.PopupMenu.Background, out.Bg),
		Border:     resolveColorWithFallback(variant.PopupMenu.Border, out.Border),
		Text:       resolveColorWithFallback(variant.PopupMenu.Text, out.Text),
		Disabled:   resolveColorWithFallback(variant.PopupMenu.Disabled, out.TextHint),
		HoverBg:    resolveColorWithFallback(variant.PopupMenu.HoverBg, out.Accent),
		HoverText:  resolveColorWithFallback(variant.PopupMenu.HoverText, out.OnAccent),
		Separator:  resolveColorWithFallback(variant.PopupMenu.Separator, out.Border),
	}
	out.Tooltip = ResolvedTooltipPalette{
		Background: resolveColorWithFallback(variant.Tooltip.Background, out.Surface),
		Text:       resolveColorWithFallback(variant.Tooltip.Text, out.Text),
	}
	out.Status = ResolvedStatusPalette{
		Background: resolveColorWithFallback(variant.Status.Background, out.Bg),
		Border:     resolveColorWithFallback(variant.Status.Border, out.Accent),
		Text:       resolveColorWithFallback(variant.Status.Text, out.Text),
	}
	out.Toast = ResolvedToastPalette{
		Background: resolveColorWithFallback(variant.Toast.Background, out.Surface),
		Text:       resolveColorWithFallback(variant.Toast.Text, out.Text),
	}

	// 背景图：相对路径转绝对
	if p.Background != nil && p.Background.Image != "" {
		bg := &ResolvedBackground{
			Mode:  p.Background.Mode,
			Slice: p.Background.Slice,
		}
		if bg.Mode == "" {
			bg.Mode = "stretch"
		}
		if p.Background.Opacity != nil {
			bg.Opacity = *p.Background.Opacity
		} else {
			bg.Opacity = 1.0
		}
		img := p.Background.Image
		if isDark && p.Background.DarkImage != "" {
			img = p.Background.DarkImage
		}
		bg.ImagePath = resolveImagePath(img, paletteFileDir)
		out.Background = bg
	}

	return out, nil
}

// parseColorOrTransparent 解析颜色字符串；"transparent" 字面值返回完全透明色
func parseColorOrTransparent(s string) color.Color {
	if s == "" || s == "transparent" {
		return color.Transparent
	}
	c, err := ParseHexColor(s)
	if err != nil {
		return color.Transparent
	}
	return c
}

// resolveColorWithFallback 字符串非空则解析；否则用 fallback
func resolveColorWithFallback(s string, fallback color.Color) color.Color {
	if s == "" {
		return fallback
	}
	if s == "transparent" {
		return color.Transparent
	}
	c, err := ParseHexColor(s)
	if err != nil {
		return fallback
	}
	return c
}

// resolveImagePath data: URI 原样返回；相对路径拼到 paletteFileDir
func resolveImagePath(p string, baseDir string) string {
	if strings.HasPrefix(p, "data:") {
		return p
	}
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(baseDir, p)
}
