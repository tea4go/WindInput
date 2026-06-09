# 设置工具轻量 Web 模式 实现计划（方案 D：镜像 Wails App 绑定）

> **For agentic workers:** REQUIRED SUB-SKILL: 使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 按任务逐个实现。步骤用 checkbox（`- [ ]`）跟踪。
>
> **项目规则覆盖**：本仓库 CLAUDE.md 规定「功能未经测试前不主动 git commit」「不主动 git push」。每个 Task 末尾 commit 仅在该 Task 测试/编译通过后执行；有疑问先与用户确认。改 Go 后运行 `go fmt`，改前端后运行格式化。文档与提交信息不使用 emoji。

**Goal:** 为 `wind_setting` 增加不依赖 WebView2 的 Web 模式：HTTP 反射网关镜像 Wails 对 `*App` 的方法绑定，前端注入 `window.go` Proxy shim，使 `wails.ts` 与所有页面零改动透明走 HTTP。

**Architecture:** 单进程双形态。`main.go` 按 `--web`/`--gui`/WebView2 探测决定 `wails.Run`（GUI）或 `webServer`（Web）。Web 形态：`net/http` 同源托管 embed `dist` + `POST /api/call`（反射分发到 `*App` 导出方法）+ `GET /api/events`（SSE 桥接 `rpcClient.SubscribeEvents`）。前端 shim 把 `window.go.main.App.XXX(args)` 转为 `/api/call`，`window.runtime.EventsOn` 转为 SSE。

**Tech Stack:** Go（`net/http`、`reflect`、`encoding/json`、`httptest`、`embed`、`golang.org/x/sys/windows/registry`）；前端 TypeScript（Proxy、EventSource、Vite/pnpm）。

---

## 已核实事实（依据，勿凭记忆改写）

- `main.go`：`//go:embed all:frontend/dist` → `var assets embed.FS`；`wails.Run(&options.App{... Bind: []interface{}{app} ...})`。
- `*App` 方法均为 `func (a *App) Xxx(...) (T, error)` 或 `(T)` 或 `error` 形态；业务经 `a.rpcClient`（`rpcapi.Client`）走命名管道。已确认方法如 `GetConfig() (*config.Config, error)`、`SetConfigItems([]rpcapi.ConfigSetItem)(*SaveConfigResult,error)`、`SwitchActiveSchema(string) error`、`GetServiceStatus()(*rpcapi.SystemStatusReply,error)`、`GetAvailableThemes()`、`GetThemePreview(string,string)(map[string]interface{},error)`、`SubscribeEvents(ctx, cb)` 等。
- `app.go startEventListener()`：`rpcClient.SubscribeEvents` 回调把 `EventMessage` 按 type（config/stats/system/dict）`wailsRuntime.EventsEmit`。Web 模式以 SSE 复刻这套映射。
- `theme_server.go`：`corsMiddleware`、端口探测（`net.Listen` 顺延）、`Shutdown` 可仿。
- `shellOpen(path) error`、`showNativeMessageBox(title,message)` 两平台均有。
- 前端 `wails.ts` 经 `import * as App from "../../wailsjs/go/main/App"` 与 `(window as any).go.main.App.XXX()` 两种方式调用——shim 需同时满足：既提供 `window.go.main.App`，也保证 `wailsjs/go/main/App` 的运行时实现走同一通道（见 Task 9 说明）。

---

## 文件结构

| 文件 | 职责 |
|------|------|
| `wind_setting/web_server.go` | webServer：`/api/call` 反射网关、`/api/events` SSE、静态托管、端口探测启停 |
| `wind_setting/web_server_test.go` | 反射网关纯函数 + 端点单测 |
| `wind_setting/webview_detect_windows.go` / `_other.go` | WebView2 探测 |
| `wind_setting/run_mode.go` / `run_mode_test.go` | 形态决策（纯函数）+ Web 启动 + ctx/事件初始化 + URL 递送 |
| `wind_setting/main.go` | 形态分流接入 |
| `wind_setting/frontend/src/lib/webShim.ts` | `window.go`/`window.runtime` Proxy shim + `__WEB_MODE__` |
| `wind_setting/frontend/src/main.ts` | 非 Wails 环境安装 shim |
| `pages/*.vue` 等 | 按 `__WEB_MODE__` 禁用原生依赖功能块 + 提示 |

