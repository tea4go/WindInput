// accessor.go — cmdbar config.get/set/toggle 使用的配置字段访问器注册表。
// 以 YAML 键路径（如 "ui.candidate_layout"）为索引，提供类型安全的读写和枚举循环切换。
// 不使用反射，而是显式函数表，确保类型安全且便于静态分析。
package config

import (
	"fmt"
	"strconv"
	"strings"
)

// FieldDesc 描述一个可通过 YAML 键路径访问的配置字段。
//   - Values 非空：枚举字段，Toggle 按 Values 顺序循环；
//   - IsBool=true：布尔字段，Toggle 翻转；
//   - 其余：不支持 Toggle（返回 error）。
type FieldDesc struct {
	Description string
	Values      []string // 枚举合法值（按循环顺序）；nil=非枚举
	IsBool      bool
	Get         func(cfg *Config) string
	Set         func(cfg *Config, value string) error
}

// ToggleValue 返回 Toggle 后应设置的目标值。
// 枚举字段循环（当前值不在 Values 中则返回 Values[0]），bool 字段翻转。
// 既非枚举也非 bool 时返回 error。
func (f FieldDesc) ToggleValue(cfg *Config) (string, error) {
	if f.IsBool {
		if f.Get(cfg) == "true" {
			return "false", nil
		}
		return "true", nil
	}
	if len(f.Values) == 0 {
		return "", fmt.Errorf("config: field does not support toggle (not bool or enum)")
	}
	cur := f.Get(cfg)
	for i, v := range f.Values {
		if v == cur {
			return f.Values[(i+1)%len(f.Values)], nil
		}
	}
	return f.Values[0], nil
}

// Section 返回 YAML 键路径的顶层区段（点分第一段），用于 coordinator 侧按区段调用对应的
// Update*Config 热更新方法。
func Section(key string) string {
	sec, _, _ := strings.Cut(key, ".")
	return sec
}

