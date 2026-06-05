package theme

import (
	"os"
	"path/filepath"
	"testing"
)

// capability_test.go — 能力声明矩阵守护。
//   - TestCapabilities_WellFormed：状态值合法、能力键/view 名在白名单、无重复 view（防 typo）。
//   - TestCapabilitiesJSON：导出 JSON 落盘 docs/design/theme-capabilities.json 并守护不漂移
//     （UPDATE_GOLDEN=1 重落）。前端编辑器消费此 JSON。

func TestCapabilities_WellFormed(t *testing.T) {
	validStatus := map[CapabilityStatus]bool{CapSupported: true, CapReserved: true, CapUnsupported: true}
	seen := map[string]bool{}
	for _, vc := range ThemeCapabilities {
		if !viewSubjects[vc.View] {
			t.Errorf("未知 view 主体 %q（不在 viewSubjects 白名单）", vc.View)
		}
		if seen[vc.View] {
			t.Errorf("view %q 重复声明", vc.View)
		}
		seen[vc.View] = true
		if len(vc.Caps) == 0 {
			t.Errorf("view %q 无任何能力声明", vc.View)
		}
		for key, st := range vc.Caps {
			if !capabilityKeys[key] {
				t.Errorf("view %q：未知能力键 %q（不在 capabilityKeys 白名单）", vc.View, key)
			}
			if !validStatus[st] {
				t.Errorf("view %q 能力 %q：非法状态值 %q", vc.View, key, st)
			}
		}
	}
	// 每个白名单 view 都应被声明（统一标准要求全覆盖）。
	for v := range viewSubjects {
		if !seen[v] {
			t.Errorf("view 主体 %q 在白名单但未声明能力", v)
		}
	}
}

func TestCapabilitiesJSON(t *testing.T) {
	got, err := MarshalCapabilities()
	if err != nil {
		t.Fatalf("MarshalCapabilities: %v", err)
	}
	// 导出路径：仓根 docs/design/theme-capabilities.json（测试 CWD = wind_input/pkg/theme）。
	goldenPath := filepath.Join("..", "..", "..", "docs", "design", "theme-capabilities.json")
	gotStr := string(got) + "\n" // 文件末尾换行
	compareOrWriteGolden(t, goldenPath, gotStr)
}

// 确保 os 在非 UPDATE_GOLDEN 路径也被引用（compareOrWriteGolden 在 v3golden_test.go）。
var _ = os.Getenv
