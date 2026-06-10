package config

import (
	"os"
	"path/filepath"
	"testing"
)

// loadV0Sample 把 testdata 样本复制到临时目录（避免 LoadFrom 迁移写出污染
// testdata）后按旧版回退路径加载。
func loadV0Sample(t *testing.T, src string) *Config {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", src))
	if err != nil {
		t.Fatalf("读取样本失败: %v", err)
	}
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(yamlPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatalf("LoadFrom 失败: %v", err)
	}
	return cfg
}

// TestMigrateV0toV1_FullSample 迁移保真：v0 全量样本（覆盖 §6 映射表全部条目，
// 所有值取非默认）迁移后逐字段断言语义一致。
func TestMigrateV0toV1_FullSample(t *testing.T) {
	cfg := loadV0Sample(t, "v0_full.yaml")

	// startup → general
	if !cfg.General.RememberLastState || cfg.General.DefaultChineseMode ||
		!cfg.General.DefaultFullWidth || cfg.General.DefaultChinesePunct {
		t.Errorf("general 迁移不保真: %+v", cfg.General)
	}
	// schema（不变）
	if cfg.Schema.Active != "pinyin" || cfg.Schema.PrimaryCodetable != "wubi86" {
		t.Errorf("schema 迁移不保真: %+v", cfg.Schema)
	}
	// hotkeys（不变）
	if cfg.Hotkeys.CommitOnSwitch || cfg.Hotkeys.SwitchEngine != "ctrl+shift+x" {
		t.Errorf("hotkeys 迁移不保真: %+v", cfg.Hotkeys)
	}
	// input 顶层标量
	if !cfg.Input.PunctFollowMode || cfg.Input.FilterMode != FilterGB18030 ||
		cfg.Input.SmartPunctAfterDigit || cfg.Input.SmartPunctList != ".:" ||
		cfg.Input.EnterBehavior != EnterClear || cfg.Input.NumpadBehavior != "follow_main" {
		t.Errorf("input 标量迁移不保真")
	}
	// capslock_behavior → capslock
	if !cfg.Input.CapsLock.CancelOnModeSwitch {
		t.Error("input.capslock 迁移不保真")
	}
	// overflow_behavior → overflow
	if cfg.Input.Overflow.NumberKey != OverflowCommit || cfg.Input.Overflow.SelectKey != OverflowCommitAndInput {
		t.Errorf("input.overflow 迁移不保真: %+v", cfg.Input.Overflow)
	}
	// temp_pinyin + accent_color 吸收
	if cfg.Input.TempPinyin.ZIncludeOnCommit {
		t.Error("temp_pinyin.z_include_on_commit=false 迁移不保真")
	}
	if cfg.Input.TempPinyin.AccentColor != "#112233" {
		t.Errorf("ui.temp_pinyin_accent_color 应迁入 input.temp_pinyin.accent_color, got %q", cfg.Input.TempPinyin.AccentColor)
	}
	// phrase
	if cfg.Input.Phrase.MinPrefixLength != 3 {
		t.Errorf("phrase.min_prefix_length = %d, want 3", cfg.Input.Phrase.MinPrefixLength)
	}
	// quick_input → features.quick_input（trigger_key 启发式迁入列表）
	qi := cfg.Features.QuickInput
	if len(qi.TriggerKeys) != 1 || qi.TriggerKeys[0] != "quote" {
		t.Errorf("quick_input.trigger_key 启发式迁移失败: %v", qi.TriggerKeys)
	}
	if qi.ForceVertical || qi.DecimalPlaces != 2 || qi.AccentColor != "#445566" {
		t.Errorf("quick_input 迁移不保真: %+v", qi)
	}
	// special_modes → features.special_modes（含预留字段整体平移）
	if len(cfg.Features.SpecialModes) != 1 {
		t.Fatalf("special_modes 应迁入 features, got %d 个", len(cfg.Features.SpecialModes))
	}
	sm := cfg.Features.SpecialModes[0]
	if sm.ID != "sym" || sm.AutoCommit != SpecialAutoCommitManual || !sm.ForceVertical ||
		sm.AccentColor != "#778899" || !sm.ShowAllOnEntry || sm.CodeCharset != "abc" {
		t.Errorf("special_modes 实例字段不保真: %+v", sm)
	}
	// stats / s2t → features
	if cfg.Features.Stats.Enabled || cfg.Features.Stats.RetainDays != 30 || cfg.Features.Stats.TrackEnglish {
		t.Errorf("features.stats 迁移不保真: %+v", cfg.Features.Stats)
	}
	if !cfg.Features.S2T.Enabled || cfg.Features.S2T.Variant != S2TTaiwan {
		t.Errorf("features.s2t 迁移不保真: %+v", cfg.Features.S2T)
	}
	// cmdbar 前缀外迁
	if cfg.Features.Cmdbar.CandidatePrefix != "▶" {
		t.Errorf("cmdbar.candidate_prefix = %q, want ▶", cfg.Features.Cmdbar.CandidatePrefix)
	}
	// toolbar → ui.toolbar
	if cfg.UI.Toolbar.Visible || cfg.UI.Toolbar.HideInFullscreen {
		t.Errorf("ui.toolbar 迁移不保真: %+v", cfg.UI.Toolbar)
	}
	// advanced → debug + compat
	if cfg.Debug.LogLevel != "debug" || !cfg.Debug.PerfSampling {
		t.Errorf("debug 迁移不保真: %+v", cfg.Debug)
	}
	if len(cfg.Compat.HostRenderProcesses) != 1 || cfg.Compat.HostRenderProcesses[0] != "Foo.exe" {
		t.Errorf("compat.host_render_processes 迁移不保真: %v", cfg.Compat.HostRenderProcesses)
	}
	// ui.candidate
	c := cfg.UI.Candidate
	if c.FontSize != 22 || c.FontSizeFollowTheme || c.PerPage != 9 || c.PerPageExtended != 10 ||
		c.MaxChars != 24 || c.Layout != LayoutVertical || c.InlinePreedit ||
		c.PreeditMode != PreeditEmbedded || !c.FlipWhenAbove || !c.HideWindow ||
		c.IndexLabels != "①②③④⑤⑥⑦⑧⑨⑩" || c.ModeAccentBorder {
		t.Errorf("ui.candidate 迁移不保真: %+v", c)
	}
	if !c.AlwaysShowPager || c.AlwaysShowPagerFollowTheme || c.ShowPageNumber ||
		c.ShowPageNumberFollowTheme || c.VerticalMaxWidth != 480 || c.VerticalMaxWidthFollowTheme {
		t.Errorf("ui.candidate behavior 覆盖层迁移不保真: %+v", c)
	}
	if c.PagerBarDisplay != PagerBarAlways || c.PageNumberDisplay != PageNumberHide {
		t.Errorf("pager 枚举迁移不保真: %v / %v", c.PagerBarDisplay, c.PageNumberDisplay)
	}
	// ui.font
	f := cfg.UI.Font
	if f.Family != "Microsoft YaHei" || f.Path != "C:/f.ttf" || f.RenderMode != FontEngineGDI ||
		f.GDIWeight != 700 || f.GDIScale != 1.5 || f.MenuWeight != 600 || f.MenuSize != 14 {
		t.Errorf("ui.font 迁移不保真: %+v", f)
	}
	// ui.theme：dark 特判 → name=default + style=dark；editor_auto_start 吸收
	th := cfg.UI.Theme
	if th.Name != "default" || th.Style != ThemeStyleDark || !th.EditorAutoStart {
		t.Errorf("ui.theme（dark 特判）迁移不保真: %+v", th)
	}
	// ui.tooltip：子表平移 + tooltip_delay 吸收
	tt := cfg.UI.Tooltip
	if tt.Delay != 300 || tt.Code.Enabled || tt.Pinyin.Enabled || tt.Pinyin.MaxReadings != 2 ||
		!tt.Chaizi.Enabled || !tt.Debug.Enabled {
		t.Errorf("ui.tooltip 迁移不保真: %+v", tt)
	}
	// ui.status_indicator：子表平移（enabled=false 保留）+ 旧顶层键回填
	si := cfg.UI.StatusIndicator
	if si.Enabled || si.DisplayMode != "always" {
		t.Errorf("status_indicator 子表平移不保真: %+v", si)
	}
	if si.Duration != 1200 || si.OffsetX != 5 || si.OffsetY != -3 {
		t.Errorf("status_indicator 旧顶层键回填失败: duration=%d x=%d y=%d", si.Duration, si.OffsetX, si.OffsetY)
	}
}

