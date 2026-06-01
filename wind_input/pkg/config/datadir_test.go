package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestReadDataDirConf_NotExist(t *testing.T) {
	dir := t.TempDir()
	got, err := readDataDirConf(filepath.Join(dir, "datadir.conf"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestReadDataDirConf_Empty(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "datadir.conf")
	os.WriteFile(confPath, []byte("  \n"), 0644)
	got, err := readDataDirConf(confPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestReadDataDirConf_ValidPath(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "datadir.conf")
	target := filepath.Join(dir, "mydata")
	os.WriteFile(confPath, []byte(target+"\n"), 0644)
	got, err := readDataDirConf(confPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != target {
		t.Fatalf("expected %q, got %q", target, got)
	}
}

func TestWriteDataDirConf(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "datadir.conf")
	target := filepath.Join(dir, "custom")
	err := writeDataDirConf(confPath, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := readDataDirConf(confPath)
	if got != target {
		t.Fatalf("expected %q, got %q", target, got)
	}
}

func TestValidateDataDirPath(t *testing.T) {
	// 绝对路径与非法字符示例随平台不同：Windows 用盘符路径，类 Unix 用 / 起始路径，
	// 否则 filepath.IsAbs 会把 Windows 路径在 macOS/Linux 上判为非绝对路径而误失败。
	validAbs := "D:\\MyData\\WindInput"
	illegalChars := "D:\\My*Data"
	if runtime.GOOS != "windows" {
		validAbs = "/tmp/MyData/WindInput"
		illegalChars = "/tmp/My*Data"
	}
	tests := []struct {
		name   string
		path   string
		wantOK bool
	}{
		{"empty", "", false},
		{"valid abs", validAbs, true},
		{"relative", "relative/path", false},
		{"illegal chars", illegalChars, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, _ := ValidateDataDirPath(tt.path)
			if ok != tt.wantOK {
				t.Errorf("ValidateDataDirPath(%q) = %v, want %v", tt.path, ok, tt.wantOK)
			}
		})
	}
}