---

## Task 1：反射调用纯函数 callReflect（核心）

**Files:** Create `wind_setting/web_server.go`、`wind_setting/web_server_test.go`

- [ ] **Step 1：写失败测试**（用独立 fake 目标，不依赖 App/rpcClient）

```go
package main

import (
	"encoding/json"
	"errors"
	"testing"
)

type fakeTarget struct{}

func (fakeTarget) Echo(s string) string            { return "hi:" + s }
func (fakeTarget) Add(a, b int) int                { return a + b }
func (fakeTarget) Boom() (string, error)           { return "", errors.New("boom") }
func (fakeTarget) Ok() (string, error)             { return "ok", nil }
func (fakeTarget) Noop()                           {}

func raw(s string) json.RawMessage { return json.RawMessage(s) }

func TestCallReflect_StringArg(t *testing.T) {
	data, err := callReflect(fakeTarget{}, "Echo", []json.RawMessage{raw(`"x"`)})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if data != "hi:x" {
		t.Fatalf("data = %v, want hi:x", data)
	}
}

func TestCallReflect_TwoIntArgs(t *testing.T) {
	data, err := callReflect(fakeTarget{}, "Add", []json.RawMessage{raw(`2`), raw(`3`)})
	if err != nil || data != 5 {
		t.Fatalf("data=%v err=%v, want 5 nil", data, err)
	}
}

func TestCallReflect_ErrorReturn(t *testing.T) {
	_, err := callReflect(fakeTarget{}, "Boom", nil)
	if err == nil || err.Error() != "boom" {
		t.Fatalf("err = %v, want boom", err)
	}
}

func TestCallReflect_DataWithNilError(t *testing.T) {
	data, err := callReflect(fakeTarget{}, "Ok", nil)
	if err != nil || data != "ok" {
		t.Fatalf("data=%v err=%v, want ok nil", data, err)
	}
}

func TestCallReflect_UnknownMethod(t *testing.T) {
	_, err := callReflect(fakeTarget{}, "Nope", nil)
	if err == nil {
		t.Fatalf("want error for unknown method")
	}
}

func TestCallReflect_ArgCountMismatch(t *testing.T) {
	_, err := callReflect(fakeTarget{}, "Add", []json.RawMessage{raw(`1`)})
	if err == nil {
		t.Fatalf("want error for arg count mismatch")
	}
}

func TestCallReflect_VoidMethod(t *testing.T) {
	data, err := callReflect(fakeTarget{}, "Noop", nil)
	if err != nil || data != nil {
		t.Fatalf("data=%v err=%v, want nil nil", data, err)
	}
}
```

- [ ] **Step 2：确认失败**

Run: `go test ./wind_setting/ -run TestCallReflect -v`
Expected: 编译失败（`callReflect` 未定义）

- [ ] **Step 3：实现 callReflect**

```go
// wind_setting/web_server.go
package main

import (
	"encoding/json"
	"fmt"
	"reflect"
)

var errType = reflect.TypeOf((*error)(nil)).Elem()

// callReflect 在 target 上按名查找导出方法并调用。
// rawArgs 按各形参类型 JSON 反序列化。返回值约定：若最后一个返回值是 error
// 则作为 error；其余作为 data（0 个→nil，1 个→该值，多个→[]interface{}）。
func callReflect(target interface{}, method string, rawArgs []json.RawMessage) (interface{}, error) {
	m := reflect.ValueOf(target).MethodByName(method)
	if !m.IsValid() {
		return nil, fmt.Errorf("unknown method: %s", method)
	}
	mt := m.Type()
	if len(rawArgs) != mt.NumIn() {
		return nil, fmt.Errorf("method %s expects %d args, got %d", method, mt.NumIn(), len(rawArgs))
	}
	in := make([]reflect.Value, mt.NumIn())
	for i := 0; i < mt.NumIn(); i++ {
		p := reflect.New(mt.In(i)) // *ParamType
		if err := json.Unmarshal(rawArgs[i], p.Interface()); err != nil {
			return nil, fmt.Errorf("arg %d (%s): %w", i, mt.In(i), err)
		}
		in[i] = p.Elem()
	}
	out := m.Call(in)

	// 分离尾部 error
	var retErr error
	if n := len(out); n > 0 && out[n-1].Type().Implements(errType) {
		if e := out[n-1]; !e.IsNil() {
			retErr = e.Interface().(error)
		}
		out = out[:n-1]
	}
	if retErr != nil {
		return nil, retErr
	}
	switch len(out) {
	case 0:
		return nil, nil
	case 1:
		return out[0].Interface(), nil
	default:
		arr := make([]interface{}, len(out))
		for i, v := range out {
			arr[i] = v.Interface()
		}
		return arr, nil
	}
}
```

