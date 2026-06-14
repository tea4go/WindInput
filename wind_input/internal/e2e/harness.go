//go:build windows || darwin

package e2e

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/huanfeng/wind_input/internal/bridge"
	"github.com/huanfeng/wind_input/internal/coordinator"
	"github.com/huanfeng/wind_input/internal/dict"
	"github.com/huanfeng/wind_input/internal/engine"
	"github.com/huanfeng/wind_input/internal/schema"
	"github.com/huanfeng/wind_input/internal/ui"
	"github.com/huanfeng/wind_input/pkg/config"
)

// Harness 持有一套装配好的输入引擎 + coordinator，提供按键驱动与状态快照。
// 非线程安全：用于单线程的测试 / REPL 顺序驱动。
type Harness struct {
	Coord     *coordinator.Coordinator
	EngineMgr *engine.Manager
	SchemaMgr *schema.SchemaManager
	DictMgr   *dict.DictManager
	SchemaID  string

	uiManager     *ui.Manager
	dataDir       string
	dataDirIsTemp bool
	lastResult    *bridge.KeyEventResult
	lastCommand   string
}

// StepSnapshot 是一次驱动操作后的可序列化记录：执行的命令 + 该次按键返回的响应类型/
// 上屏文本 + coordinator 完整核心状态。golden 文件即由若干 StepSnapshot 串成。
type StepSnapshot struct {
	Command    string            `json:"command,omitempty"`
	ResultType string            `json:"result_type"`
	CommitText string            `json:"commit_text,omitempty"`
	State      coordinator.State `json:"state"`
}

