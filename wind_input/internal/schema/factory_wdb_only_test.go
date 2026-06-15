// factory_wdb_only_test.go — wdb-only 词库分发模式的回归测试。
//
// 背景：用户不想分享 .dict.yaml 源文件时，可只发布同名 .wdb 文件（已把所有
// import_tables 合并进去的单文件产物）。加载侧检测到 yaml 缺失但同路径 wdb
// 存在时，直接加载 wdb，跳过缓存生成流程。
//
// resolveWdbOnly 的设计要点：与 resolvePath 使用相同的四目录搜索顺序
// (exeDir → exeDir/schemas → dataDir → dataDir/schemas)，使得发布者把
// wdb 放在 dataDir/schemas/<subdir>/ 下就能被正确发现，即便 resolvePath
// 的回退路径指向 exeDir 里的不同目录。
package schema

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huanfeng/wind_input/internal/dict"
	"github.com/huanfeng/wind_input/internal/dict/dictcache"
	"github.com/huanfeng/wind_input/internal/engine/codetable"
)

// makeWdbOnly 辅助：用 writeMinimalRimeCodetable 写 yaml，转换为 wdb，删掉 yaml，
// 返回 wdb 路径。模拟"发布方只提供预编译 wdb、不附带 yaml"的场景。
func makeWdbOnly(t *testing.T, dir, dictName string, entries ...string) string {
	t.Helper()
	logger := slog.New(slog.DiscardHandler)
	yamlPath := writeMinimalRimeCodetable(t, dir, dictName, entries...)
	wdbPath := filepath.Join(dir, dictName+".wdb")
	if err := dictcache.ConvertRimeCodetableToWdb(yamlPath, wdbPath, logger, nil); err != nil {
		t.Fatalf("makeWdbOnly: 转换失败: %v", err)
	}
	if err := os.Remove(yamlPath); err != nil {
		t.Fatalf("makeWdbOnly: 删除 yaml 失败: %v", err)
	}
	return wdbPath
}

// ─── wdbOnlyInDir ─────────────────────────────────────────────────────────────

func TestWdbOnlyInDir(t *testing.T) {
	cases := []struct {
		name     string
		src      string
		wantDir  string
		wantBase string
	}{
		{
			name:     "yaml后缀",
			src:      filepath.Join("data", "schemas", "wubi", "wubi.dict.yaml"),
			wantDir:  filepath.Join("data", "schemas", "wubi"),
			wantBase: "wubi.wdb",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := wdbOnlyInDir(tc.src)
			if got == "" {
				t.Fatalf("wdbOnlyInDir(%q) = \"\", want non-empty", tc.src)
			}
			if filepath.Dir(got) != tc.wantDir {
				t.Errorf("dir = %q, want %q", filepath.Dir(got), tc.wantDir)
			}
			if filepath.Base(got) != tc.wantBase {
				t.Errorf("base = %q, want %q", filepath.Base(got), tc.wantBase)
			}
		})
	}

	t.Run("无已知后缀返回空", func(t *testing.T) {
		if got := wdbOnlyInDir("foo.dict.json"); got != "" {
			t.Errorf("got %q, want \"\"", got)
		}
	})
}

// ─── resolveWdbOnly ──────────────────────────────────────────────────────────

