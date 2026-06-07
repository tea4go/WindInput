package rpc

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/huanfeng/wind_input/pkg/config"
	"github.com/huanfeng/wind_input/pkg/rpcapi"
)

// ── mockConfigReloader ──────────────────────────────────────────────────────

type mockConfigReloader struct{}

func (m *mockConfigReloader) ReloadConfig() error { return nil }
func (m *mockConfigReloader) ApplyConfigUpdate(oldCfg, newCfg *config.Config, _ map[string]bool) (bool, error) {
	*oldCfg = *newCfg
	return false, nil
}
func (m *mockConfigReloader) RebuildDictCache() (int, error) { return 0, nil }

// newTestConfigService 构建带 no-op saveFn 的 ConfigService，供各测试复用。
func newTestConfigService(cfg *config.Config) *ConfigService {
	return &ConfigService{
		cfgMu:          new(sync.RWMutex),
		cfg:            cfg,
		configReloader: &mockConfigReloader{},
		broadcaster:    NewEventBroadcaster(nil),
		saveFn:         func(*config.Config) error { return nil },
	}
}

// ── resolveKeyPath ──────────────────────────────────────────────────────────

func TestResolveKeyPath_Valid(t *testing.T) {
	cases := []struct {
		key     string
		section string
		path    []string
	}{
		{"ui.font_size", "ui", []string{"font_size"}},
		{"input.auto_pair.chinese", "input", []string{"auto_pair", "chinese"}},
		{"advanced.log_level", "advanced", []string{"log_level"}},
	}
	for _, tc := range cases {
		sec, path, err := resolveKeyPath(tc.key)
		if err != nil {
			t.Errorf("%q: unexpected error: %v", tc.key, err)
			continue
		}
		if sec != tc.section {
			t.Errorf("%q: section want %q got %q", tc.key, tc.section, sec)
		}
		if len(path) != len(tc.path) {
			t.Errorf("%q: path len want %d got %d", tc.key, len(tc.path), len(path))
			continue
		}
		for i := range tc.path {
			if path[i] != tc.path[i] {
				t.Errorf("%q: path[%d] want %q got %q", tc.key, i, tc.path[i], path[i])
			}
		}
	}
}

func TestResolveKeyPath_Invalid(t *testing.T) {
	cases := []string{"noDot", ".bad", "bad.", ""}
	for _, key := range cases {
		_, _, err := resolveKeyPath(key)
		if err == nil {
			t.Errorf("%q: expected error, got nil", key)
		}
	}
}

// ── getSectionMap / setSectionFromMap ───────────────────────────────────────

func TestGetSectionMap_AllSections(t *testing.T) {
	cfg := config.SystemDefaultConfig()
	sections := []string{"startup", "schema", "hotkeys", "ui", "toolbar", "input", "advanced", "stats"}
	for _, sec := range sections {
		m, err := getSectionMap(cfg, sec)
		if err != nil {
			t.Errorf("section %q: %v", sec, err)
			continue
		}
		if m == nil {
			t.Errorf("section %q: got nil map", sec)
		}
	}
}

func TestGetSectionMap_Unknown(t *testing.T) {
	cfg := config.SystemDefaultConfig()
	_, err := getSectionMap(cfg, "nonexistent")
	if err == nil {
		t.Error("expected error for unknown section")
	}
}

func TestSetSectionRoundTrip(t *testing.T) {
	cfg := config.SystemDefaultConfig()
	m, _ := getSectionMap(cfg, "ui")
	m["font_size"] = float64(18)
	if err := setSectionFromMap(cfg, "ui", m); err != nil {
		t.Fatalf("setSectionFromMap: %v", err)
	}
	if cfg.UI.FontSize != 18 {
		t.Errorf("expected font_size=18, got %v", cfg.UI.FontSize)
	}
}

// ── getNestedKey / setNestedKey ─────────────────────────────────────────────

func TestGetNestedKey_Flat(t *testing.T) {
	m := map[string]any{"key": "value"}
	v, err := getNestedKey(m, []string{"key"})
	if err != nil || v != "value" {
		t.Errorf("expected 'value', got %v, err=%v", v, err)
	}
}