- [ ] **Step 4：确认通过** — Run: `go test ./wind_setting/ -run TestCallReflect -v` → 全 PASS
- [ ] **Step 5：go fmt + commit**

```bash
cd wind_setting && go fmt ./... && cd ..
git add wind_setting/web_server.go wind_setting/web_server_test.go
git commit -m "feat(setting): web 模式反射调用核心 callReflect"
```

---

## Task 2：POST /api/call 端点

**Files:** Modify `wind_setting/web_server.go`、`web_server_test.go`

- [ ] **Step 1：写失败测试**

```go
import (
	"net/http"
	"net/http/httptest"
)

func TestCallEndpoint_MethodGuard(t *testing.T) {
	ws := &webServer{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/call", nil)
	ws.muxWithStatic(nil).ServeHTTP(rec, req)
	var resp callResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Error == "" {
		t.Fatalf("want error in body for GET /api/call")
	}
}

func TestCallEndpoint_UnknownMethod(t *testing.T) {
	ws := &webServer{app: &App{}} // 不触达 rpcClient 的方法
	rec := httptest.NewRecorder()
	body := `{"method":"NoSuchMethod","args":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/call", strings_NewReader(body))
	ws.muxWithStatic(nil).ServeHTTP(rec, req)
	var resp callResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Error == "" {
		t.Fatalf("want error for unknown method")
	}
}
```

顶部 import `"strings"`，并把 `strings_NewReader` 替换为 `strings.NewReader`（此处占位提示，实际写 `strings.NewReader`）。

- [ ] **Step 2：确认失败** — Run: `go test ./wind_setting/ -run TestCallEndpoint -v` → 编译失败（`webServer`/`callResponse`/`muxWithStatic` 未定义）

- [ ] **Step 3：实现**

```go
import (
	"io/fs"
	"net/http"
)

type webServer struct {
	app    *App
	server *http.Server
	port   int
}

type callRequest struct {
	Method string            `json:"method"`
	Args   []json.RawMessage `json:"args"`
}

type callResponse struct {
	Data  interface{} `json:"data"`
	Error string      `json:"error,omitempty"`
}

func writeCall(w http.ResponseWriter, data interface{}, errStr string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(callResponse{Data: data, Error: errStr})
}

func (ws *webServer) handleCall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeCall(w, nil, "method not allowed")
		return
	}
	var req callRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCall(w, nil, "bad request: "+err.Error())
		return
	}
	data, err := callReflect(ws.app, req.Method, req.Args)
	if err != nil {
		writeCall(w, nil, err.Error())
		return
	}
	writeCall(w, data, "")
}

func (ws *webServer) muxWithStatic(staticFS fs.FS) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/call", ws.handleCall)
	mux.HandleFunc("/api/events", ws.handleEvents) // Task 4 实现；先占位空 handler 以便编译
	if staticFS != nil {
		mux.Handle("/", http.FileServer(http.FS(staticFS)))
	}
	return corsMiddleware(mux)
}
```

> 为通过本 Task 编译，先加最小占位 `func (ws *webServer) handleEvents(w http.ResponseWriter, r *http.Request) {}`，Task 4 再实现。

- [ ] **Step 4：确认通过** — Run: `go test ./wind_setting/ -run "TestCall" -v` → PASS
- [ ] **Step 5：go fmt + commit**

```bash
cd wind_setting && go fmt ./... && cd ..
git add wind_setting/web_server.go wind_setting/web_server_test.go
git commit -m "feat(setting): web 模式 /api/call 反射网关端点"
```

---

## Task 3：静态托管 + 端口探测 + 启停

**Files:** Modify `wind_setting/web_server.go`、`web_server_test.go`

- [ ] **Step 1：写失败测试**

```go
func TestStartPicksFreePort(t *testing.T) {
	ws := &webServer{}
	if err := ws.Start(nil); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer ws.Stop()
	if ws.port < webServerBasePort || ws.port >= webServerBasePort+webServerMaxTries {
		t.Fatalf("port = %d out of range", ws.port)
	}
}
```

- [ ] **Step 2：确认失败** — Run: `go test ./wind_setting/ -run TestStartPicksFreePort -v` → 编译失败

- [ ] **Step 3：实现**（仿 theme_server.go）

```go
import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"
)

