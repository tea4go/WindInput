package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFontSizeFollowTheme_DefaultAndMigration 验证字号跟随主题的默认值与保守迁移：
// 新装默认跟随；老配置（无字段）迁移为自定义；显式写值则保留。
func TestFontSizeFollowTheme_DefaultAndMigration(t *testing.T) {
	if !DefaultConfig().UI.FontSizeFollowTheme {
		t.Error("DefaultConfig 应默认 FontSizeFollowTheme=true（新装跟随主题）")
	}

	dir := t.TempDir()

	// 老配置：存在但未写 font_size_follow_theme → 探针迁移为自定义 false，字号保留。
	oldPath := filepath.Join(dir, "old.yaml")
	if err := os.WriteFile(oldPath, []byte("ui:\n  font_size: 20\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(oldPath)
	if err != nil {
		t.Fatalf("LoadFrom old: %v", err)
	}
	if cfg.UI.FontSizeFollowTheme {
		t.Error("老配置（无 font_size_follow_theme）应迁移为自定义 false")
	}
	if cfg.UI.FontSize != 20 {
		t.Errorf("老配置 font_size 应保留 20, got %v", cfg.UI.FontSize)
	}

	// 显式写 true → 保留（不被迁移覆盖）。
	newPath := filepath.Join(dir, "new.yaml")
	if err := os.WriteFile(newPath, []byte("ui:\n  font_size: 18\n  font_size_follow_theme: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg2, err := LoadFrom(newPath)
	if err != nil {
		t.Fatalf("LoadFrom new: %v", err)
	}
	if !cfg2.UI.FontSizeFollowTheme {
		t.Error("显式 font_size_follow_theme: true 应保留")
	}
}