func TestGetNestedKey_Nested(t *testing.T) {
	m := map[string]any{"a": map[string]any{"b": float64(42)}}
	v, err := getNestedKey(m, []string{"a", "b"})
	if err != nil || v != float64(42) {
		t.Errorf("expected 42, got %v, err=%v", v, err)
	}
}

func TestGetNestedKey_Missing(t *testing.T) {
	m := map[string]any{"a": map[string]any{}}
	_, err := getNestedKey(m, []string{"a", "missing"})
	if err == nil {
		t.Error("expected error for missing key")
	}
}

func TestSetNestedKey_Flat(t *testing.T) {
	m := map[string]any{}
	setNestedKey(m, []string{"x"}, 99)
	if m["x"] != 99 {
		t.Errorf("expected 99, got %v", m["x"])
	}
}

func TestSetNestedKey_DeepNested(t *testing.T) {
	m := map[string]any{}
	setNestedKey(m, []string{"a", "b", "c"}, "deep")
	v, err := getNestedKey(m, []string{"a", "b", "c"})
	if err != nil || v != "deep" {
		t.Errorf("expected 'deep', got %v, err=%v", v, err)
	}
}

func TestSetNestedKey_Overwrite(t *testing.T) {
	m := map[string]any{"a": map[string]any{"b": "old"}}
	setNestedKey(m, []string{"a", "b"}, "new")
	v, _ := getNestedKey(m, []string{"a", "b"})
	if v != "new" {
		t.Errorf("expected 'new', got %v", v)
	}
}

// ── diffSections ────────────────────────────────────────────────────────────

func TestDiffSections_NoChange(t *testing.T) {
	cfg := config.SystemDefaultConfig()
	cfg2 := config.SystemDefaultConfig()
	diff := diffSections(cfg, cfg2)
	if len(diff) != 0 {
		t.Errorf("expected no diff, got %v", diff)
	}
}

func TestDiffSections_UIChanged(t *testing.T) {
	cfg := config.SystemDefaultConfig()
	cfg2 := config.SystemDefaultConfig()
	cfg2.UI.FontSize = cfg.UI.FontSize + 4
	diff := diffSections(cfg, cfg2)
	if !diff["ui"] {
		t.Error("expected ui section to be in diff")
	}
	if diff["input"] || diff["hotkeys"] {
		t.Errorf("unexpected sections in diff: %v", diff)
	}
}

func TestDiffSections_MultipleSections(t *testing.T) {
	cfg := config.SystemDefaultConfig()
	cfg2 := config.SystemDefaultConfig()
	cfg2.UI.FontSize++
	cfg2.Input.FilterMode = "none"
	diff := diffSections(cfg, cfg2)
	if !diff["ui"] || !diff["input"] {
		t.Errorf("expected ui+input in diff, got %v", diff)
	}
}

// ── ConfigService: GetAll / Get / GetDefaults ───────────────────────────────

func TestConfigGetAll(t *testing.T) {
	cfg := config.SystemDefaultConfig()
	svc := newTestConfigService(cfg)

	var reply rpcapi.ConfigGetAllReply
	if err := svc.GetAll(&rpcapi.Empty{}, &reply); err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(reply.Config) == 0 {
		t.Error("expected non-empty config JSON")
	}
	var m map[string]any
	if err := json.Unmarshal(reply.Config, &m); err != nil {
		t.Fatalf("invalid JSON from GetAll: %v", err)
	}
	if _, ok := m["ui"]; !ok {
		t.Error("expected 'ui' key in config")
	}
}

func TestConfigGet_ValidKey(t *testing.T) {
	cfg := config.SystemDefaultConfig()
	cfg.UI.FontSize = 16
	svc := newTestConfigService(cfg)

	var reply rpcapi.ConfigGetReply
	if err := svc.Get(&rpcapi.ConfigGetArgs{Keys: []string{"ui.font_size"}}, &reply); err != nil {
		t.Fatalf("Get: %v", err)
	}
	v, ok := reply.Values["ui.font_size"]
	if !ok {
		t.Fatal("key ui.font_size not in reply")
	}
	if v != float64(16) {
		t.Errorf("expected 16, got %v", v)
	}
}