// BuildHarness 按 cmd/service/main.go 的序列装配引擎与 coordinator（去 bridge/IPC，
// UI headless）。拼音方案首次需后台生成 wdat，BuildHarness 会同步等待其就绪后再返回。
func BuildHarness(opts Options) (*Harness, error) {
	logger := opts.logger()
	schemaID := opts.schemaID()

	dataRoot := opts.DataRoot
	if dataRoot == "" {
		found, err := findDataRoot()
		if err != nil {
			return nil, err
		}
		dataRoot = found
	}
	if _, err := os.Stat(filepath.Join(dataRoot, "schemas")); err != nil {
		return nil, fmt.Errorf("e2e: DataRoot %q 下没有 schemas/（请先构建生成 build_debug/data，或显式指定 Options.DataRoot）: %w", dataRoot, err)
	}

	dataDir := opts.DataDir
	dataDirIsTemp := false
	if dataDir == "" {
		tmp, err := os.MkdirTemp("", "windinput-e2e-")
		if err != nil {
			return nil, fmt.Errorf("e2e: 创建临时用户目录失败: %w", err)
		}
		dataDir = tmp
		dataDirIsTemp = true
	}

	// 配置：用默认配置保证确定性，不读开发机上的真实 config.toml；仅覆盖活跃方案。
	cfg := config.DefaultConfig()
	cfg.Schema.Active = schemaID
	if opts.TempPinyinTriggerKeys != nil {
		cfg.Input.TempPinyin.TriggerKeys = opts.TempPinyinTriggerKeys
	}
	if opts.Configure != nil {
		opts.Configure(cfg)
	}

	// 引擎 + 方案管理器（main.go:262-282）
	engineMgr := engine.NewManager(logger)
	engineMgr.SetDataRoot(dataRoot)

	schemaMgr := schema.NewSchemaManager(dataRoot, dataDir, logger)
	if err := schemaMgr.LoadSchemas(); err != nil {
		cleanupTemp(dataDir, dataDirIsTemp)
		return nil, fmt.Errorf("e2e: 加载方案失败: %w", err)
	}
	engineMgr.SetSchemaManager(schemaMgr)
	engineMgr.SetPrimarySchemas(cfg.Schema.PrimaryCodetable, cfg.Schema.PrimaryPinyin)

	// 词库管理器（main.go:288-308）。
	// 注意：必须先 OpenStore 再 Initialize —— Initialize 内部 PhraseLayer.LoadFromStore
	// 会访问 store，store 为 nil 时崩溃（与 main.go 总是先 OpenStore 一致）。
	// 用临时空 db：初始等价于"无词频/学习历史"，结果确定；词频/学习用例在运行中通过
	// 选词 + Harness.FlushLearning 在该 db 内累积频次（选词异步批量写，须显式 flush）。
	dictManager := dict.NewDictManager(dataDir, dataRoot, logger)
	if err := dictManager.OpenStore(filepath.Join(dataDir, "user_data.db")); err != nil {
		logger.Warn("e2e: 打开 user_data.db 失败", "err", err)
	}
	if err := dictManager.Initialize(); err != nil {
		logger.Warn("e2e: dict manager 初始化告警", "err", err)
	}
	if pl := dictManager.GetPhraseLayer(); pl != nil {
		pl.SetMinPrefixLength(cfg.Input.Phrase.MinPrefixLength)
	}
	engineMgr.SetDictManager(dictManager)

	// 激活方案 + 切换用户数据层（main.go:320-335）
	if err := schemaMgr.SetActive(schemaID); err != nil {
		dictManager.Close()
		cleanupTemp(dataDir, dataDirIsTemp)
		return nil, fmt.Errorf("e2e: 方案 %q 不存在: %w", schemaID, err)
	}
	activeSchema := schemaMgr.GetActiveSchema()
	if activeSchema != nil {
		dictManager.SwitchSchemaFull(schemaID, activeSchema.DataSchemaID(),
			activeSchema.Learning.TempMaxEntries, activeSchema.Learning.TempPromoteCount)
	}

	// 创建并激活引擎；拼音首次走后台 wdat 生成，需同步等待后重试（main.go:349-379）
	if err := engineMgr.SwitchSchema(schemaID); err != nil {
		if errors.Is(err, schema.ErrAssetBuilding) {
			var wg sync.WaitGroup
			wg.Add(1)
			schema.OnPinyinWdatReady(func() { wg.Done() })
			wg.Wait()
			if err2 := engineMgr.SwitchSchema(schemaID); err2 != nil {
				dictManager.Close()
				cleanupTemp(dataDir, dataDirIsTemp)
				return nil, fmt.Errorf("e2e: wdat 就绪后切换方案 %q 仍失败: %w", schemaID, err2)
			}
		} else {
			dictManager.Close()
			cleanupTemp(dataDir, dataDirIsTemp)
			return nil, fmt.Errorf("e2e: 切换方案 %q 失败: %w", schemaID, err)
		}
	}

	// 学习策略 + 过滤模式（main.go:383-391）
	if activeSchema != nil {
		engineMgr.UpdateLearningConfig(&activeSchema.Learning)
	}
	if cfg.Input.FilterMode != "" {
		engineMgr.UpdateFilterMode(cfg.Input.FilterMode)
	}

	// UI headless：构造但永不 Start()，IsReady() 恒 false，coordinator 候选展示路径
	// 的 ready 守卫会跳过所有弹窗（main.go:393 仅省去 Start goroutine）。
	uiManager := ui.NewManager(logger)

	appCompat := config.LoadAppCompat()
	coord := coordinator.NewCoordinator(engineMgr, uiManager, cfg, appCompat, logger)

	// 决策器接管：harness **显式 pin**，不受 loadDevConfig 读到的开发机 wind_dev.toml 影响，
	// 保证 golden 确定性（生产默认现为 true，但测试不依赖之）。默认关（与录制 golden 一致）；
	// Options.DeciderEnabled 或环境变量 WIND_E2E_DECIDER=1 开启——用于 golden A/B：同一用例在
	// 决策器关/开下复跑，验证新决策器逻辑与旧逻辑逐字节等价。
	deciderOn := opts.DeciderEnabled || os.Getenv("WIND_E2E_DECIDER") == "1"
	coord.SetDeciderEnabledForTest(deciderOn)

	// 引导键特殊模式（自定义码表）：注入测试 fixture 目录并重建注册表。
	// 码表懒加载，仅首次激活时读盘。
	if len(opts.SpecialModes) > 0 {
		dir := opts.SpecialSchemasDir
		if dir == "" {
			dir = "testdata"
		}
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}
		coord.ConfigureSpecialModes(opts.SpecialModes, []string{dir})
	}

	return &Harness{
		Coord:         coord,
		EngineMgr:     engineMgr,
		SchemaMgr:     schemaMgr,
		DictMgr:       dictManager,
		SchemaID:      schemaID,
		uiManager:     uiManager,
		dataDir:       dataDir,
		dataDirIsTemp: dataDirIsTemp,
	}, nil
}

