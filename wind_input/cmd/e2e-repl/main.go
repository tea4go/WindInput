//go:build windows || darwin

// e2e-repl 是 in-process 输入法测试的交互式外壳：用真实发布数据装配一套引擎+coordinator
// （headless，不弹窗、不碰 TSF DLL），从 stdin 读指令模拟按键，每步打印完整状态 JSON。
// 用于开发期手动验证（替代"开应用敲字"），与 internal/e2e 的 go test 共用同一 harness。
//
// 用法：
//
//	go run ./cmd/e2e-repl -schema pinyin
//	> type nihao
//	> select 1
//	> state
//	> :quit
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/huanfeng/wind_input/internal/e2e"
)

func main() {
	schemaID := flag.String("schema", "pinyin", "方案 ID: pinyin / wubi86 / shuangpin / wubi86_pinyin")
	dataRoot := flag.String("repo", "", "含 schemas/ 的数据目录（默认自动探测 build_debug/data）")
	dataDir := flag.String("data", "", "用户数据目录（默认临时目录）")
	flag.Parse()

	// 仅显示 Warn 及以上，过滤 DirectWrite/装配的 INFO 噪声。
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	h, err := e2e.BuildHarness(e2e.Options{
		SchemaID: *schemaID,
		DataRoot: *dataRoot,
		DataDir:  *dataDir,
		Logger:   logger,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "构建 harness 失败: %v\n", err)
		os.Exit(1)
	}

	printHelp()
	fmt.Printf("[方案 %s 就绪]\n", h.SchemaID)

	sc := bufio.NewScanner(os.Stdin)
	fmt.Print("> ")
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			fmt.Print("> ")
			continue
		}
		var quit bool
		h, quit = dispatch(h, line, logger)
		if quit {
			break
		}
		fmt.Print("> ")
	}
	h.Close()
}

// dispatch 执行一条 REPL 指令；schema/reset 会重建 harness 并返回新实例。
// 返回 (当前 harness, 是否退出)。
func dispatch(h *e2e.Harness, line string, logger *slog.Logger) (cur *e2e.Harness, quit bool) {
	cur = h
	// h.Key 对未知键名会 panic；REPL 不应因此崩溃，统一兜底。
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("命令出错: %v\n", r)
		}
	}()

	parts := strings.SplitN(line, " ", 2)
	cmd := parts[0]
	arg := ""
	if len(parts) > 1 {
		arg = strings.TrimSpace(parts[1])
	}

	switch cmd {
	case ":quit", ":q", "exit":
		return cur, true
	case "help", "?":
		printHelp()
		return cur, false
	case "type":
		h.Type(arg)
	case "key":
		h.Key(arg)
	case "select", "sel":
		n, err := strconv.Atoi(arg)
		if err != nil {
			fmt.Println("用法: select <0-9>")
			return cur, false
		}
		h.SelectCandidate(n)
	case "space":
		h.Space()
	case "enter":
		h.Enter()
	case "bs", "backspace":
		h.Backspace()
	case "pgdn", "pagedown":
		h.PageDown()
	case "pgup", "pageup":
		h.PageUp()
	case "state", "st":
		// 仅打印当前状态（落到下方 printState）。
	case "schema":
		newH, err := e2e.BuildHarness(e2e.Options{SchemaID: arg, Logger: logger})
		if err != nil {
			fmt.Printf("切换方案失败: %v\n", err)
			return cur, false
		}
		h.Close()
		fmt.Printf("[切换到方案 %s]\n", newH.SchemaID)
		printState(newH)
		return newH, false
	case "reset":
		newH, err := e2e.BuildHarness(e2e.Options{SchemaID: h.SchemaID, Logger: logger})
		if err != nil {
			fmt.Printf("reset 失败: %v\n", err)
			return cur, false
		}
		h.Close()
		printState(newH)
		return newH, false
	default:
		fmt.Printf("未知命令 %q（输入 help 查看）\n", cmd)
		return cur, false
	}

	printState(h)
	return cur, false
}

// printState 把当前快照打印为缩进 JSON。
func printState(h *e2e.Harness) {
	b, err := json.MarshalIndent(h.Snapshot(), "", "  ")
	if err != nil {
		fmt.Printf("序列化状态失败: %v\n", err)
		return
	}
	fmt.Println(string(b))
}

func printHelp() {
	fmt.Println(`命令：
  type <文本>     逐字符输入（如 type nihao）
  key <键名>      按具名键（space/enter/backspace/pageup/pagedown/esc/方向键）或单字符
  select <n>      选当前页第 n 个候选（1-9，0=第10个）
  space / enter   空格上屏 / 回车
  bs              退格
  pgdn / pgup     下翻页 / 上翻页
  state           打印当前状态
  schema <id>     切换方案（pinyin/wubi86/...）
  reset           重置为当前方案的初始状态
  help / :quit    帮助 / 退出`)
}