func TestConfigGet_UnknownSection(t *testing.T) {
	cfg := config.SystemDefaultConfig()
	svc := newTestConfigService(cfg)

	var reply rpcapi.ConfigGetReply
	err := svc.Get(&rpcapi.ConfigGetArgs{Keys: []string{"unknown.field"}}, &reply)
	if err == nil {
		t.Error("expected error for unknown section")
	}
}

func TestConfigGet_InvalidKeyFormat(t *testing.T) {
	cfg := config.SystemDefaultConfig()
	svc := newTestConfigService(cfg)

	var reply rpcapi.ConfigGetReply
	err := svc.Get(&rpcapi.ConfigGetArgs{Keys: []string{"noDot"}}, &reply)
	if err == nil {
		t.Error("expected error for key without dot")
	}
}

func TestConfigGetDefaults(t *testing.T) {
	cfg := config.SystemDefaultConfig()
	svc := newTestConfigService(cfg)

	var reply rpcapi.ConfigGetDefaultsReply
	if err := svc.GetDefaults(&rpcapi.Empty{}, &reply); err != nil {
		t.Fatalf("GetDefaults: %v", err)
	}
	if len(reply.Config) == 0 {
		t.Error("expected non-empty defaults JSON")
	}
}

// ── ConfigService: Set / SetAll / Reset ────────────────────────────────────

func TestConfigSet_SingleKey(t *testing.T) {
	cfg := config.SystemDefaultConfig()
	cfg.UI.FontSize = 12
	svc := newTestConfigService(cfg)

	var reply rpcapi.ConfigSetReply
	err := svc.Set(&rpcapi.ConfigSetArgs{
		Items: []rpcapi.ConfigSetItem{{Key: "ui.font_size", Value: float64(20)}},
	}, &reply)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if len(reply.Applied) != 1 || reply.Applied[0] != "ui.font_size" {
		t.Errorf("unexpected Applied: %v", reply.Applied)
	}
	if cfg.UI.FontSize != 20 {
		t.Errorf("expected font_size=20 in live config, got %v", cfg.UI.FontSize)
	}
}

func TestConfigSet_MultipleKeys(t *testing.T) {
	cfg := config.SystemDefaultConfig()
	svc := newTestConfigService(cfg)

	var reply rpcapi.ConfigSetReply
	err := svc.Set(&rpcapi.ConfigSetArgs{
		Items: []rpcapi.ConfigSetItem{
			{Key: "ui.font_size", Value: float64(14)},
			{Key: "toolbar.visible", Value: true},
		},
	}, &reply)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if len(reply.Applied) != 2 {
		t.Errorf("expected 2 applied, got %d", len(reply.Applied))
	}
}

func TestConfigSet_InvalidKey(t *testing.T) {
	cfg := config.SystemDefaultConfig()
	svc := newTestConfigService(cfg)

	var reply rpcapi.ConfigSetReply
	err := svc.Set(&rpcapi.ConfigSetArgs{
		Items: []rpcapi.ConfigSetItem{{Key: "badkey", Value: 1}},
	}, &reply)
	if err == nil {
		t.Error("expected error for invalid key")
	}
}

func TestConfigSetAll(t *testing.T) {
	cfg := config.SystemDefaultConfig()
	svc := newTestConfigService(cfg)

	newCfg := config.SystemDefaultConfig()
	newCfg.UI.FontSize = 22
	data, _ := json.Marshal(newCfg)

	var reply rpcapi.ConfigSetAllReply
	if err := svc.SetAll(&rpcapi.ConfigSetAllArgs{Config: data}, &reply); err != nil {
		t.Fatalf("SetAll: %v", err)
	}
	if cfg.UI.FontSize != 22 {
		t.Errorf("expected font_size=22 after SetAll, got %v", cfg.UI.FontSize)
	}
}