// Fields 是 YAML 路径 → FieldDesc 的注册表，键均为小写。
// 只覆盖通过 cmdbar 操作有实际意义的字段；复杂嵌套结构（hotkeys、special_modes 等）不暴露。
var Fields = map[string]FieldDesc{
	// ── UI ──────────────────────────────────────────────────────────────
	"ui.candidate_layout": {
		Description: "候选布局 horizontal（横排）| vertical（竖排）",
		Values:      []string{"horizontal", "vertical"},
		Get:         func(cfg *Config) string { return string(cfg.UI.CandidateLayout) },
		Set: func(cfg *Config, v string) error {
			l := CandidateLayout(v)
			if !l.Valid() {
				return fmt.Errorf("invalid candidate_layout %q (horizontal|vertical)", v)
			}
			cfg.UI.CandidateLayout = l
			return nil
		},
	},
	"ui.theme": {
		Description: "主题名称，如 default / msime 或自定义主题名",
		Get:         func(cfg *Config) string { return cfg.UI.Theme },
		Set: func(cfg *Config, v string) error {
			if v == "" {
				return fmt.Errorf("theme cannot be empty")
			}
			cfg.UI.Theme = v
			return nil
		},
	},
	"ui.theme_style": {
		Description: "主题风格 system（跟随系统）| light | dark",
		Values:      []string{"system", "light", "dark"},
		Get:         func(cfg *Config) string { return string(cfg.UI.ThemeStyle) },
		Set: func(cfg *Config, v string) error {
			s := ThemeStyle(v)
			if !s.Valid() {
				return fmt.Errorf("invalid theme_style %q (system|light|dark)", v)
			}
			cfg.UI.ThemeStyle = s
			return nil
		},
	},
	"ui.preedit_mode": {
		Description: "编码显示模式 top（独立行）| embedded（嵌入候选行前）",
		Values:      []string{"top", "embedded"},
		Get:         func(cfg *Config) string { return string(cfg.UI.PreeditMode) },
		Set: func(cfg *Config, v string) error {
			m := PreeditMode(v)
			if !m.Valid() {
				return fmt.Errorf("invalid preedit_mode %q (top|embedded)", v)
			}
			cfg.UI.PreeditMode = m
			return nil
		},
	},
	"ui.inline_preedit": {
		Description: "内嵌预编辑（true=嵌入应用输入框，false=在候选窗单独显示）",
		IsBool:      true,
		Get:         func(cfg *Config) string { return strconv.FormatBool(cfg.UI.InlinePreedit) },
		Set: func(cfg *Config, v string) error {
			b, err := parseBoolValue(v)
			if err != nil {
				return fmt.Errorf("invalid bool for inline_preedit: %w", err)
			}
			cfg.UI.InlinePreedit = b
			return nil
		},
	},
	"ui.hide_candidate_window": {
		Description: "隐藏候选窗（true=不显示候选框）",
		IsBool:      true,
		Get:         func(cfg *Config) string { return strconv.FormatBool(cfg.UI.HideCandidateWindow) },
		Set: func(cfg *Config, v string) error {
			b, err := parseBoolValue(v)
			if err != nil {
				return fmt.Errorf("invalid bool for hide_candidate_window: %w", err)
			}
			cfg.UI.HideCandidateWindow = b
			return nil
		},
	},
	"ui.candidates_per_page": {
		Description: "每页候选数（1-10）",
		Get:         func(cfg *Config) string { return strconv.Itoa(cfg.UI.CandidatesPerPage) },
		Set: func(cfg *Config, v string) error {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 || n > 10 {
				return fmt.Errorf("invalid candidates_per_page %q (1-10)", v)
			}
			cfg.UI.CandidatesPerPage = n
			return nil
		},
	},
	// ── S2T ─────────────────────────────────────────────────────────────
	"s2t.enabled": {
		Description: "简入繁出开关",
		IsBool:      true,
		Get:         func(cfg *Config) string { return strconv.FormatBool(cfg.S2T.Enabled) },
		Set: func(cfg *Config, v string) error {
			b, err := parseBoolValue(v)
			if err != nil {
				return fmt.Errorf("invalid bool for s2t.enabled: %w", err)
			}
			cfg.S2T.Enabled = b
			return nil
		},
	},
	"s2t.variant": {
		Description: "繁体变体 s2t（标准）| s2tw（台湾正体）| s2twp（台湾正体+词汇）| s2hk（香港繁体）",
		Values:      []string{"s2t", "s2tw", "s2twp", "s2hk"},
		Get:         func(cfg *Config) string { return string(cfg.S2T.Variant) },
		Set: func(cfg *Config, v string) error {
			sv := S2TVariant(v)
			if !sv.Valid() {
				return fmt.Errorf("invalid s2t.variant %q (s2t|s2tw|s2twp|s2hk)", v)
			}
			cfg.S2T.Variant = sv
			return nil
		},
	},
	// ── Input ────────────────────────────────────────────────────────────
	"input.filter_mode": {
		Description: "候选过滤模式 smart（智能）| general（常用字）| gb18030（不限制）",
		Values:      []string{"smart", "general", "gb18030"},
		Get:         func(cfg *Config) string { return string(cfg.Input.FilterMode) },
		Set: func(cfg *Config, v string) error {
			m := FilterMode(v)
			if !m.Valid() {
				return fmt.Errorf("invalid filter_mode %q (smart|general|gb18030)", v)
			}
			cfg.Input.FilterMode = m
			return nil
		},
	},
	"input.enter_behavior": {
		Description: "回车键行为 commit（上屏）| clear（清空）| commit_and_input | ignore",
		Values:      []string{"commit", "clear", "commit_and_input", "ignore"},
		Get:         func(cfg *Config) string { return string(cfg.Input.EnterBehavior) },
		Set: func(cfg *Config, v string) error {
			b := EnterBehavior(v)
			if !b.Valid() {
				return fmt.Errorf("invalid enter_behavior %q", v)
			}
			cfg.Input.EnterBehavior = b
			return nil
		},
	},
	"input.punct_follow_mode": {
		Description: "标点跟随模式（中英切换时标点同步切换）",
		IsBool:      true,
		Get:         func(cfg *Config) string { return strconv.FormatBool(cfg.Input.PunctFollowMode) },
		Set: func(cfg *Config, v string) error {
			b, err := parseBoolValue(v)
			if err != nil {
				return fmt.Errorf("invalid bool for punct_follow_mode: %w", err)
			}
			cfg.Input.PunctFollowMode = b
			return nil
		},
	},
	// ── Startup ──────────────────────────────────────────────────────────
	"startup.default_chinese_mode": {
		Description: "启动默认中文模式",
		IsBool:      true,
		Get:         func(cfg *Config) string { return strconv.FormatBool(cfg.Startup.DefaultChineseMode) },
		Set: func(cfg *Config, v string) error {
			b, err := parseBoolValue(v)
			if err != nil {
				return fmt.Errorf("invalid bool: %w", err)
			}
			cfg.Startup.DefaultChineseMode = b
			return nil
		},
	},
	"startup.default_full_width": {
		Description: "启动默认全角",
		IsBool:      true,
		Get:         func(cfg *Config) string { return strconv.FormatBool(cfg.Startup.DefaultFullWidth) },
		Set: func(cfg *Config, v string) error {
			b, err := parseBoolValue(v)
			if err != nil {
				return fmt.Errorf("invalid bool: %w", err)
			}
			cfg.Startup.DefaultFullWidth = b
			return nil
		},
	},
	"startup.default_chinese_punct": {
		Description: "启动默认中文标点",
		IsBool:      true,
		Get:         func(cfg *Config) string { return strconv.FormatBool(cfg.Startup.DefaultChinesePunct) },
		Set: func(cfg *Config, v string) error {
			b, err := parseBoolValue(v)
			if err != nil {
				return fmt.Errorf("invalid bool: %w", err)
			}
			cfg.Startup.DefaultChinesePunct = b
			return nil
		},
	},
}

// GetField 返回字段描述，路径不区分大小写。路径未注册时返回 (zero, false)。
func GetField(key string) (FieldDesc, bool) {
	f, ok := Fields[strings.ToLower(key)]
	return f, ok
}

// parseBoolValue 解析布尔值字符串，接受 true/false/1/0/yes/no/on/off（不区分大小写）。
func parseBoolValue(v string) (bool, error) {
	switch strings.ToLower(v) {
	case "true", "1", "yes", "on":
		return true, nil
	case "false", "0", "no", "off":
		return false, nil
	}
	return false, fmt.Errorf("expected true/false, got %q", v)
}
