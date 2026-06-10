package config

import (
	"bytes"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestEnumYAMLRoundTrip 验证所有具名 string 枚举类型的 YAML 序列化行为
// 与原始字符串完全等价 —— 即重构后旧 YAML 配置文件可以无损加载。
func TestEnumYAMLRoundTrip(t *testing.T) {
	cases := []struct {
		name  string
		typed any
		raw   string // 期望序列化后的 YAML 标量值
	}{
		{"EnterCommit", EnterCommit, "commit"},
		{"EnterClear", EnterClear, "clear"},
		{"EnterCommitAndInput", EnterCommitAndInput, "commit_and_input"},
		{"EnterIgnore", EnterIgnore, "ignore"},

		{"SpaceOnEmptyCommit", SpaceOnEmptyCommit, "commit"},
		{"SpaceOnEmptyClear", SpaceOnEmptyClear, "clear"},
		{"SpaceOnEmptyCommitAndInput", SpaceOnEmptyCommitAndInput, "commit_and_input"},
		{"SpaceOnEmptyIgnore", SpaceOnEmptyIgnore, "ignore"},

		{"OverflowIgnore", OverflowIgnore, "ignore"},
		{"OverflowCommit", OverflowCommit, "commit"},
		{"OverflowCommitAndInput", OverflowCommitAndInput, "commit_and_input"},

		{"FilterSmart", FilterSmart, "smart"},
		{"FilterGeneral", FilterGeneral, "general"},
		{"FilterGB18030", FilterGB18030, "gb18030"},

		{"ThemeStyleSystem", ThemeStyleSystem, "system"},
		{"ThemeStyleLight", ThemeStyleLight, "light"},
		{"ThemeStyleDark", ThemeStyleDark, "dark"},

		{"LayoutHorizontal", LayoutHorizontal, "horizontal"},
		{"LayoutVertical", LayoutVertical, "vertical"},

		{"PreeditTop", PreeditTop, "top"},
		{"PreeditEmbedded", PreeditEmbedded, "embedded"},

		{"PinyinSeparatorAuto", PinyinSeparatorAuto, "auto"},
		{"PinyinSeparatorQuote", PinyinSeparatorQuote, "quote"},
		{"PinyinSeparatorBacktick", PinyinSeparatorBacktick, "backtick"},
		{"PinyinSeparatorNone", PinyinSeparatorNone, "none"},

		{"FontEngineDirectWrite", FontEngineDirectWrite, "directwrite"},
		{"FontEngineGDI", FontEngineGDI, "gdi"},
		{"FontEngineFreetype", FontEngineFreetype, "freetype"},

		{"PagerBarDefault", PagerBarDefault, "\"\""},
		{"PagerBarAlways", PagerBarAlways, "always"},
		{"PagerBarAuto", PagerBarAuto, "auto"},
		{"PagerBarHide", PagerBarHide, "hide"},
		{"PageNumberDefault", PageNumberDefault, "\"\""},
		{"PageNumberShow", PageNumberShow, "show"},
		{"PageNumberHide", PageNumberHide, "hide"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, err := yaml.Marshal(c.typed)
			if err != nil {
				t.Fatalf("yaml.Marshal(%v) error: %v", c.typed, err)
			}
			got := strings.TrimSpace(string(data))
			if got != c.raw {
				t.Errorf("yaml.Marshal(%v) = %q, want %q", c.typed, got, c.raw)
			}
		})
	}
}

// TestEnumStringEquivalence 验证旧 YAML（裸字符串）能正确反序列化为新具名类型。
// 这是兼容性的核心保证：用户的 wind_input.yaml 在重构后无需修改即可加载。
func TestEnumStringEquivalence(t *testing.T) {
	t.Run("EnterBehavior", func(t *testing.T) {
		var v EnterBehavior
		if err := yaml.Unmarshal([]byte(`commit_and_input`), &v); err != nil {
			t.Fatal(err)
		}
		if v != EnterCommitAndInput {
			t.Errorf("got %q, want %q", v, EnterCommitAndInput)
		}
	})

	t.Run("FilterMode", func(t *testing.T) {
		var v FilterMode
		if err := yaml.Unmarshal([]byte(`smart`), &v); err != nil {
			t.Fatal(err)
		}
		if v != FilterSmart {
			t.Errorf("got %q, want %q", v, FilterSmart)
		}
	})

	t.Run("ThemeStyle dark legacy", func(t *testing.T) {
		var v ThemeStyle
		if err := yaml.Unmarshal([]byte(`dark`), &v); err != nil {
			t.Fatal(err)
		}
		if v != ThemeStyleDark {
			t.Errorf("got %q, want %q", v, ThemeStyleDark)
		}
	})

	t.Run("CandidateLayout", func(t *testing.T) {
		var v CandidateLayout
		if err := yaml.Unmarshal([]byte(`vertical`), &v); err != nil {
			t.Fatal(err)
		}
		if v != LayoutVertical {
			t.Errorf("got %q, want %q", v, LayoutVertical)
		}
	})
}