// Close 释放 UI 渲染句柄、词库资源并清理临时用户目录。
// uiManager 虽未 Start()，NewManager 仍初始化了 DirectWrite 渲染器（占图形句柄），
// 必须 Destroy 回收，否则同进程多用例会累积泄漏。
func (h *Harness) Close() {
	if h.uiManager != nil {
		h.uiManager.Destroy()
	}
	if h.DictMgr != nil {
		h.DictMgr.Close()
	}
	cleanupTemp(h.dataDir, h.dataDirIsTemp)
}

// ── 按键驱动 ────────────────────────────────────────────────────────────────

// press 执行一次按键并缓存返回值与命令描述。
func (h *Harness) press(cmd string, d bridge.KeyEventData) *bridge.KeyEventResult {
	h.lastResult = h.Coord.HandleKeyEvent(d)
	h.lastCommand = cmd
	return h.lastResult
}

// Type 逐字符输入一个字符串（每个 rune 一次按键）。
func (h *Harness) Type(s string) {
	for _, r := range s {
		h.press("type "+string(r), runeToKeyEvent(r))
	}
	h.lastCommand = "type " + s
}

// Key 按下一个具名键或单字符（如 "space" / "pagedown" / "a"）。未知键名 panic（测试期暴露）。
func (h *Harness) Key(name string) *bridge.KeyEventResult {
	d, ok := nameToKeyEvent(name)
	if !ok {
		panic(fmt.Sprintf("e2e: 未知键名 %q", name))
	}
	return h.press("key "+name, d)
}

// SelectCandidate 选择当前页第 n 个候选（n ∈ 1..9，0 表示第 10 个）。
func (h *Harness) SelectCandidate(n int) *bridge.KeyEventResult {
	return h.press("select "+strconv.Itoa(n), runeToKeyEvent(rune('0'+n)))
}

// Space / Enter / Backspace / PageDown / PageUp 是常用功能键的便捷封装。
func (h *Harness) Space() *bridge.KeyEventResult     { return h.Key("space") }
func (h *Harness) Enter() *bridge.KeyEventResult     { return h.Key("enter") }
func (h *Harness) Backspace() *bridge.KeyEventResult { return h.Key("backspace") }
func (h *Harness) PageDown() *bridge.KeyEventResult  { return h.Key("pagedown") }
func (h *Harness) PageUp() *bridge.KeyEventResult    { return h.Key("pageup") }

// FlushLearning 同步 flush 词频增量到 store，使前面选词记录的频次对随后查询立即生效。
// 选词的词频写入是异步批量的（生产靠后台 50 条/30s flush），测试需显式调用此方法
// 才能在同一次运行内观察到词频重排。
func (h *Harness) FlushLearning() {
	if h.DictMgr != nil {
		if err := h.DictMgr.FlushFreq(); err != nil {
			panic(fmt.Sprintf("e2e: flush 词频失败: %v", err))
		}
	}
}

// Snapshot 返回当前完整状态 + 上一次按键的响应类型/上屏文本。
func (h *Harness) Snapshot() StepSnapshot {
	return StepSnapshot{
		Command:    h.lastCommand,
		ResultType: resultTypeStr(h.lastResult),
		CommitText: resultCommitText(h.lastResult),
		State:      h.Coord.ExportState(),
	}
}

func resultTypeStr(r *bridge.KeyEventResult) string {
	if r == nil {
		return ""
	}
	return string(r.Type)
}

// resultCommitText 仅在响应确实上屏文本的类型下返回 Text。
func resultCommitText(r *bridge.KeyEventResult) string {
	if r == nil {
		return ""
	}
	switch r.Type {
	case bridge.ResponseTypeInsertText,
		bridge.ResponseTypeInsertTextWithCursor,
		bridge.ResponseTypeReplaceBackward:
		return r.Text
	}
	return ""
}

// ── 路径探测 ────────────────────────────────────────────────────────────────

// findDataRoot 从当前工作目录向上逐级查找含 build_debug/data/schemas/pinyin.schema.toml
// 的目录，返回该 build_debug/data 路径（即 main.go 运行时的 dataRoot）。
func findDataRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("e2e: 取工作目录失败: %w", err)
	}
	dir := cwd
	for {
		candidate := filepath.Join(dir, "build_debug", "data")
		if _, err := os.Stat(filepath.Join(candidate, "schemas", "pinyin.schema.toml")); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("e2e: 从 %q 向上未找到 build_debug/data/schemas（请先构建一次，或用 Options.DataRoot 指定）", cwd)
		}
		dir = parent
	}
}

func cleanupTemp(dir string, isTemp bool) {
	if isTemp && dir != "" {
		_ = os.RemoveAll(dir)
	}
}
