package theme

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

// refRe 匹配 ${name} 引用，name 由字母数字下划线构成；不支持嵌套引用（无点号）
var refRe = regexp.MustCompile(`\$\{([a-zA-Z_][a-zA-Z0-9_]*)\}`)

// expandPaletteRefs 展开 PaletteVariant 中所有 ${name} 引用。
// 支持引用的 token：primary、变体内的顶层语义色（bg / surface / border / text / accent / ...）。
// 不允许两级引用（被引用值再含 ${...} 报错）。
func expandPaletteRefs(v *PaletteVariant, primary string) error {
	// 先建一张可引用值表 — 注意：顺序很重要。
	// 用户可能写 accent: "${primary}"；展开 accent 时需要先解析 primary。
	// 此处约定 primary 已是字面值（不能为 ${xxx}），由 PaletteSchema 校验保证。
	if hasRef(primary) {
		return fmt.Errorf("palette.primary 不允许使用 ${} 引用: %q", primary)
	}

	table := map[string]string{
		"primary": primary,
	}

	// 顶层语义色之间也可互引（如 accent: ${primary}）。
	// 用迭代式展开：每轮把无引用的字面值加入 table；最多 N 轮（防循环）。
	semFields := []struct {
		name string
		ptr  *string
	}{
		{"bg", &v.Bg},
		{"surface", &v.Surface},
		{"border", &v.Border},
		{"text", &v.Text},
		{"text_dim", &v.TextDim},
		{"text_hint", &v.TextHint},
		{"accent", &v.Accent},
		{"on_accent", &v.OnAccent},
		{"shadow", &v.Shadow},
	}

	const maxIter = 4
	for iter := 0; iter < maxIter; iter++ {
		progress := false
		for _, f := range semFields {
			if _, done := table[f.name]; done {
				continue
			}
			if *f.ptr == "" {
				continue
			}
			if !hasRef(*f.ptr) {
				table[f.name] = *f.ptr
				progress = true
				continue
			}
			expanded, ok := tryExpand(*f.ptr, table)
			if ok {
				if hasRef(expanded) {
					// 引用目标本身又是引用 — 第二级，禁止
					return fmt.Errorf("palette.%s 引用链超过 1 级: %q", f.name, *f.ptr)
				}
				*f.ptr = expanded
				table[f.name] = expanded
				progress = true
			}
		}
		if !progress {
			break
		}
	}

	// 检查是否仍有未解析的顶层引用
	for _, f := range semFields {
		if *f.ptr != "" && hasRef(*f.ptr) {
			return fmt.Errorf("palette.%s 引用未能解析: %q", f.name, *f.ptr)
		}
	}

	// 展开各组件块中的引用：用 reflect 遍历 string 字段
	componentVals := []reflect.Value{
		reflect.ValueOf(&v.CandidateWindow).Elem(),
		reflect.ValueOf(&v.Toolbar).Elem(),
		reflect.ValueOf(&v.PopupMenu).Elem(),
		reflect.ValueOf(&v.Tooltip).Elem(),
		reflect.ValueOf(&v.Status).Elem(),
		reflect.ValueOf(&v.Toast).Elem(),
	}
	for _, cv := range componentVals {
		if err := expandStringFields(cv, table); err != nil {
			return err
		}
	}
	return nil
}

func expandStringFields(rv reflect.Value, table map[string]string) error {
	t := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		fv := rv.Field(i)
		if fv.Kind() != reflect.String || !fv.CanSet() {
			continue
		}
		s := fv.String()
		if s == "" || !hasRef(s) {
			continue
		}
		expanded, ok := tryExpand(s, table)
		if !ok {
			return fmt.Errorf("palette.%s 字段引用未知 token: %q", t.Field(i).Name, s)
		}
		if hasRef(expanded) {
			return fmt.Errorf("palette.%s 字段引用链超过 1 级: %q", t.Field(i).Name, s)
		}
		fv.SetString(expanded)
	}
	return nil
}

func hasRef(s string) bool {
	return strings.Contains(s, "${")
}

// tryExpand 把 ${name} 替换为 table[name]；只要有一个 name 不在 table 则返回 (s, false)。
// 全部成功时返回 (expanded, true)。
func tryExpand(s string, table map[string]string) (string, bool) {
	allFound := true
	out := refRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := refRe.FindStringSubmatch(m)
		name := sub[1]
		val, ok := table[name]
		if !ok {
			allFound = false
			return m
		}
		return val
	})
	return out, allFound
}