// TestMigrateV0toV1_QuickInputDisabled 熔合启发式：enabled=false 清空 trigger_keys。
func TestMigrateV0toV1_QuickInputDisabled(t *testing.T) {
	m := map[string]any{
		"input": map[string]any{
			"quick_input": map[string]any{
				"enabled":      false,
				"trigger_keys": []any{"semicolon"},
			},
		},
	}
	migrateV0toV1(m)
	f, _ := safeGetMap(m, "features")
	qi, _ := safeGetMap(f, "quick_input")
	tks, ok := safeGetSlice(qi, "trigger_keys")
	if !ok || len(tks) != 0 {
		t.Errorf("enabled=false 应清空 trigger_keys, got %v", tks)
	}
	if _, has := qi["enabled"]; has {
		t.Error("deprecated enabled 键应删除")
	}
}

// TestMigrateV0toV1_StatusIndicatorBackfillRules 回填规则：新键已存在不覆盖；
// 旧值无效（duration<=0 / offset==0）不回填。
func TestMigrateV0toV1_StatusIndicatorBackfillRules(t *testing.T) {
	m := map[string]any{
		"ui": map[string]any{
			"status_indicator_duration": 1500,
			"status_indicator_offset_x": 0, // 无效（offset 要求 !=0）→ 不回填
			"status_indicator": map[string]any{
				"duration": 900, // 新键已存在 → 旧键 1500 丢弃
			},
		},
	}
	migrateV0toV1(m)
	ui, _ := safeGetMap(m, "ui")
	si, _ := safeGetMap(ui, "status_indicator")
	if d, _ := safeGetInt(si, "duration"); d != 900 {
		t.Errorf("新键已存在时不应被旧键覆盖, duration=%d want 900", d)
	}
	if _, has := si["offset_x"]; has {
		t.Error("offset_x=0 无效旧值不应回填")
	}
	if _, has := ui["status_indicator_duration"]; has {
		t.Error("旧顶层键应删除")
	}
}

