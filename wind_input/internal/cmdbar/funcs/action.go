package funcs

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/huanfeng/wind_input/internal/cmdbar"
)

// §3.4 action functions. They are registered separately from the pure
// §3.1-§3.3 functions because they require runtime Services injection
// (see cmdbar.Services). Use cmdbar.RegisterActions to install them
// onto a Registry; the funcs package's init() does NOT register them
// implicitly.

// actionFuncs returns the FuncSpec list for the action MVP. Each
// Eval handler resolves its required service from ctx.Services() at
// invocation time. If the service is nil it returns
// cmdbar.ErrServiceUnavailable so callers can degrade.
//
// 注意: `type` 不在此列表内。它由 eval.Evaluate 在解析 CommandPhrase.Actions
// 时显式拦截构造为 ActionText (P5), 由 coordinator 走 TSF InsertText 通路
// 落字, 不再经过 registry.Lookup; registry 中的 type stub 仅供 arity 元信息,
// 不会真正被 Evaluate 调用。
func actionFuncs() []cmdbar.FuncSpec {
	return []cmdbar.FuncSpec{
		{Name: "open", MinArgs: 1, MaxArgs: 1, Pure: false, Eval: fnOpen},
		{Name: "run", MinArgs: 1, MaxArgs: -1, Pure: false, Eval: fnRun},
		{Name: "shell", MinArgs: 1, MaxArgs: 2, Pure: false, Eval: fnShell},
		{Name: "key.tap", MinArgs: 1, MaxArgs: 1, Pure: false, Eval: fnKeyTap},
		{Name: "key.seq", MinArgs: 1, MaxArgs: -1, Pure: false, Eval: fnKeySeq},
		{Name: "clip.copy", MinArgs: 1, MaxArgs: 1, Pure: false, Eval: fnClipCopy},
		{Name: "clip.paste", MinArgs: 0, MaxArgs: 0, Pure: false, Eval: fnClipPaste},
		{Name: "search", MinArgs: 2, MaxArgs: 2, Pure: false, Eval: fnSearch},
	}
}

// svcs fetches the Services bundle from ctx, or returns
// ErrServiceUnavailable if none is attached.
func svcs(ctx cmdbar.EvalContext) (*cmdbar.Services, error) {
	s := ctx.Services()
	if s == nil {
		return nil, cmdbar.ErrServiceUnavailable
	}
	return s, nil
}

func fnOpen(ctx cmdbar.EvalContext, args []string) (string, error) {
	s, err := svcs(ctx)
	if err != nil {
		return "", err
	}
	if s.Open == nil {
		return "", fmt.Errorf("open: %w", cmdbar.ErrServiceUnavailable)
	}
	if err := s.Open.Open(args[0]); err != nil {
		return "", fmt.Errorf("open: %w", err)
	}
	return "", nil
}

func fnRun(ctx cmdbar.EvalContext, args []string) (string, error) {
	s, err := svcs(ctx)
	if err != nil {
		return "", err
	}
	if s.Proc == nil {
		return "", fmt.Errorf("run: %w", cmdbar.ErrServiceUnavailable)
	}
	cmd := args[0]
	var rest []string
	if len(args) > 1 {
		rest = args[1:]
	}
	if err := s.Proc.Run(cmd, rest...); err != nil {
		return "", fmt.Errorf("run: %w", err)
	}
	return "", nil
}

func fnShell(ctx cmdbar.EvalContext, args []string) (string, error) {
	s, err := svcs(ctx)
	if err != nil {
		return "", err
	}
	if s.Proc == nil {
		return "", fmt.Errorf("shell: %w", cmdbar.ErrServiceUnavailable)
	}
	// 1 参形式: 保留旧 Shell() 调用 (兼容现有 mock 与 cmdbar-repl 行为)。
	if len(args) == 1 {
		if err := s.Proc.Shell(args[0]); err != nil {
			return "", fmt.Errorf("shell: %w", err)
		}
		return "", nil
	}
	// 2 参形式: 第二参为 flag 字符串, 逗号拆分 (空段被 ShellEx 内部跳过)。
	var flags []string
	for _, part := range strings.Split(args[1], ",") {
		if t := strings.TrimSpace(part); t != "" {
			flags = append(flags, t)
		}
	}
	if err := s.Proc.ShellEx(args[0], flags); err != nil {
		return "", fmt.Errorf("shell: %w", err)
	}
	return "", nil
}

func fnKeyTap(ctx cmdbar.EvalContext, args []string) (string, error) {
	s, err := svcs(ctx)
	if err != nil {
		return "", err
	}
	if s.Keys == nil {
		return "", fmt.Errorf("key.tap: %w", cmdbar.ErrServiceUnavailable)
	}
	if err := s.Keys.Tap(args[0]); err != nil {
		return "", fmt.Errorf("key.tap: %w", err)
	}
	return "", nil
}

func fnKeySeq(ctx cmdbar.EvalContext, args []string) (string, error) {
	s, err := svcs(ctx)
	if err != nil {
		return "", err
	}
	if s.Keys == nil {
		return "", fmt.Errorf("key.seq: %w", cmdbar.ErrServiceUnavailable)
	}
	if err := s.Keys.Sequence(args...); err != nil {
		return "", fmt.Errorf("key.seq: %w", err)
	}
	return "", nil
}

func fnClipCopy(ctx cmdbar.EvalContext, args []string) (string, error) {
	s, err := svcs(ctx)
	if err != nil {
		return "", err
	}
	if s.Clip == nil {
		return "", fmt.Errorf("clip.copy: %w", cmdbar.ErrServiceUnavailable)
	}
	if err := s.Clip.SetText(args[0]); err != nil {
		return "", fmt.Errorf("clip.copy: %w", err)
	}
	return "", nil
}

func fnClipPaste(ctx cmdbar.EvalContext, args []string) (string, error) {
	s, err := svcs(ctx)
	if err != nil {
		return "", err
	}
	if s.Keys == nil {
		return "", fmt.Errorf("clip.paste: %w (keys)", cmdbar.ErrServiceUnavailable)
	}
	if err := s.Keys.Tap("Ctrl+V"); err != nil {
		return "", fmt.Errorf("clip.paste: %w", err)
	}
	return "", nil
}

// searchEngineURLs maps engine ids from §3.4 to their query-URL prefix.
// %s is replaced with the URL-encoded query.
var searchEngineURLs = map[string]string{
	"baidu":  "https://www.baidu.com/s?wd=",
	"bing":   "https://www.bing.com/search?q=",
	"google": "https://www.google.com/search?q=",
	"zdic":   "https://www.zdic.net/hans/",
}

func fnSearch(ctx cmdbar.EvalContext, args []string) (string, error) {
	s, err := svcs(ctx)
	if err != nil {
		return "", err
	}
	engine := strings.ToLower(strings.TrimSpace(args[0]))
	q := args[1]
	// If host provides a custom SearchEngine, prefer it.
	if s.Search != nil {
		if err := s.Search.Search(engine, q); err != nil {
			return "", fmt.Errorf("search: %w", err)
		}
		return "", nil
	}
	// Default: compose URL and forward to Open.
	prefix, ok := searchEngineURLs[engine]
	if !ok {
		return "", fmt.Errorf("search: unknown engine %q", engine)
	}
	if s.Open == nil {
		return "", fmt.Errorf("search: %w", cmdbar.ErrServiceUnavailable)
	}
	target := prefix + url.QueryEscape(q)
	if err := s.Open.Open(target); err != nil {
		return "", fmt.Errorf("search: %w", err)
	}
	return "", nil
}