// TestConfigSetAll_PreservesStats 防回归：全局保存（SetAll）提交的 formData 不含 stats 字段，
// 反序列化后 stats 的 *bool 为 nil（JSON null）。SetAll 必须保留服务端现有 stats，
// 否则会把用户在统计页关闭的 track_english=false 冲回默认 true。
func TestConfigSetAll_PreservesStats(t *testing.T) {
	cfg := config.SystemDefaultConfig()
	enabled := true
	track := false
	cfg.Stats.Enabled = &enabled
	cfg.Stats.TrackEnglish = &track // 用户在统计页关闭了英文统计
	svc := newTestConfigService(cfg)

	// 模拟前端全局保存：formData 不含 stats，序列化到 config.Config 时 Stats 的 *bool 为 nil
	frontendCfg := config.SystemDefaultConfig()
	frontendCfg.Stats = config.StatsConfig{} // Enabled/TrackEnglish = nil → JSON null
	frontendCfg.UI.FontSize = 22             // 同时修改一个全局表单字段
	data, _ := json.Marshal(frontendCfg)

	var reply rpcapi.ConfigSetAllReply
	if err := svc.SetAll(&rpcapi.ConfigSetAllArgs{Config: data}, &reply); err != nil {
		t.Fatalf("SetAll: %v", err)
	}

	if cfg.UI.FontSize != 22 {
		t.Errorf("全局表单字段未生效：font_size want 22, got %v", cfg.UI.FontSize)
	}
	if cfg.Stats.IsTrackEnglish() != false {
		t.Errorf("track_english 被全局保存覆盖：want false, got %v", cfg.Stats.IsTrackEnglish())
	}
	if cfg.Stats.IsEnabled() != true {
		t.Errorf("stats.enabled 被全局保存覆盖：want true, got %v", cfg.Stats.IsEnabled())
	}
}

func TestConfigReset_ToDefault(t *testing.T) {
	def := config.SystemDefaultConfig()
	cfg := config.SystemDefaultConfig()
	cfg.UI.FontSize = 99
	svc := newTestConfigService(cfg)

	var reply rpcapi.ConfigResetReply
	if err := svc.Reset(&rpcapi.ConfigResetArgs{Keys: []string{"ui.font_size"}}, &reply); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if cfg.UI.FontSize != def.UI.FontSize {
		t.Errorf("expected font_size reset to %v, got %v", def.UI.FontSize, cfg.UI.FontSize)
	}
}

func TestConfigSetActiveSchema(t *testing.T) {
	cfg := config.SystemDefaultConfig()
	cfg.Schema.Available = []string{"wubi", "pinyin"}
	cfg.Schema.Active = "wubi"
	svc := newTestConfigService(cfg)

	var empty rpcapi.Empty
	if err := svc.SetActiveSchema(&rpcapi.SetActiveSchemaArgs{SchemaID: "pinyin"}, &empty); err != nil {
		t.Fatalf("SetActiveSchema: %v", err)
	}
	if cfg.Schema.Active != "pinyin" {
		t.Errorf("expected active=pinyin, got %q", cfg.Schema.Active)
	}
}

// TestConfigSet_StatsTrackEnglish 验证 stats 配置改走通用 Config.Set（按 key）后，
// *bool 字段经 setNestedKey/setSectionFromMap 往返正确：设 false 得 false，
// 且未改的其它 stats 字段（enabled）保留默认。这是 stats 并入全局保存的核心保证。
func TestConfigSet_StatsTrackEnglish(t *testing.T) {
	cfg := config.SystemDefaultConfig() // 默认 track_english/enabled 均为 nil→true 语义
	svc := newTestConfigService(cfg)

	var reply rpcapi.ConfigSetReply
	if err := svc.Set(&rpcapi.ConfigSetArgs{
		Items: []rpcapi.ConfigSetItem{{Key: "stats.track_english", Value: false}},
	}, &reply); err != nil {
		t.Fatalf("Set: %v", err)
	}

	if cfg.Stats.IsTrackEnglish() != false {
		t.Errorf("track_english 应为 false，got %v", cfg.Stats.IsTrackEnglish())
	}
	if cfg.Stats.IsEnabled() != true {
		t.Errorf("未改的 enabled 应保留默认 true，got %v", cfg.Stats.IsEnabled())
	}
}