const (
	webServerBasePort = 18923
	webServerMaxTries = 3
)

func (ws *webServer) Start(staticFS fs.FS) error {
	handler := ws.muxWithStatic(staticFS)
	var lastErr error
	for i := 0; i < webServerMaxTries; i++ {
		port := webServerBasePort + i
		ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
		if err != nil {
			lastErr = err
			continue
		}
		ws.port = port
		srv := &http.Server{Handler: handler}
		ws.server = srv
		go func() { _ = srv.Serve(ln) }()
		return nil
	}
	return fmt.Errorf("端口 %d-%d 均被占用：%w", webServerBasePort, webServerBasePort+webServerMaxTries-1, lastErr)
}

func (ws *webServer) Stop() {
	if ws.server == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = ws.server.Shutdown(ctx)
	ws.server = nil
	ws.port = 0
}

func (ws *webServer) url() string { return "http://127.0.0.1:" + strconv.Itoa(ws.port) }
```

- [ ] **Step 4：确认通过** — Run: `go test ./wind_setting/ -v` → PASS
- [ ] **Step 5：go fmt + commit**

```bash
cd wind_setting && go fmt ./... && cd ..
git add wind_setting/web_server.go wind_setting/web_server_test.go
git commit -m "feat(setting): web 模式静态托管与端口探测启停"
```

---

## Task 4：GET /api/events（SSE 桥接 SubscribeEvents）

**Files:** Modify `wind_setting/web_server.go`、`web_server_test.go`

- [ ] **Step 1：写失败测试**（验证 SSE 头与可写；事件源用注入的 fake，避免依赖真实管道）

```go
func TestEvents_SetsSSEHeaders(t *testing.T) {
	ws := &webServer{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	// 立即取消，避免 handler 阻塞
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(ctx)
	ws.muxWithStatic(nil).ServeHTTP(rec, req)
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}
}
```

- [ ] **Step 2：确认失败** — Run: `go test ./wind_setting/ -run TestEvents -v` → FAIL（占位 handler 未设头）

- [ ] **Step 3：实现**

```go
func (ws *webServer) handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}
	if ws.app == nil || ws.app.rpcClient == nil {
		return
	}
	// 订阅 RPC 事件并按现有 startEventListener 的映射写 SSE。
	// 用请求 ctx，断开即退出订阅。
	send := func(eventName string, payload map[string]string) {
		b, _ := json.Marshal(payload)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventName, b)
		flusher.Flush()
	}
	_ = ws.app.rpcClient.SubscribeEvents(r.Context(), func(msg rpcapi.EventMessage) {
		payload := map[string]string{
			"type":      string(msg.Type),
			"schema_id": msg.SchemaID,
			"action":    string(msg.Action),
		}
		switch msg.Type {
		case rpcapi.EventTypeConfig:
			send(rpcapi.WailsEventConfig, payload)
		case rpcapi.EventTypeStats:
			send(rpcapi.WailsEventStats, payload)
		case rpcapi.EventTypeSystem:
			send(rpcapi.WailsEventSystem, payload)
		default:
			send(rpcapi.WailsEventDict, payload)
		}
	})
}
```

顶部 import 加 `"github.com/huanfeng/wind_input/pkg/rpcapi"`。

> 执行注意：`rpcapi.EventMessage`/`EventTypeConfig`/`WailsEventConfig` 等符号名照 `app.go startEventListener` 已用的写（已确认存在）。SSE 的 `eventName` 用与 Wails 事件同名常量，前端 shim 的 `EventsOn(name)` 即可对上。

- [ ] **Step 4：确认通过** — Run: `go test ./wind_setting/ -run TestEvents -v` → PASS
- [ ] **Step 5：go fmt + commit**

```bash
cd wind_setting && go fmt ./... && cd ..
git add wind_setting/web_server.go wind_setting/web_server_test.go
git commit -m "feat(setting): web 模式 SSE 事件桥接"
```

---

## Task 5：WebView2 探测

**Files:** Create `webview_detect_windows.go`、`webview_detect_other.go`、`webview_detect_test.go`

- [ ] **Step 1：写失败测试**

```go
package main