// TestResolveWdbOnly_SearchOrder 验证搜索顺序与 resolvePath 一致：
// exeDir → exeDir/schemas → dataDir → dataDir/schemas。
// 每个子测试只在其中一个目录放 wdb，确认能被找到且搜索不会误命中其他目录。
func TestResolveWdbOnly_SearchOrder(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	rel := filepath.Join("mydict", "mydict.dict.yaml") // 相对词库路径

	setupDirs := func(t *testing.T) (exeDir, datDir string) {
		t.Helper()
		exeDir = t.TempDir()
		datDir = t.TempDir()
		return
	}

	// 在指定目录放 wdb，其他目录不放
	placeWdb := func(t *testing.T, base string) {
		t.Helper()
		sub := filepath.Join(base, "mydict")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		makeWdbOnly(t, sub, "mydict", "abcd\t测试词\t100")
	}

	t.Run("exeDir直接子目录", func(t *testing.T) {
		exeDir, datDir := setupDirs(t)
		placeWdb(t, exeDir)
		got := resolveWdbOnly(exeDir, datDir, rel)
		if got == "" {
			t.Fatal("应在 exeDir 找到 wdb，得到空字符串")
		}
		_ = logger
	})

	t.Run("exeDir_schemas子目录", func(t *testing.T) {
		exeDir, datDir := setupDirs(t)
		placeWdb(t, filepath.Join(exeDir, "schemas"))
		got := resolveWdbOnly(exeDir, datDir, rel)
		if got == "" {
			t.Fatal("应在 exeDir/schemas 找到 wdb，得到空字符串")
		}
	})

	t.Run("dataDir直接子目录", func(t *testing.T) {
		exeDir, datDir := setupDirs(t)
		placeWdb(t, datDir)
		got := resolveWdbOnly(exeDir, datDir, rel)
		if got == "" {
			t.Fatal("应在 dataDir 找到 wdb，得到空字符串")
		}
	})

	t.Run("dataDir_schemas子目录", func(t *testing.T) {
		exeDir, datDir := setupDirs(t)
		placeWdb(t, filepath.Join(datDir, "schemas"))
		got := resolveWdbOnly(exeDir, datDir, rel)
		if got == "" {
			t.Fatal("应在 dataDir/schemas 找到 wdb，得到空字符串")
		}
	})

	t.Run("wdb不存在返回空", func(t *testing.T) {
		exeDir, datDir := setupDirs(t)
		got := resolveWdbOnly(exeDir, datDir, rel)
		if got != "" {
			t.Errorf("wdb 不存在时应返回空，got %q", got)
		}
	})

	t.Run("exeDir优先于dataDir", func(t *testing.T) {
		exeDir, datDir := setupDirs(t)
		// 两处都放 wdb（内容相同但路径不同），应返回 exeDir 那个
		placeWdb(t, exeDir)
		placeWdb(t, datDir)
		got := resolveWdbOnly(exeDir, datDir, rel)
		wantPrefix := exeDir
		if !strings.HasPrefix(got, wantPrefix) {
			t.Errorf("got %q，应优先返回 exeDir 下的路径（前缀 %q）", got, wantPrefix)
		}
	})

	t.Run("绝对路径降级为同目录查找", func(t *testing.T) {
		dir := t.TempDir()
		absYaml := filepath.Join(dir, "mydict.dict.yaml")
		makeWdbOnly(t, dir, "mydict", "abcd\t测试词\t100")
		got := resolveWdbOnly("", "", absYaml)
		if got == "" {
			t.Fatal("绝对路径场景：应在同目录找到 wdb")
		}
	})
}

// ─── loadCodetable wdb-only 路径 ──────────────────────────────────────────────

// TestLoadCodetable_WdbOnly 验证：yaml 缺失但 wdbOnlyHint 指向合法 wdb 时，
// loadCodetable 能直接加载 wdb，不尝试转换 yaml（yaml 不存在也不报错）。
func TestLoadCodetable_WdbOnly(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	dir := t.TempDir()

	// 制造 wdb-only 文件
	makeWdbOnly(t, dir, "test_main", "abcd\t测词\t100", "abce\t另词\t50")

	wdbPath := filepath.Join(dir, "test_main.wdb")
	// srcPath 指向不存在的 yaml（模拟安装目录里没有源文件）
	fakeSrcPath := filepath.Join(t.TempDir(), "test_main.dict.yaml")

	engine := codetable.NewEngine(&codetable.Config{MaxCodeLength: 4}, logger)
	// mmap 持有文件句柄：必须在 TempDir 清理前 Close（t.Cleanup 按 LIFO 顺序执行）
	t.Cleanup(func() { engine.Close() })
	cacheKey := "test_wdb_only_main"

	if err := loadCodetable(engine, fakeSrcPath, wdbPath, DictTypeRimeCodetable, cacheKey, logger, nil); err != nil {
		t.Fatalf("loadCodetable wdb-only 失败: %v", err)
	}

	ct := engine.GetCodeTable()
	if ct == nil {
		t.Fatal("加载后 CodeTable 为 nil")
	}
	if ct.EntryCount() == 0 {
		t.Error("加载后 EntryCount = 0，应有词条")
	}
}

