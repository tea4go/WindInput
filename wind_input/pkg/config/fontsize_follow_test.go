package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFontSizeFollowTheme_DefaultAndLoad 验证字号跟随主题的默认值与读取：
// 走通用「缺失=继承默认」机制（无特例探针）——默认 true，缺失字段继承默认 true，显式值按写读。
func TestFontSizeFollowTheme_DefaultAndLoad(t *testing.T) {
	if !DefaultConfig().UI.FontSizeFollowTheme {
		t.Error("DefaultConfig 应默认 FontSizeFollowTheme=true（新装跟随主题）")
	}

	dir := t.TempDir()

	// 配置存在但未写 font_size_follow_theme → 走通用 merge-on-default，继承默认 true。
	// （原探针迁移已移除：不再把"缺失"特判为 false。）
	absentPath := filepath.Join(dir, "absent.yaml")
	if err := os.WriteFile(absentPath, []byte("ui:\n  font_size: 20\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(absentPath)
	if err != nil {
		t.Fatalf("LoadFrom absent: %v", err)
	}
	if !cfg.UI.FontSizeFollowTheme {
		t.Error("缺失字段应继承默认 true（通用 merge-on-default）")
	}
	if cfg.UI.FontSize != 20 {
		t.Errorf("font_size 应保留 20, got %v", cfg.UI.FontSize)
	}

	// 显式 false → 读到 false。
	falsePath := filepath.Join(dir, "false.yaml")
	if err := os.WriteFile(falsePath, []byte("ui:\n  font_size_follow_theme: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgF, err := LoadFrom(falsePath)
	if err != nil {
		t.Fatalf("LoadFrom false: %v", err)
	}
	if cfgF.UI.FontSizeFollowTheme {
		t.Error("显式 font_size_follow_theme: false 应读到 false")
	}
}

// TestFontSizeFollowTheme_SaveLoadRoundTrip 守护本字段的持久化闭环（回归测试）：
// SaveTo 走 diff-save，LoadFrom 走 merge-on-default；true 与 false 都必须 round-trip 一致。
// 历史 bug：默认 true 的 bool + omitempty + 探针迁移三者叠加，导致 true 存不下来（回读成 false）。
func TestFontSizeFollowTheme_SaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()

	for _, want := range []bool{true, false} {
		cfg := DefaultConfig()
		cfg.UI.FontSizeFollowTheme = want
		path := filepath.Join(dir, "rt.yaml")
		if err := SaveTo(cfg, path); err != nil {
			t.Fatalf("SaveTo(%v): %v", want, err)
		}
		got, err := LoadFrom(path)
		if err != nil {
			t.Fatalf("LoadFrom(%v): %v", want, err)
		}
		if got.UI.FontSizeFollowTheme != want {
			t.Errorf("round-trip FontSizeFollowTheme: 存 %v 回读 %v（持久化未闭环）", want, got.UI.FontSizeFollowTheme)
		}
	}
}