import "testing"

func TestWebView2DetectDoesNotPanic(t *testing.T) {
	_ = isWebView2Installed()
}
```

- [ ] **Step 2：确认失败** — Run: `go test ./wind_setting/ -run TestWebView2 -v` → 编译失败

- [ ] **Step 3：实现**

```go
// webview_detect_windows.go
//go:build windows

package main

import "golang.org/x/sys/windows/registry"

func isWebView2Installed() bool {
	keys := []string{
		`SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}`,
		`SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}`,
	}
	roots := []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER}
	for _, key := range keys {
		for _, root := range roots {
			k, err := registry.OpenKey(root, key, registry.QUERY_VALUE)
			if err != nil {
				continue
			}
			pv, _, err := k.GetStringValue("pv")
			k.Close()
			if err == nil && pv != "" && pv != "0.0.0.0" {
				return true
			}
		}
	}
	return false
}
```

```go
// webview_detect_other.go
//go:build !windows

package main

func isWebView2Installed() bool { return true }
```

- [ ] **Step 4：确认通过** — Run: `go test ./wind_setting/ -run TestWebView2 -v` → PASS
- [ ] **Step 5：go fmt + commit**

```bash
cd wind_setting && go fmt ./... && cd ..
git add wind_setting/webview_detect_windows.go wind_setting/webview_detect_other.go wind_setting/webview_detect_test.go
git commit -m "feat(setting): WebView2 运行时探测"
```

---

## Task 6：形态决策纯函数

**Files:** Create `run_mode.go`、`run_mode_test.go`

- [ ] **Step 1：写失败测试**

```go
package main

import "testing"

func TestResolveRunMode(t *testing.T) {
	cases := []struct {
		args         []string
		webInstalled bool
		want         runMode
	}{
		{[]string{"--web"}, true, modeWeb},
		{[]string{"--gui"}, false, modeGUI},
		{[]string{}, true, modeGUI},
		{[]string{}, false, modeWeb},
		{[]string{"--page", "about"}, true, modeGUI},
	}
	for _, c := range cases {
		if got := resolveRunMode(c.args, c.webInstalled); got != c.want {
			t.Fatalf("resolveRunMode(%v,%v)=%v want %v", c.args, c.webInstalled, got, c.want)
		}
	}
}
```

- [ ] **Step 2：确认失败** — Run: `go test ./wind_setting/ -run TestResolveRunMode -v` → 编译失败

- [ ] **Step 3：实现**

```go
// run_mode.go
package main

import "strings"

type runMode int

const (
	modeGUI runMode = iota
	modeWeb
)

func resolveRunMode(args []string, webViewInstalled bool) runMode {
	for _, a := range args {
		switch strings.ToLower(a) {
		case "--web":
			return modeWeb
		case "--gui":
			return modeGUI
		}
	}
	if webViewInstalled {
		return modeGUI
	}
	return modeWeb
}
```

- [ ] **Step 4：确认通过** — Run: `go test ./wind_setting/ -run TestResolveRunMode -v` → PASS
- [ ] **Step 5：go fmt + commit**

```bash
cd wind_setting && go fmt ./... && cd ..
git add wind_setting/run_mode.go wind_setting/run_mode_test.go
git commit -m "feat(setting): 运行形态决策纯函数"
```

---

## Task 7：runWebMode（启动 + ctx/事件初始化 + URL 递送）

**Files:** Modify `run_mode.go`

- [ ] **Step 1：实现**

```go
import (
	"context"
	"io/fs"
)

