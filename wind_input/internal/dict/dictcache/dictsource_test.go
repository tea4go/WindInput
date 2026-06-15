package dictcache

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// import_tables 从 rime YAML 头正确解析。
func TestImportTablesParity(t *testing.T) {
	dir := t.TempDir()
	yamlMain := filepath.Join(dir, "main.dict.yaml")
	yamlContent := "---\nname: main\nimport_tables:\n  - cn_dicts/8105\n  - others\n...\n"
	if err := os.WriteFile(yamlMain, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	wantImports := []string{"cn_dicts/8105", "others"}
	if got := discoverRimeCodetableImports(yamlMain); !reflect.DeepEqual(got, wantImports) {
		t.Fatalf("yaml import_tables 解析异常: %+v", got)
	}
}
