package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/huanfeng/wind_input/pkg/config"
	"github.com/huanfeng/wind_input/pkg/theme"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"gopkg.in/yaml.v3"
)

// ImportThemeResult 主题导入结果
type ImportThemeResult struct {
	Success   bool   `json:"success"`
	Cancelled bool   `json:"cancelled"`
	ThemeName string `json:"theme_name"`
	Conflict  bool   `json:"conflict"`
	ErrorMsg  string `json:"error_msg"`
}

// ImportThemeFromFile 打开系统文件选择对话框，读取并导入 yaml 主题文件。
// force=true 时覆盖同名主题。
func (a *App) ImportThemeFromFile(force bool) ImportThemeResult {
	path, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "选择主题文件",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "YAML 主题文件 (*.yaml)", Pattern: "*.yaml"},
		},
	})
	if err != nil {
		return ImportThemeResult{ErrorMsg: "打开文件对话框失败: " + err.Error()}
	}
	if path == "" {
		return ImportThemeResult{Cancelled: true}
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return ImportThemeResult{ErrorMsg: "读取文件失败: " + err.Error()}
	}

	return importThemeFromContent(content, force)
}

// ImportThemeFromText 校验并导入粘贴的 YAML 文本内容。
// force=true 时覆盖同名主题。
func (a *App) ImportThemeFromText(yamlContent string, force bool) ImportThemeResult {
	if strings.TrimSpace(yamlContent) == "" {
		return ImportThemeResult{ErrorMsg: "内容不能为空"}
	}
	return importThemeFromContent([]byte(yamlContent), force)
}

// importThemeFromContent 统一校验写入管线：解析 → 校验 → 冲突检测 → 写入。
func importThemeFromContent(content []byte, force bool) ImportThemeResult {
	// 1. 解析 YAML
	t := &theme.Theme{}
	if err := yaml.Unmarshal(content, t); err != nil {
		return ImportThemeResult{ErrorMsg: "YAML 格式错误: " + err.Error()}
	}

	// 2. meta.name 必填
	if t.Meta.Name == "" {
		return ImportThemeResult{ErrorMsg: "主题缺少 meta.name 字段"}
	}

	// 3. 写入临时目录，用 LightweightManager 全链校验（base 存在性 + token 引用完整性）
	tmpDir, err := os.MkdirTemp("", "windinput-theme-import-*")
	if err != nil {
		return ImportThemeResult{ErrorMsg: "创建临时目录失败"}
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "theme.yaml")
	if err := os.WriteFile(tmpFile, content, 0o644); err != nil {
		return ImportThemeResult{ErrorMsg: "写入临时文件失败"}
	}

	mgr := theme.NewLightweightManager(nil)
	if err := mgr.LoadTheme(tmpFile); err != nil {
		return ImportThemeResult{ErrorMsg: "主题校验失败: " + err.Error()}
	}
	if mgr.GetResolvedV3() == nil {
		return ImportThemeResult{ErrorMsg: "主题非 v3 格式或解析失败（缺少有效的 colors 块）"}
	}

	// 4. 计算目标目录，检测同名冲突
	slug := sanitizeThemeSlug(t.Meta.Name)
	userThemesDir, err := config.GetThemesUserDir()
	if err != nil {
		return ImportThemeResult{ErrorMsg: "获取用户主题目录失败: " + err.Error()}
	}

	destDir := filepath.Join(userThemesDir, slug)
	if _, err := os.Stat(destDir); err == nil && !force {
		return ImportThemeResult{
			ThemeName: t.Meta.Name,
			Conflict:  true,
			ErrorMsg:  fmt.Sprintf("已存在主题「%s」", t.Meta.Name),
		}
	}

	// 5. 写入用户主题目录
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return ImportThemeResult{ErrorMsg: "创建主题目录失败: " + err.Error()}
	}
	if err := os.WriteFile(filepath.Join(destDir, "theme.yaml"), content, 0o644); err != nil {
		return ImportThemeResult{ErrorMsg: "写入主题文件失败: " + err.Error()}
	}

	return ImportThemeResult{
		Success:   true,
		ThemeName: t.Meta.Name,
	}
}

// sanitizeThemeSlug 将 meta.name 转为合法的 Windows 目录名：
// 去除非法字符（\ / : * ? " < > |），空格替换为下划线，保留其余字符。
func sanitizeThemeSlug(name string) string {
	const illegal = `\/:*?"<>|`
	var sb strings.Builder
	for _, r := range name {
		if strings.ContainsRune(illegal, r) {
			continue
		}
		if r == ' ' {
			sb.WriteRune('_')
		} else {
			sb.WriteRune(r)
		}
	}
	result := strings.TrimSpace(sb.String())
	if result == "" {
		return "imported_theme"
	}
	return result
}