// TestLoadCodetable_WdbOnly_EmptyHint_SameDir 验证回退路径：wdbOnlyHint 为空时，
// 若 yaml 缺失而同目录存在同名 wdb，也能正常加载。
func TestLoadCodetable_WdbOnly_EmptyHint_SameDir(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	dir := t.TempDir()
	makeWdbOnly(t, dir, "fallback_main", "zzzz\t回退词\t10")

	// srcPath 与 wdb 同目录（wdbOnlyInDir 能推导出来），hint 故意传空
	fakeSrcPath := filepath.Join(dir, "fallback_main.dict.yaml")
	engine := codetable.NewEngine(&codetable.Config{MaxCodeLength: 4}, logger)
	t.Cleanup(func() { engine.Close() })

	if err := loadCodetable(engine, fakeSrcPath, "", DictTypeRimeCodetable, "fallback_key", logger, nil); err != nil {
		t.Fatalf("同目录回退路径加载失败: %v", err)
	}
	if engine.GetCodeTable() == nil || engine.GetCodeTable().EntryCount() == 0 {
		t.Error("同目录回退：加载后词条为空")
	}
}

// ─── loadExtraCodetable wdb-only 路径 ─────────────────────────────────────────

// TestLoadExtraCodetable_WdbOnly 验证：附加词库 yaml 缺失但 wdbOnlyHint 存在时，
// 能正确注册为 DictManager 的 system layer 并可在 CompositeDict 中检索到词条。
func TestLoadExtraCodetable_WdbOnly(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	dir := t.TempDir()
	makeWdbOnly(t, dir, "test_extra", "bauq\t孙燕姿\t100")

	wdbPath := filepath.Join(dir, "test_extra.wdb")
	fakeSrcPath := filepath.Join(t.TempDir(), "test_extra.dict.yaml")

	dm := dict.NewDictManager(t.TempDir(), t.TempDir(), logger)
	spec := DictSpec{
		ID:   "extra",
		Path: fakeSrcPath,
		Type: DictTypeRimeCodetable,
	}

	layer, err := loadExtraCodetable(dm, "wubitest", fakeSrcPath, wdbPath, spec, "wubitest_extra", logger)
	if err != nil {
		t.Fatalf("loadExtraCodetable wdb-only 失败: %v", err)
	}
	if layer == nil {
		t.Fatal("返回 layer 为 nil")
	}
	// CodeTableLayer 持有 mmap 句柄，测试结束前需先 Close 再清理 TempDir
	if cl, ok := layer.(*dict.CodeTableLayer); ok {
		t.Cleanup(func() { cl.Close() })
	}

	cd := dm.GetCompositeDict()
	if !compositeHasText(cd, "bauq", "孙燕姿") {
		t.Errorf("CompositeDict 中未找到 孙燕姿(bauq)；候选=%v",
			candTexts(cd.Search("bauq", dict.SearchOptions{})))
	}
}

// TestLoadExtraCodetable_WdbOnly_EmptyHint_SameDir 验证附加词库同目录回退路径。
func TestLoadExtraCodetable_WdbOnly_EmptyHint_SameDir(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	dir := t.TempDir()
	makeWdbOnly(t, dir, "fallback_extra", "abcd\t回退附加词\t1")

	fakeSrcPath := filepath.Join(dir, "fallback_extra.dict.yaml")
	dm := dict.NewDictManager(t.TempDir(), t.TempDir(), logger)
	spec := DictSpec{ID: "extra", Path: fakeSrcPath, Type: DictTypeRimeCodetable}

	layer, err := loadExtraCodetable(dm, "s", fakeSrcPath, "", spec, "s_extra", logger)
	if err != nil {
		t.Fatalf("同目录回退附加词库失败: %v", err)
	}
	if layer == nil {
		t.Fatal("layer 为 nil")
	}
	if cl, ok := layer.(*dict.CodeTableLayer); ok {
		t.Cleanup(func() { cl.Close() })
	}
}

