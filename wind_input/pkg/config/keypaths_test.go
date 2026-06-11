package config

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateKeyPaths = flag.Bool("update", false, "写出 config-keys.json 供前端 key 一致性校验")

// TestAllKeyPaths_Sanity 抽样断言 v1 关键路径存在、v0 旧路径不存在。
func TestAllKeyPaths_Sanity(t *testing.T) {
	paths := AllKeyPaths()
	set := make(map[string]bool, len(paths))
	for _, p := range paths {
		set[p] = true
	}

	for _, want := range []string{
		"general.remember_last_state",
		"ui.candidate.font_size",
		"ui.candidate.per_page",
		"ui.font.render_mode",
		"ui.theme.name",
		"ui.toolbar.hide_in_fullscreen",
		"ui.tooltip.delay",
		"features.stats.enabled",
		"features.s2t.variant",
		"features.cmdbar.candidate_prefix",
		"features.quick_input.accent_color",
		"input.temp_pinyin.accent_color",
		"input.capslock.cancel_on_mode_switch",
		"compat.host_render_processes",
		"debug.log_level",
	} {
		if !set[want] {
			t.Errorf("v1 关键路径缺失: %s", want)
		}
	}
	for _, gone := range []string{
		"startup.remember_last_state",
		"ui.font_size",
		"ui.candidates_per_page",
		"ui.theme_style",
		"toolbar.visible",
		"stats.enabled",
		"s2t.enabled",
		"advanced.log_level",
		"input.quick_input.trigger_keys",
	} {
		if set[gone] {
			t.Errorf("v0 旧路径不应存在: %s", gone)
		}
	}
}

// TestExportKeyPaths 在 -update 标志下把全量 key 清单写到前端 generated 目录，
// 供 wind_setting 前端测试做 schema/搜索索引 key ∈ 清单 的一致性断言。
// 用法：go test ./pkg/config/ -run TestExportKeyPaths -update
func TestExportKeyPaths(t *testing.T) {
	if !*updateKeyPaths {
		t.Skip("仅在 -update 时写出（CI 校验由前端测试消费已生成文件）")
	}
	out := filepath.Join("..", "..", "..", "wind_setting", "frontend", "src", "generated", "config-keys.json")
	data, err := json.MarshalIndent(AllKeyPaths(), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("已写出 %d 个 key 到 %s", len(AllKeyPaths()), out)

	// 同时写出 Go 侧命名常量包 configkey，供 Go 调用方以编译期可校验的常量
	// 替代裸字符串键路径。
	constOut := filepath.Join("configkey", "keys_gen.go")
	if err := os.MkdirAll(filepath.Dir(constOut), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(constOut, []byte(GenerateConfigKeyConsts()), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("已写出 configkey 常量到 %s", constOut)
}

// TestFieldsKeysAreValid 守卫：accessor.Fields 注册表里的每个键路径都必须 ∈ AllKeyPaths()
// （即对应 Config 结构体真实存在的 yaml tag）。重构 struct tag 后若 Fields 未同步，
// 本测试立即失败——防止 config.get/set/toggle 操作一个不存在的键路径（历史 bug 根因）。
func TestFieldsKeysAreValid(t *testing.T) {
	valid := make(map[string]bool)
	for _, p := range AllKeyPaths() {
		valid[p] = true
	}
	for key := range Fields {
		if !valid[key] {
			t.Errorf("accessor.Fields 含非法键路径 %q：在 Config yaml tag 中不存在（struct 改名后未同步？）", key)
		}
	}
}
