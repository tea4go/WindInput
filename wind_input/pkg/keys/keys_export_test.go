package keys

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateKeys = flag.Bool("update", false, "写出 keys.json 供设置前端 enums 一致性校验")

// TestExportKeys 在 -update 标志下把规范按键 token / 修饰键 / 别名映射写到设置前端的
// generated/keys.json，供前端 enums.ts 的一致性测试断言（前端按键 token 的值必须 ∈
// 规范清单）。用法：go test ./pkg/keys/ -run TestExportKeys -update
func TestExportKeys(t *testing.T) {
	if !*updateKeys {
		t.Skip("仅在 -update 时写出（CI 校验由前端测试消费已生成文件）")
	}

	keyList := CanonicalKeys()
	keyStrs := make([]string, len(keyList))
	for i, k := range keyList {
		keyStrs[i] = string(k)
	}

	modList := CanonicalModifiers()
	modStrs := make([]string, len(modList))
	for i, m := range modList {
		modStrs[i] = string(m)
	}

	aliasMap := Aliases()
	aliasStrs := make(map[string]string, len(aliasMap))
	for k, v := range aliasMap {
		aliasStrs[k] = string(v)
	}

	payload := map[string]any{
		"keys":      keyStrs,
		"modifiers": modStrs,
		"aliases":   aliasStrs,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	out := filepath.Join("..", "..", "..", "wind_setting", "frontend", "src", "generated", "keys.json")
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("已写出 %d 个规范按键、%d 个修饰键、%d 条别名到 %s", len(keyStrs), len(modStrs), len(aliasStrs), out)
}