// ─── ensureCodetableWdb wdb-only 跳过 ─────────────────────────────────────────

// TestEnsureCodetableWdb_WdbOnly_Skip 验证：yaml 不存在但 wdbOnlyHint 非空时，
// ensureCodetableWdb 立即返回 nil（不尝试转换 yaml、不报错）。
func TestEnsureCodetableWdb_WdbOnly_Skip(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	dir := t.TempDir()
	makeWdbOnly(t, dir, "skip_test", "abcd\t词\t1")

	wdbHint := filepath.Join(dir, "skip_test.wdb")
	fakeSrcPath := filepath.Join(t.TempDir(), "skip_test.dict.yaml") // yaml 不存在

	if err := ensureCodetableWdb(fakeSrcPath, wdbHint, DictTypeRimeCodetable, "skip_key", nil, logger); err != nil {
		t.Errorf("wdb-only 场景 ensureCodetableWdb 应返回 nil，got: %v", err)
	}
}

// TestEnsureCodetableWdb_WdbOnly_EmptyHint_SameDir 验证同目录回退的跳过行为。
func TestEnsureCodetableWdb_WdbOnly_EmptyHint_SameDir(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	dir := t.TempDir()
	makeWdbOnly(t, dir, "skip_samedir", "abcd\t词\t1")

	fakeSrcPath := filepath.Join(dir, "skip_samedir.dict.yaml") // yaml 不存在，wdb 在同目录

	if err := ensureCodetableWdb(fakeSrcPath, "", DictTypeRimeCodetable, "skip_samedir_key", nil, logger); err != nil {
		t.Errorf("同目录回退 ensureCodetableWdb 应返回 nil，got: %v", err)
	}
}

// TestResolveWdbOnly_EndToEnd 端到端验证：模拟 resolvePath 回退到 exeDir 但 wdb
// 实际位于 dataDir/schemas 的场景（即本功能的核心动机：wubiex 方案放在用户 Roaming
// 目录而非 Program Files）。
func TestResolveWdbOnly_EndToEnd(t *testing.T) {
	exeDir := t.TempDir() // 模拟 Program Files/data
	datDir := t.TempDir() // 模拟 Roaming/WindInput

	// 用户把 wdb 放在 dataDir/schemas/wubiex/
	wubiexDir := filepath.Join(datDir, "schemas", "wubiex")
	if err := os.MkdirAll(wubiexDir, 0o755); err != nil {
		t.Fatal(err)
	}
	makeWdbOnly(t, wubiexDir, "wubiex_jidian", "abcd\t测词\t100")

	// yaml 相对路径（与方案配置中写法一致）
	dictRelPath := filepath.Join("wubiex", "wubiex_jidian.dict.yaml")

	got := resolveWdbOnly(exeDir, datDir, dictRelPath)
	if got == "" {
		t.Fatal("端到端：未在 dataDir/schemas/wubiex/ 找到 wdb")
	}
	wantPath := filepath.Join(wubiexDir, "wubiex_jidian.wdb")
	if got != wantPath {
		t.Errorf("got %q, want %q", got, wantPath)
	}

	// 进一步验证整个 loadCodetable 可用该 hint 正常加载
	logger := slog.New(slog.DiscardHandler)
	// srcPath 回退到 exeDir（yaml 不存在）
	fakeSrcPath := filepath.Join(exeDir, "schemas", "wubiex", "wubiex_jidian.dict.yaml")
	engine := codetable.NewEngine(&codetable.Config{MaxCodeLength: 4}, logger)
	t.Cleanup(func() { engine.Close() })
	if err := loadCodetable(engine, fakeSrcPath, got, DictTypeRimeCodetable, "e2e_key", logger, nil); err != nil {
		t.Fatalf("端到端 loadCodetable 失败: %v", err)
	}
	if engine.GetCodeTable() == nil || engine.GetCodeTable().EntryCount() == 0 {
		t.Error("端到端：加载后词条为空")
	}
}