// TestDefaultConfigYAMLRoundTrip 是核心保险测试：
// DefaultConfig() → marshal → unmarshal → marshal 必须字节一致。
// 如果任何枚举字段的 YAML 序列化行为偏离原始 string 语义，此测试会失败。
func TestDefaultConfigYAMLRoundTrip(t *testing.T) {
	cfg := DefaultConfig()

	first, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("first Marshal: %v", err)
	}

	var reload Config
	if err := yaml.Unmarshal(first, &reload); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	second, err := yaml.Marshal(&reload)
	if err != nil {
		t.Fatalf("second Marshal: %v", err)
	}

	if !bytes.Equal(first, second) {
		t.Errorf("round-trip not byte-identical\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}

	// 抽样核对关键枚举字段值未丢失
	if reload.Input.EnterBehavior != cfg.Input.EnterBehavior {
		t.Errorf("EnterBehavior drift: %q vs %q", reload.Input.EnterBehavior, cfg.Input.EnterBehavior)
	}
	if reload.Input.FilterMode != cfg.Input.FilterMode {
		t.Errorf("FilterMode drift: %q vs %q", reload.Input.FilterMode, cfg.Input.FilterMode)
	}
	if reload.UI.Theme.Style != cfg.UI.Theme.Style {
		t.Errorf("ThemeStyle drift: %q vs %q", reload.UI.Theme.Style, cfg.UI.Theme.Style)
	}
	if reload.UI.Candidate.Layout != cfg.UI.Candidate.Layout {
		t.Errorf("Layout drift: %q vs %q", reload.UI.Candidate.Layout, cfg.UI.Candidate.Layout)
	}
	if reload.UI.Candidate.PreeditMode != cfg.UI.Candidate.PreeditMode {
		t.Errorf("PreeditMode drift: %q vs %q", reload.UI.Candidate.PreeditMode, cfg.UI.Candidate.PreeditMode)
	}
}

// TestLegacyYAMLLoading 验证裸字符串枚举值 unmarshal 后落到正确的具名常量
// （这些字符串字面量是历史上散落各处的 case 值；样本为 v1 结构，v0 旧结构
// 的加载走 migrateV0toV1，由 migration_v1_test.go 覆盖）。
func TestLegacyYAMLLoading(t *testing.T) {
	legacy := []byte(`
input:
  enter_behavior: commit_and_input
  space_on_empty_behavior: clear
  filter_mode: gb18030
ui:
  theme:
    style: dark
  candidate:
    layout: vertical
    preedit_mode: embedded
  font:
    render_mode: freetype
`)

	var cfg Config
	if err := yaml.Unmarshal(legacy, &cfg); err != nil {
		t.Fatalf("unmarshal legacy yaml: %v", err)
	}

	checks := []struct {
		name    string
		got     any
		want    any
		isValid func() bool
	}{
		{"enter_behavior", cfg.Input.EnterBehavior, EnterCommitAndInput, func() bool { return cfg.Input.EnterBehavior.Valid() }},
		{"space_on_empty_behavior", cfg.Input.SpaceOnEmptyBehavior, SpaceOnEmptyClear, func() bool { return cfg.Input.SpaceOnEmptyBehavior.Valid() }},
		{"filter_mode", cfg.Input.FilterMode, FilterGB18030, func() bool { return cfg.Input.FilterMode.Valid() }},
		{"theme_style", cfg.UI.Theme.Style, ThemeStyleDark, func() bool { return cfg.UI.Theme.Style.Valid() }},
		{"layout", cfg.UI.Candidate.Layout, LayoutVertical, func() bool { return cfg.UI.Candidate.Layout.Valid() }},
		{"preedit_mode", cfg.UI.Candidate.PreeditMode, PreeditEmbedded, func() bool { return cfg.UI.Candidate.PreeditMode.Valid() }},
		{"render_mode", cfg.UI.Font.RenderMode, FontEngineFreetype, func() bool { return cfg.UI.Font.RenderMode.Valid() }},
	}

	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if c.got != c.want {
				t.Errorf("got %v, want %v", c.got, c.want)
			}
			if !c.isValid() {
				t.Errorf("Valid() returned false for legitimate value %v", c.got)
			}
		})
	}
}

// TestEnumValidRejectsInvalid 确保 Valid() 拒绝空值与未知值，避免静默接受错配置。
func TestEnumValidRejectsInvalid(t *testing.T) {
	t.Run("empty string is invalid", func(t *testing.T) {
		if EnterBehavior("").Valid() {
			t.Error("empty EnterBehavior should be invalid")
		}
		if FilterMode("").Valid() {
			t.Error("empty FilterMode should be invalid")
		}
		if ThemeStyle("").Valid() {
			t.Error("empty ThemeStyle should be invalid")
		}
		if CandidateLayout("").Valid() {
			t.Error("empty CandidateLayout should be invalid")
		}
		if PreeditMode("").Valid() {
			t.Error("empty PreeditMode should be invalid")
		}
		if FontEngine("").Valid() {
			t.Error("empty FontEngine should be invalid")
		}
	})

	// PagerBarDisplay/PageNumberDisplay 特例：空字符串有效，表示"使用主题配置"
	t.Run("PagerBarDisplay empty string is valid", func(t *testing.T) {
		if !PagerBarDisplay("").Valid() {
			t.Error("empty PagerBarDisplay should be valid (use theme config)")
		}
	})
	t.Run("PageNumberDisplay empty string is valid", func(t *testing.T) {
		if !PageNumberDisplay("").Valid() {
			t.Error("empty PageNumberDisplay should be valid (use theme config)")
		}
	})

	t.Run("unknown values are invalid", func(t *testing.T) {
		if EnterBehavior("delete").Valid() {
			t.Error("unknown EnterBehavior should be invalid")
		}
		if FilterMode("strict").Valid() {
			t.Error("unknown FilterMode should be invalid")
		}
		if ThemeStyle("rainbow").Valid() {
			t.Error("unknown ThemeStyle should be invalid")
		}
		if CandidateLayout("grid").Valid() {
			t.Error("unknown CandidateLayout should be invalid")
		}
	})
}
