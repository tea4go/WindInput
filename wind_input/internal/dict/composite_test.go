package dict

import (
	"slices"
	"testing"
)

// makeCodetable 构建内存码表：content 为 RIME 格式字符串。
func makeCodetable(t *testing.T, content string) *CodeTable {
	t.Helper()
	ct := NewCodeTable()
	if err := ct.parse(content); err != nil {
		t.Fatalf("parse codetable: %v", err)
	}
	return ct
}

// TestCompositeDict_LayerDeclarationOrder 验证声明靠前的词库（layer 0）候选
// 在无权重差异时整体排在声明靠后的词库（layer 1）之前。
//
// 测试数据使用显式 weight=0，模拟生产环境 wdb 无权重词条的存储值。
// ct.parse() 对无权重条目会合成 1000000-entryOrder 的递减权重，
// 两个词库各自从 0 计数会产生跨词库权重碰撞，与生产 wdb 行为不符，
// 因此测试数据须显式写 weight=0。
func TestCompositeDict_LayerDeclarationOrder(t *testing.T) {
	// xh：主词库，同码 3 个候选，weight=0
	xhContent := "[CodeTableHeader]\nname=xh\n[CodeTable]\nab\t主一\t0\nab\t主二\t0\nab\t主三\t0\n"
	// fl：附加词库，同码 2 个候选，weight=0
	flContent := "[CodeTableHeader]\nname=fl\n[CodeTable]\nab\t附一\t0\nab\t附二\t0\n"

	xhLayer := NewCodeTableLayer("xh", LayerTypeSystem, makeCodetable(t, xhContent))
	flLayer := NewCodeTableLayer("fl", LayerTypeSystem, makeCodetable(t, flContent))

	cd := NewCompositeDict()
	cd.AddLayer(xhLayer)
	cd.AddLayer(flLayer)

	results := cd.Search("ab", SearchOptions{})
	if len(results) == 0 {
		t.Fatal("Search 返回空结果")
	}

	texts := make([]string, len(results))
	for i, c := range results {
		texts[i] = c.Text
	}

	xhWords := []string{"主一", "主二", "主三"}
	flWords := []string{"附一", "附二"}

	// 找到最后一个 xh 候选的位置和第一个 fl 候选的位置
	lastXH := -1
	for _, w := range xhWords {
		idx := slices.Index(texts, w)
		if idx == -1 {
			t.Errorf("xh 词「%s」未出现在结果中，结果=%v", w, texts)
			continue
		}
		if idx > lastXH {
			lastXH = idx
		}
	}

	firstFL := len(texts)
	for _, w := range flWords {
		idx := slices.Index(texts, w)
		if idx == -1 {
			t.Errorf("fl 词「%s」未出现在结果中，结果=%v", w, texts)
			continue
		}
		if idx < firstFL {
			firstFL = idx
		}
	}

	if lastXH >= firstFL {
		t.Errorf("声明顺序错乱：xh 最后候选位置 %d >= fl 首个候选位置 %d，结果=%v",
			lastXH, firstFL, texts)
	}
}

// TestCompositeDict_LayerDeclarationOrder_WeightOverrides 验证当 fl 词库权重
// 显式高于 xh 时，权重仍优先于声明顺序（层偏移不破坏有意义的权重设置）。
func TestCompositeDict_LayerDeclarationOrder_WeightOverrides(t *testing.T) {
	xhContent := "[CodeTableHeader]\nname=xh\n[CodeTable]\nab\t主候选\t100\n"
	// fl 给出更高权重，应排在主候选之前
	flContent := "[CodeTableHeader]\nname=fl\n[CodeTable]\nab\t附高权重候选\t9999\n"

	xhLayer := NewCodeTableLayer("xh", LayerTypeSystem, makeCodetable(t, xhContent))
	flLayer := NewCodeTableLayer("fl", LayerTypeSystem, makeCodetable(t, flContent))

	cd := NewCompositeDict()
	cd.AddLayer(xhLayer)
	cd.AddLayer(flLayer)

	results := cd.Search("ab", SearchOptions{})
	if len(results) < 2 {
		t.Fatalf("期望至少 2 个候选，得到 %d", len(results))
	}

	if results[0].Text != "附高权重候选" {
		texts := make([]string, len(results))
		for i, c := range results {
			texts[i] = c.Text
		}
		t.Errorf("高权重 fl 候选应排第一，实际第一=%q，全部=%v", results[0].Text, texts)
	}
}