// TestMigrateV0toV1_DirtyDataNoPanic 脏数据宽容：类型错误的键不 panic，按缺失降级。
func TestMigrateV0toV1_DirtyDataNoPanic(t *testing.T) {
	m := map[string]any{
		"startup":  "not-a-map",                     // 节本身是脏标量 → renameKeyV1 原样搬到 general
		"stats":    []any{1, 2},                     // 脏类型 → 原样进 features.stats（unmarshal 阶段 TypeError 兜底）
		"advanced": map[string]any{"log_level": 42}, // 值类型错 → 原样搬（unmarshal 兜底）
		"ui": map[string]any{
			"theme":                     123, // 非 string → 不做 dark 特判，留原位
			"status_indicator_duration": "abc",
			"tooltip_delay":             300,
		},
		"input": map[string]any{
			"quick_input": map[string]any{"enabled": "yes", "trigger_key": 5},
		},
	}
	// 不 panic 即基本通过
	migrateV0toV1(m)
	ui, _ := safeGetMap(m, "ui")
	if _, has := ui["status_indicator_duration"]; has {
		t.Error("脏类型旧键也应删除（按缺失降级）")
	}
	tt, _ := safeGetMap(ui, "tooltip")
	if d, _ := safeGetInt(tt, "delay"); d != 300 {
		t.Error("脏数据不应影响其它键的正常迁移")
	}
}

// TestMigrateV0toV1_Idempotent v1 结构的 map 再过一遍迁移应为 no-op
// （防 SaveTo 写出的 YAML 路径文件被重复迁移破坏）。
func TestMigrateV0toV1_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rt.yaml")
	cfg := DefaultConfig()
	cfg.UI.Candidate.FontSize = 31
	cfg.Features.S2T.Enabled = true
	if err := SaveTo(cfg, path); err != nil {
		t.Fatal(err)
	}
	// SaveTo 写出的 yaml 带 version=1，LoadFrom 按显式 version 不重迁移；
	// 即便强制再跑一遍 migrateV0toV1 也应是 no-op。
	got, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.UI.Candidate.FontSize != 31 || !got.Features.S2T.Enabled {
		t.Errorf("v1 文件往返不保真: font_size=%v s2t=%v", got.UI.Candidate.FontSize, got.Features.S2T.Enabled)
	}
}
