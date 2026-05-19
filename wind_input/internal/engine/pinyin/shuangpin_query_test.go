package pinyin

import (
	"testing"

	"github.com/huanfeng/wind_input/internal/engine/pinyin/shuangpin"
)

// TestIsShuangpinFinalKey 验证 IsShuangpinFinalKey 在全拼和各双拼方案下的行为
func TestIsShuangpinFinalKey(t *testing.T) {
	// 全拼引擎：始终返回 false
	engFull := NewEngine(nil, nil)
	for _, key := range []byte{';', '\'', ',', '.', 'a', 'z'} {
		if engFull.IsShuangpinFinalKey(key) {
			t.Errorf("全拼模式下 IsShuangpinFinalKey('%c') 应返回 false", key)
		}
	}

	// 微软双拼：; → ing（FinalMap 中有 ';'）
	mspyScheme := shuangpin.Get("mspy")
	if mspyScheme == nil {
		t.Fatal("未找到 mspy 方案，请检查 schemes_builtin.go 的 init()")
	}
	engMSPY := NewEngine(nil, nil)
	engMSPY.SetShuangpinConverter(shuangpin.NewConverter(mspyScheme))

	semicolonCases := []struct {
		key  byte
		want bool
		desc string
	}{
		{';', true, "mspy: ; 是韵母键（ing）"},
		{'a', true, "mspy: a 是韵母键（a）"},
		{'z', true, "mspy: z 是韵母键（ei）"},
		{'1', false, "mspy: 1 不是韵母键"},
		{'!', false, "mspy: ! 不是韵母键"},
	}
	for _, tc := range semicolonCases {
		got := engMSPY.IsShuangpinFinalKey(tc.key)
		if got != tc.want {
			t.Errorf("mspy IsShuangpinFinalKey('%c'): got %v, want %v (%s)", tc.key, got, tc.want, tc.desc)
		}
	}

	// 搜狗双拼：; → ing
	sogouScheme := shuangpin.Get("sogou")
	if sogouScheme == nil {
		t.Fatal("未找到 sogou 方案")
	}
	engSogou := NewEngine(nil, nil)
	engSogou.SetShuangpinConverter(shuangpin.NewConverter(sogouScheme))
	if !engSogou.IsShuangpinFinalKey(';') {
		t.Error("sogou: IsShuangpinFinalKey(';') 应返回 true（ing）")
	}

	// 紫光双拼：; → ing
	ziguangScheme := shuangpin.Get("ziguang")
	if ziguangScheme == nil {
		t.Fatal("未找到 ziguang 方案")
	}
	engZiguang := NewEngine(nil, nil)
	engZiguang.SetShuangpinConverter(shuangpin.NewConverter(ziguangScheme))
	if !engZiguang.IsShuangpinFinalKey(';') {
		t.Error("ziguang: IsShuangpinFinalKey(';') 应返回 true（ing）")
	}

	// 小鹤双拼：; 不在 FinalMap 中
	xiaoheScheme := shuangpin.Get("xiaohe")
	if xiaoheScheme == nil {
		t.Fatal("未找到 xiaohe 方案")
	}
	engXiaohe := NewEngine(nil, nil)
	engXiaohe.SetShuangpinConverter(shuangpin.NewConverter(xiaoheScheme))
	if engXiaohe.IsShuangpinFinalKey(';') {
		t.Error("xiaohe: IsShuangpinFinalKey(';') 应返回 false（小鹤无此映射）")
	}
}