// runWebMode 启动 Web 形态：初始化 app 上下文、起 HTTP 服务、递送 URL，阻塞保活。
func runWebMode(app *App, staticFS fs.FS) {
	// Web 形态不走 wails.Run，手动补关键初始化：
	app.ctx = context.Background() // 占位 ctx，供读 a.ctx 的方法使用

	ws := &webServer{app: app}
	if err := ws.Start(staticFS); err != nil {
		showNativeMessageBox("清风输入法设置 - 启动失败", "无法启动 Web 设置服务：\n"+err.Error())
		return
	}
	defer ws.Stop()

	u := ws.url()
	_ = shellOpen(u) // 自动开浏览器，失败不致命
	showNativeMessageBox("清风输入法设置（Web 模式）",
		"已在浏览器打开设置页。若未自动打开，请手动访问：\n"+u)

	select {} // 保活，事件订阅在 /api/events 连接时按需启动
}
```

> 设计说明：事件订阅放在 `/api/events` 首次连接时启动（Task 4），不在此处常驻，避免无前端时空跑。`app.ctx` 用 `context.Background()`，使读 `a.ctx` 的方法（如 `wailsRuntime.Log*`）不空指针；这些 Wails 日志调用在无 runtime 时的容错见下方风险。

> 风险/执行注意：若某些被反射调用的方法内部直接调 `wailsRuntime.Log*(a.ctx, ...)` 在无 Wails runtime 下报错或 panic，需在执行时确认其行为；多数 `wailsRuntime.Log*` 仅写日志，传 `context.Background()` 一般安全。依赖 dialog/EventsEmit 的方法由前端禁用（Task 11），不会被调用到。

- [ ] **Step 2：编译验证** — Run: `cd wind_setting && go build ./... && cd ..` → 通过
- [ ] **Step 3：go fmt + commit**

```bash
cd wind_setting && go fmt ./... && cd ..
git add wind_setting/run_mode.go
git commit -m "feat(setting): Web 模式启动与 URL 递送"
```

---

## Task 8：main.go 接入形态分流

**Files:** Modify `wind_setting/main.go`

- [ ] **Step 1：实现**

在 `main()` 内、`app := NewApp()` 及其字段赋值之后、`wails.Run(...)` 之前插入：

```go
	if resolveRunMode(os.Args[1:], isWebView2Installed()) == modeWeb {
		distFS, err := fs.Sub(assets, "frontend/dist")
		if err != nil {
			showNativeMessageBox("清风输入法设置 - 启动失败", err.Error())
			return
		}
		runWebMode(app, distFS)
		return
	}
```

顶部 import 加 `"io/fs"`。

- [ ] **Step 2：编译 + 冒烟**

Run: `cd wind_setting && go build -o wind_setting.exe . && cd ..`
Run（手动）: `wind_setting\wind_setting.exe --web`
Expected: 原生提示框显示 `http://127.0.0.1:18923`，浏览器打开后设置页加载（前端 shim 尚未装，API 调用会失败属正常，下一 Task 处理）

- [ ] **Step 3：go fmt + commit**

```bash
cd wind_setting && go fmt ./... && cd ..
git add wind_setting/main.go
git commit -m "feat(setting): main 接入 Web/GUI 形态分流"
```

---

## Task 9：前端 webShim.ts + main.ts 安装

**Files:** Create `frontend/src/lib/webShim.ts`；Modify `frontend/src/main.ts`

> 执行前 Read `main.ts` 确认 App 挂载位置与 `wailsjs` 运行时绑定方式。关键：`wails.ts` 同时用 `import * as App from "wailsjs/go/main/App"` 与 `window.go.main.App`。`wailsjs/go/main/App` 的生成实现内部正是调用 `window.go.main.App.*`（Wails 约定），因此**只要 shim 提供 `window.go.main.App`，两条路径都通**。执行时打开一个 `wailsjs/go/main/App.js` 确认其实现确实转发到 `window.go`（若不是，则 shim 需改为覆盖 `wailsjs` 运行时的 `Call` 入口——读后决定）。

- [ ] **Step 1：实现 webShim.ts**

