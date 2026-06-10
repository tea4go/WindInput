package config

// migrateV0toV1 把 v0（旧版 YAML 结构）配置 map 就地迁移为 v1 新结构。
// 完整 key 映射表见 docs/design/config-restructure.md §6；同时熔合 v0 时代的
// 三个启发式迁移（quick_input 旧字段、theme:"dark"、status_indicator 旧顶层
// 字段回填）。FontSizeFollowTheme 旧探针已于 v0 时代移除（缺失=默认 true），
// 直接平移无需启发式。
//
// 本函数操作的 map 恒来自 yaml.v3 解析（v0 必为 YAML），取值一律走 safeGet*
// （migrate.go），脏数据按键缺失降级，不 panic。所有搬移遵循"目标键已存在
// 则不覆盖"（防双写场景下丢新值）。
func migrateV0toV1(m map[string]any) {
	// 1. startup → general
	renameKeyV1(m, "startup", "general")

	// 2. input 子表：改名 + quick_input/special_modes 外迁 features
	if input, ok := safeGetMap(m, "input"); ok {
		renameKeyV1(input, "capslock_behavior", "capslock")
		renameKeyV1(input, "overflow_behavior", "overflow")

		if qi, ok := safeGetMap(input, "quick_input"); ok {
			migrateQuickInputMapV0(qi)
			putIfAbsent(ensureMapV1(m, "features"), "quick_input", qi)
			delete(input, "quick_input")
		}
		if v, ok := input["special_modes"]; ok {
			// 数组整体平移；实例内全部字段（含预留）同名不动
			putIfAbsent(ensureMapV1(m, "features"), "special_modes", v)
			delete(input, "special_modes")
		}
	}

	// 3. 顶层节迁移：stats/s2t → features.*；toolbar → ui.toolbar；
	//    advanced → debug + compat
	for _, k := range []string{"stats", "s2t"} {
		if v, ok := m[k]; ok {
			putIfAbsent(ensureMapV1(m, "features"), k, v)
			delete(m, k)
		}
	}
	if v, ok := m["toolbar"]; ok {
		putIfAbsent(ensureMapV1(m, "ui"), "toolbar", v)
		delete(m, "toolbar")
	}
	if adv, ok := safeGetMap(m, "advanced"); ok {
		if v, ok := adv["log_level"]; ok {
			putIfAbsent(ensureMapV1(m, "debug"), "log_level", v)
		}
		if v, ok := adv["perf_sampling"]; ok {
			putIfAbsent(ensureMapV1(m, "debug"), "perf_sampling", v)
		}
		if v, ok := adv["host_render_processes"]; ok {
			putIfAbsent(ensureMapV1(m, "compat"), "host_render_processes", v)
		}
		delete(m, "advanced")
	}

	// 4. ui 拆分（candidate/font/theme 子表 + tooltip_delay/status_indicator
	//    旧键 + accent/cmdbar 外迁）
	if ui, ok := safeGetMap(m, "ui"); ok {
		migrateUIMapV0(m, ui)
	}
}

