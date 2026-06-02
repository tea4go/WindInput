package theme

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// InlineTheme 把外链形态 Theme 展开为内联形态：
//   - Layout 字段从 string ID 变为 map[string]any（内联对象）
//   - Palette 字段同上
//   - Overrides 字段被合并进内联对象后清空
//
// 用于编辑器加载主题（编辑器内部统一处理内联）、用户分享主题（单 yaml 文件）。
//
// 注意：仅 Layout/Palette/Overrides 三个 v2.5 字段被深拷贝（通过 yaml round-trip）；
// 其它 v2/legacy 字段仅做 shallow copy。当前 Theme 这些字段均为值类型，无共享风险。
func (m *Manager) InlineTheme(t *Theme) (*Theme, error) {
	out := *t // shallow copy（仅复制顶层；下方会重建 v2.5 字段）

	// 解析 layout
	layout, err := m.resolveLayoutField(t.Layout)
	if err != nil {
		return nil, fmt.Errorf("inline layout: %w", err)
	}
	// 应用 overrides
	if t.Overrides != nil && len(t.Overrides.Layout) > 0 {
		if err := applyOverridesYAML(layout, t.Overrides.Layout); err != nil {
			return nil, fmt.Errorf("inline layout overrides: %w", err)
		}
	}
	layoutMap, err := structToMap(layout)
	if err != nil {
		return nil, err
	}

	// 解析 palette
	palette, _, err := m.resolvePaletteField(t.Palette, "")
	if err != nil {
		return nil, fmt.Errorf("inline palette: %w", err)
	}
	if t.Overrides != nil && len(t.Overrides.Palette) > 0 {
		if err := applyOverridesYAML(palette, t.Overrides.Palette); err != nil {
			return nil, fmt.Errorf("inline palette overrides: %w", err)
		}
	}
	paletteMap, err := structToMap(palette)
	if err != nil {
		return nil, err
	}

	out.Layout = layoutMap
	out.Palette = paletteMap
	out.Overrides = nil
	return &out, nil
}

// ExternalizeTheme 把内联形态拆为外链三件（写文件）：
//   - <outDir>/<themeID>/theme.yaml
//   - <outDir>/_layouts/<layoutID>.yaml
//   - <outDir>/_palettes/<paletteID>.yaml
//
// layoutID/paletteID 来自内联块的 meta.name；若为空则用 themeID 作为 ID 后缀。
// 返回写入的 layout/palette ID。
func ExternalizeTheme(t *Theme, outDir, themeID string) (string, string, error) {
	if !t.HasV25Schema() {
		return "", "", fmt.Errorf("theme 未使用 v2.5 schema，不能 externalize")
	}

	// 取 layout / palette schema
	var layoutSchema LayoutSchema
	var paletteSchema PaletteSchema
	if err := anyToStruct(t.Layout, &layoutSchema); err != nil {
		return "", "", fmt.Errorf("layout 必须为内联对象: %w", err)
	}
	if err := anyToStruct(t.Palette, &paletteSchema); err != nil {
		return "", "", fmt.Errorf("palette 必须为内联对象: %w", err)
	}

	layoutID := slugify(layoutSchema.Meta.Name, themeID+"-layout")
	paletteID := slugify(paletteSchema.Meta.Name, themeID+"-palette")

	// 写文件
	if err := writeYAMLFile(filepath.Join(outDir, "_layouts", layoutID+".yaml"), layoutSchema); err != nil {
		return "", "", err
	}
	if err := writeYAMLFile(filepath.Join(outDir, "_palettes", paletteID+".yaml"), paletteSchema); err != nil {
		return "", "", err
	}

	// 写 theme.yaml（外链形态）
	out := *t
	out.Layout = layoutID
	out.Palette = paletteID
	out.Overrides = nil
	if err := writeYAMLFile(filepath.Join(outDir, themeID, "theme.yaml"), out); err != nil {
		return "", "", err
	}

	return layoutID, paletteID, nil
}

// structToMap 把任意 struct 通过 yaml round-trip 转为 map[string]any，便于内联存储
func structToMap(v any) (map[string]any, error) {
	data, err := yaml.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// anyToStruct 把 string/map 形态的字段统一为 target struct（仅支持 map 形态；string 报错）
func anyToStruct[T any](v any, target *T) error {
	switch x := v.(type) {
	case nil:
		return fmt.Errorf("nil 字段")
	case string:
		return fmt.Errorf("外链字符串 ID 不能直接 externalize: %q", x)
	case map[string]any, map[any]any:
		data, err := yaml.Marshal(x)
		if err != nil {
			return err
		}
		return yaml.Unmarshal(data, target)
	default:
		return fmt.Errorf("不支持的字段类型: %T", v)
	}
}

func writeYAMLFile(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// slugify 把名字转为安全的文件名 ID；空时用 fallback
func slugify(name, fallback string) string {
	if name == "" {
		return fallback
	}
	out := make([]rune, 0, len(name))
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			out = append(out, r)
		case r == ' ':
			out = append(out, '-')
		}
	}
	// 去掉首尾的 '-'
	for len(out) > 0 && out[0] == '-' {
		out = out[1:]
	}
	for len(out) > 0 && out[len(out)-1] == '-' {
		out = out[:len(out)-1]
	}
	if len(out) == 0 {
		return fallback
	}
	return string(out)
}
