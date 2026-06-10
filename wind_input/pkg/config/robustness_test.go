package config

import (
	"os"
	"path/filepath"
	"testing"
)

// robustness_test.go — 配置加载健壮性回归守护：
// 用户配置中某标量字段被写成不兼容的 YAML 结构（如序列）时，LoadFrom 必须：
//  (a) 不返回错误（降级而非崩溃）；
//  (b) 保留其余合法字段的用户值；
//  (c) 备份原始坏文件到 path+".bak"；
//  (d) 自愈回写后，第二次 LoadFrom 能干净加载、不再触发任何降级。
// 防止「坏配置永久降级、输入法完全不可用、设置存不了」的问题复发。

func TestLoadFrom_SelfHealsIncompatibleField(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")

	// 大部分合法：ui.theme 为合法字符串（用户值，区别于默认 "default"）；
	// 但 ui.font_size 本应是标量整数，这里被写成 YAML 序列 → *yaml.TypeError（部分解码）。
	content := "" +
		"ui:\n" +
		"  theme: robust-custom-theme\n" +
		"  font_size:\n" +
		"    - 1\n" +
		"    - 2\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// (a) 不返回错误
	cfg, err := LoadFrom(p)
	if err != nil {
		t.Fatalf("LoadFrom 应自愈降级而非报错, got err: %v", err)
	}

	// (b) 合法字段保留用户值
	if cfg.UI.Theme.Name != "robust-custom-theme" {
		t.Errorf("合法字段未保留用户值: theme want %q got %q", "robust-custom-theme", cfg.UI.Theme.Name)
	}

	// (c) 备份文件存在
	if _, statErr := os.Stat(p + ".bak"); statErr != nil {
		t.Errorf("坏配置应被备份到 %q, stat err: %v", p+".bak", statErr)
	}

	// 自愈后磁盘文件内容应不再含不兼容结构（font_size 序列已被剔除）。
	healed, readErr := os.ReadFile(p)
	if readErr != nil {
		t.Fatalf("读取自愈后的配置失败: %v", readErr)
	}
	if len(healed) == 0 {
		t.Fatal("自愈回写后配置文件为空")
	}

	// (d) 第二次 LoadFrom 不再降级：原始坏文件已被自愈，再次加载应干净无错，
	//     且用户值仍保留。这里通过「不会再次生成 .bak（删除后不再出现）」与无错共同验证。
	if rmErr := os.Remove(p + ".bak"); rmErr != nil {
		t.Fatalf("删除首次备份失败: %v", rmErr)
	}
	cfg2, err2 := LoadFrom(p)
	if err2 != nil {
		t.Fatalf("第二次 LoadFrom 应干净加载, got err: %v", err2)
	}
	if cfg2.UI.Theme.Name != "robust-custom-theme" {
		t.Errorf("自愈后用户值未保留: theme want %q got %q", "robust-custom-theme", cfg2.UI.Theme.Name)
	}
	if _, statErr := os.Stat(p + ".bak"); statErr == nil {
		t.Error("文件已自愈，第二次 LoadFrom 不应再次触发降级与备份")
	}
}