```ts
// 非 Wails（浏览器 Web 模式）下，注入与 Wails runtime 等价的 window.go / window.runtime。
// 使 wails.ts 与所有页面零改动透明走 HTTP。
export function installWebShimIfNeeded(): void {
  const w = window as any;
  if (w.go?.main?.App) return; // 已在 Wails 环境

  const call = (method: string, args: any[]) =>
    fetch("/api/call", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ method, args }),
    })
      .then((r) => r.json())
      .then((r) => (r.error ? Promise.reject(new Error(r.error)) : r.data));

  const appProxy = new Proxy(
    {},
    { get: (_t, m: string) => (...a: any[]) => call(m, a) },
  );
  w.go = { main: { App: appProxy } };

  // 事件：SSE -> EventsOn 回调
  const listeners: Record<string, Array<(...a: any[]) => void>> = {};
  const es = new EventSource("/api/events");
  const onFrame = (name: string) => (e: MessageEvent) => {
    let payload: any = undefined;
    try {
      payload = JSON.parse(e.data);
    } catch {
      payload = e.data;
    }
    (listeners[name] || []).forEach((cb) => cb(payload));
  };
  // 与后端 SSE event 名一致（config/stats/system/dict 等 Wails 事件名常量）
  ["config", "stats", "system", "dict", "update:available", "update:progress",
   "update:done", "update:error", "navigate", "navigate-addword", "protocol-import",
  ].forEach((name) => es.addEventListener(name, onFrame(name)));

  w.runtime = {
    EventsOn: (name: string, cb: (...a: any[]) => void) => {
      (listeners[name] ||= []).push(cb);
    },
    EventsOff: (name: string) => {
      delete listeners[name];
    },
    EventsOnce: (name: string, cb: (...a: any[]) => void) => {
      const once = (...a: any[]) => {
        cb(...a);
        delete listeners[name];
      };
      (listeners[name] ||= []).push(once);
    },
    // 无意义的窗口/退出操作在 Web 模式 no-op
    Quit: () => {},
    WindowShow: () => {},
    WindowHide: () => {},
    BrowserOpenURL: (url: string) => window.open(url, "_blank"),
  };

  w.__WEB_MODE__ = true;
}
```

> 执行注意：SSE `event` 名列表需与后端 Task 4 实际写出的事件名一致（后端目前只桥接 config/stats/system/dict 四类；update:* 等若后端未经 SSE 推送则前端监听无害但收不到——首版以四类配置/词库事件为准，其余可后续补）。

- [ ] **Step 2：main.ts 安装 shim（早于 createApp().mount）**

```ts
import { installWebShimIfNeeded } from "./lib/webShim";
installWebShimIfNeeded();
// ...existing createApp(App).mount("#app")
```

- [ ] **Step 3：类型检查 + 手动验证**

Run: `cd wind_setting/frontend && pnpm run build && cd ../..`
重新构建 exe，`--web` 下验证：通用页能读配置、改一项保存生效（重开浏览器确认持久化）、方案切换、外观页主题预览显示。

- [ ] **Step 4：格式化 + commit**

```bash
cd wind_setting/frontend && <前端格式化命令> && cd ../..
git add wind_setting/frontend/src/lib/webShim.ts wind_setting/frontend/src/main.ts
git commit -m "feat(setting): 前端 web 模式 Proxy/SSE shim"
```

---

## Task 10：各页禁用并提示（按 __WEB_MODE__）

**Files:** Modify `pages/AppearancePage.vue`、`AdvancedPage.vue`、`DictionaryPage.vue`、`GeneralPage.vue` 等

> 定位依据：依赖 Wails runtime（dialog/原生 shell/协议/在线主题服务）的功能块需禁用。这些通过各页 `import { ... } from "../api/wails"` 暴露（如 `openThemesFolder`、`setProtocolRegistered`、`importUserDict`）。执行前 Read 各页 import 段与模板。

- [ ] **Step 1：禁用提示通用件**

新建 `frontend/src/lib/webEnv.ts`：
```ts
export const DESKTOP_ONLY_HINT = "web 版暂不支持此功能，请使用桌面版";
export const isWebMode = (): boolean => !!(window as any).__WEB_MODE__;
```

- [ ] **Step 2：逐页禁用原生依赖功能块**