// migrateUIMapV0 拆分 v0 的扁平 ui 节。
func migrateUIMapV0(root, ui map[string]any) {
	// candidate 子表（旧键 → 新键）
	moveKeysLazy(ui, map[string]string{
		"font_size":                       "font_size",
		"font_size_follow_theme":          "font_size_follow_theme",
		"candidates_per_page":             "per_page",
		"candidates_per_page_extended":    "per_page_extended",
		"max_candidate_chars":             "max_chars",
		"candidate_layout":                "layout",
		"inline_preedit":                  "inline_preedit",
		"preedit_mode":                    "preedit_mode",
		"flip_layout_when_above":          "flip_when_above",
		"hide_candidate_window":           "hide_window",
		"candidate_index_labels":          "index_labels",
		"mode_accent_border":              "mode_accent_border",
		"always_show_pager":               "always_show_pager",
		"always_show_pager_follow_theme":  "always_show_pager_follow_theme",
		"show_page_number":                "show_page_number",
		"show_page_number_follow_theme":   "show_page_number_follow_theme",
		"vertical_max_width":              "vertical_max_width",
		"vertical_max_width_follow_theme": "vertical_max_width_follow_theme",
		"pager_bar_display":               "pager_bar_display",
		"page_number_display":             "page_number_display",
	}, ui, "candidate")

	// font 子表
	moveKeysLazy(ui, map[string]string{
		"font_family":      "family",
		"font_path":        "path",
		"text_render_mode": "render_mode",
		"gdi_font_weight":  "gdi_weight",
		"gdi_font_scale":   "gdi_scale",
		"menu_font_weight": "menu_weight",
		"menu_font_size":   "menu_size",
	}, ui, "font")

	// theme 子表：旧 ui.theme 是字符串，与新子表同名——必须先取出再建子表。
	// 熔合旧启发式：theme=="dark" → name="default" + style="dark"（覆盖已有 style）。
	themeName, hasName := safeGetString(ui, "theme")
	if hasName {
		delete(ui, "theme")
	}
	themeStyle, hasStyle := safeGetString(ui, "theme_style")
	if _, exists := ui["theme_style"]; exists {
		delete(ui, "theme_style")
	}
	if hasName && themeName == "dark" {
		themeName = "default"
		themeStyle = "dark"
		hasStyle = true
	}
	_, hasEditorAuto := ui["theme_editor_auto_start"]
	if hasName || hasStyle || hasEditorAuto {
		th := ensureMapV1(ui, "theme")
		if hasName {
			putIfAbsent(th, "name", themeName)
		}
		if hasStyle {
			putIfAbsent(th, "style", themeStyle)
		}
		if v, ok := ui["theme_editor_auto_start"]; ok {
			putIfAbsent(th, "editor_auto_start", v)
			delete(ui, "theme_editor_auto_start")
		}
	}

	// tooltip：子表同名平移（无需动），仅吸收 tooltip_delay → tooltip.delay
	if v, ok := ui["tooltip_delay"]; ok {
		putIfAbsent(ensureMapV1(ui, "tooltip"), "delay", v)
		delete(ui, "tooltip_delay")
	}

	// status_indicator：子表同名平移；三个旧顶层键仅当新键缺失且旧值有效时回填
	// （熔合 migrateStatusIndicatorConfig：duration 要求 >0，offset 要求 !=0）。
	for from, spec := range map[string]struct {
		to      string
		nonZero bool // true=偏移类（!=0 有效），false=时长类（>0 有效）
	}{
		"status_indicator_duration": {"duration", false},
		"status_indicator_offset_x": {"offset_x", true},
		"status_indicator_offset_y": {"offset_y", true},
	} {
		if n, ok := safeGetInt(ui, from); ok {
			valid := n > 0
			if spec.nonZero {
				valid = n != 0
			}
			if valid {
				putIfAbsent(ensureMapV1(ui, "status_indicator"), spec.to, n)
			}
		}
		delete(ui, from)
	}

	// 外迁：accent 颜色按功能就近，cmdbar 前缀进 features.cmdbar
	if v, ok := ui["temp_pinyin_accent_color"]; ok {
		tp := ensureMapV1(ensureMapV1(root, "input"), "temp_pinyin")
		putIfAbsent(tp, "accent_color", v)
		delete(ui, "temp_pinyin_accent_color")
	}
	if v, ok := ui["quick_input_accent_color"]; ok {
		qi := ensureMapV1(ensureMapV1(root, "features"), "quick_input")
		putIfAbsent(qi, "accent_color", v)
		delete(ui, "quick_input_accent_color")
	}
	if v, ok := ui["cmdbar_candidate_prefix"]; ok {
		cb := ensureMapV1(ensureMapV1(root, "features"), "cmdbar")
		putIfAbsent(cb, "candidate_prefix", v)
		delete(ui, "cmdbar_candidate_prefix")
	}
}

// migrateQuickInputMapV0 熔合 quick_input 的 v0 旧字段启发式
// （原 migrateQuickInputConfig，struct 层 → map 层等价改写）：
//   - trigger_key 非空且 trigger_keys 缺失：enabled 缺失或 true 时迁入列表；
//   - enabled 显式 false：清空 trigger_keys（空列表=关闭）；
//   - 两个旧键一律删除。
func migrateQuickInputMapV0(qi map[string]any) {
	enabled, hasEnabled := safeGetBool(qi, "enabled")
	if tk, ok := safeGetString(qi, "trigger_key"); ok && tk != "" {
		if _, hasTKs := qi["trigger_keys"]; !hasTKs {
			if !hasEnabled || enabled {
				qi["trigger_keys"] = []any{tk}
			}
		}
	}
	if hasEnabled && !enabled {
		qi["trigger_keys"] = []any{}
	}
	delete(qi, "enabled")
	delete(qi, "trigger_key")
}

// ---- map 搬移辅助（仅迁移函数使用）----

// renameKeyV1 把 m[from] 改名为 m[to]（to 已存在则丢弃 from，避免覆盖新值）。
func renameKeyV1(m map[string]any, from, to string) {
	if v, ok := m[from]; ok {
		putIfAbsent(m, to, v)
		delete(m, from)
	}
}

// putIfAbsent 仅当 key 不存在时写入。
func putIfAbsent(m map[string]any, key string, v any) {
	if _, exists := m[key]; !exists {
		m[key] = v
	}
}

// ensureMapV1 取或建子表。key 已存在但不是 map（脏数据）时覆盖为空表
// （脏值丢弃，降级为默认——与 safeGet* 的"键缺失"语义一致）。
func ensureMapV1(m map[string]any, key string) map[string]any {
	if sub, ok := safeGetMap(m, key); ok {
		return sub
	}
	sub := map[string]any{}
	m[key] = sub
	return sub
}

// moveKeysLazy 按映射表把 src 的键搬进 src[subKey] 子表；
// 仅在确有旧键时才创建子表（不给空配置凭空生成子表）。
func moveKeysLazy(src map[string]any, mapping map[string]string, parent map[string]any, subKey string) {
	for from, to := range mapping {
		if v, ok := src[from]; ok {
			putIfAbsent(ensureMapV1(parent, subKey), to, v)
			delete(src, from)
		}
	}
}