对以下控件加 `:disabled="isWebMode()"` + 悬停/行内提示 `DESKTOP_ONLY_HINT`：
- 外观页：在线主题编辑服务开关、删除主题、打开主题文件夹、导入主题（保留主题选择/预览，走反射网关正常）
- 高级页：协议注册、备份/还原、更改数据目录、打开日志/配置文件夹、性能/内存诊断导出
- 词库页：导入/导出（文件对话框）按钮（保留增删改查浏览）
- 通用页：方案高级子配置中依赖原生的项（方案选择/切换正常）

> 注意：方案 D 下这些页的非原生功能本就透明可用，故是**控件级**禁用，不是整页禁用。词库/统计页**不再整页禁用**，仅禁用其文件对话框类操作。

- [ ] **Step 3：类型检查 + 手动验证**

Run: `cd wind_setting/frontend && pnpm run build && cd ../..`
`--web` 下逐页确认：原生功能按钮置灰提示，其余功能可用；改配置后若涉及事件刷新，界面同步（验证 SSE）。

- [ ] **Step 4：格式化 + commit**

```bash
cd wind_setting/frontend && <前端格式化命令> && cd ../..
git add wind_setting/frontend/src/lib/webEnv.ts wind_setting/frontend/src/pages/
git commit -m "feat(setting): web 模式禁用原生依赖功能块并提示"
```

---

## Task 11：端到端验证 + AGENTS.md 同步

- [ ] **Step 1：Go 全测 + 构建** — Run: `cd wind_setting && go test ./... && go build ./... && go fmt ./... && cd ..` → 全 PASS
- [ ] **Step 2：前端构建** — Run: `cd wind_setting/frontend && pnpm run build && cd ../..` → 通过
- [ ] **Step 3：真实环境冒烟**
  - 正常机器：双击 exe 仍 GUI；`--web` 浏览器模式可用
  - 无 WebView2（或临时令 `isWebView2Installed` 返 false）：双击自动 Web + 开浏览器 + 原生提示
  - `--gui` 强制桌面有效
  - 验证项：配置读写/保存、方案切换重载、主题预览、事件刷新（改配置界面同步）、原生功能禁用提示
- [ ] **Step 4：更新 AGENTS.md**：`wind_setting/AGENTS.md`（新增 web_server/run_mode/webview_detect、`/api/call`+`/api/events`）、`frontend/src/lib/AGENTS.md`（webShim/webEnv）；运行 `scripts/lint_agents_md.ps1`
- [ ] **Step 5：commit**

```bash
git add wind_setting/AGENTS.md wind_setting/frontend/src/lib/AGENTS.md
git commit -m "docs(setting): 同步 web 模式 AGENTS.md"
```

---

## Self-Review 记录

- **spec 覆盖**：第 3 架构→Task 1/2/3；第 4 入口→Task 5/6/8；第 5 后端（call/events/static）→Task 1-4；第 6 前端 shim→Task 9；第 7 启动初始化→Task 7；第 8 禁用→Task 10；第 9 安全→Task 2/9（反射仅导出方法、127.0.0.1）；第 11 测试→各 Task + Task 11。
- **占位符检查**：Task 2 测试里 `strings_NewReader` 已标注为 `strings.NewReader` 的书写提示；`<前端格式化命令>` 标注执行时确认；其余无 TBD。
- **执行时需 Read 核实点（已显式标注）**：`wailsjs/go/main/App.js` 是否转发到 `window.go`（Task 9）；`rpcapi.EventMessage/EventType*/WailsEvent*` 符号（Task 4，照 app.go 已用）；被反射方法对 `nil`/占位 ctx 的容错（Task 7）；各页 import 与模板（Task 10）。
- **类型一致性**：`webServer`、`callReflect`、`callRequest/callResponse`、`writeCall`、`muxWithStatic`、`Start/Stop/url`、`resolveRunMode/runMode`、`runWebMode`、`isWebView2Installed`、`installWebShimIfNeeded`、`isWebMode/DESKTOP_ONLY_HINT` 全计划一致。
- **相对原 REST 方案的优势**：后端从「逐个 REST 端点」收敛为「单个反射网关」；前端从「settings.ts 平行封装 + 各页改造」收敛为「一个 shim + 控件级禁用」；新增功能零网关改动。
