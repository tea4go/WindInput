package funcs

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/huanfeng/wind_input/internal/cmdbar"
)

// §3.2 text-processing functions. All are pure.

func textFuncs() []cmdbar.FuncSpec {
	return []cmdbar.FuncSpec{
		{Name: "len", MinArgs: 1, MaxArgs: 1, Pure: true, Eval: fnLen},
		{Name: "upper", MinArgs: 1, MaxArgs: 1, Pure: true, Eval: fnUpper},
		{Name: "lower", MinArgs: 1, MaxArgs: 1, Pure: true, Eval: fnLower},
		{Name: "trim", MinArgs: 1, MaxArgs: 2, Pure: true, Eval: fnTrim},
		{Name: "sub", MinArgs: 2, MaxArgs: 3, Pure: true, Eval: fnSub},
		{Name: "replace", MinArgs: 3, MaxArgs: 3, Pure: true, Eval: fnReplace},
		{Name: "regex", MinArgs: 3, MaxArgs: 3, Pure: true, Eval: fnRegex},
		{Name: "split", MinArgs: 3, MaxArgs: 3, Pure: true, Eval: fnSplit},
		{Name: "concat", MinArgs: 0, MaxArgs: -1, Pure: true, Eval: fnConcat},
		{Name: "reverse", MinArgs: 1, MaxArgs: 1, Pure: true, Eval: fnReverse},
		{Name: "t2s", MinArgs: 1, MaxArgs: 1, Pure: true, Eval: fnT2s},
		{Name: "s2t", MinArgs: 1, MaxArgs: 1, Pure: true, Eval: fnS2t},
		{Name: "pinyin", MinArgs: 1, MaxArgs: 1, Pure: true, Eval: fnPinyin},
		{Name: "url", MinArgs: 1, MaxArgs: 1, Pure: true, Eval: fnURL},
		{Name: "html", MinArgs: 1, MaxArgs: 1, Pure: true, Eval: fnHTML},
		{Name: "json", MinArgs: 1, MaxArgs: 1, Pure: true, Eval: fnJSON},
		{Name: "base64", MinArgs: 1, MaxArgs: 1, Pure: true, Eval: fnBase64},
		{Name: "default", MinArgs: 2, MaxArgs: 2, Pure: true, Eval: fnDefault},
	}
}

func fnLen(_ cmdbar.EvalContext, args []string) (string, error) {
	return strconv.Itoa(utf8Len(args[0])), nil
}

func fnUpper(_ cmdbar.EvalContext, args []string) (string, error) {
	return strings.ToUpper(args[0]), nil
}

func fnLower(_ cmdbar.EvalContext, args []string) (string, error) {
	return strings.ToLower(args[0]), nil
}

func fnTrim(_ cmdbar.EvalContext, args []string) (string, error) {
	if len(args) == 1 {
		return strings.TrimSpace(args[0]), nil
	}
	return strings.Trim(args[0], args[1]), nil
}

// resolve1Based converts a 1-based, possibly negative index into a
// 0-based position for a slice of length n. Returns (-1, false) when
// the index is out of range (excluding the negative-from-end shorthand,
// which is allowed as long as it lands within [1, n]).
func resolve1Based(idx, n int) (int, bool) {
	if idx < 0 {
		idx = n + 1 + idx // -1 → n
	}
	if idx < 1 || idx > n {
		return -1, false
	}
	return idx - 1, true
}

func fnSub(_ cmdbar.EvalContext, args []string) (string, error) {
	rs := runeSlice(args[0])
	n := len(rs)
	start, err := parseArgInt(args[1])
	if err != nil {
		return "", fmt.Errorf("sub: %w", err)
	}
	s, ok := resolve1Based(start, n)
	if !ok {
		return "", nil
	}
	if len(args) == 2 {
		return string(rs[s:]), nil
	}
	end, err := parseArgInt(args[2])
	if err != nil {
		return "", fmt.Errorf("sub: %w", err)
	}
	e, ok := resolve1Based(end, n)
	if !ok {
		return "", nil
	}
	// end is inclusive 1-based per design; convert to exclusive bound.
	e++
	if e <= s {
		return "", nil
	}
	return string(rs[s:e]), nil
}

func fnReplace(_ cmdbar.EvalContext, args []string) (string, error) {
	return strings.ReplaceAll(args[0], args[1], args[2]), nil
}

func fnRegex(_ cmdbar.EvalContext, args []string) (string, error) {
	re, err := regexp.Compile(args[1])
	if err != nil {
		return "", fmt.Errorf("regex: %w", err)
	}
	return re.ReplaceAllString(args[0], args[2]), nil
}

func fnSplit(_ cmdbar.EvalContext, args []string) (string, error) {
	n, err := parseArgInt(args[2])
	if err != nil {
		return "", fmt.Errorf("split: %w", err)
	}
	parts := strings.Split(args[0], args[1])
	idx, ok := resolve1Based(n, len(parts))
	if !ok {
		return "", nil
	}
	return parts[idx], nil
}

func fnConcat(_ cmdbar.EvalContext, args []string) (string, error) {
	return strings.Join(args, ""), nil
}

func fnReverse(_ cmdbar.EvalContext, args []string) (string, error) {
	rs := runeSlice(args[0])
	for i, j := 0, len(rs)-1; i < j; i, j = i+1, j-1 {
		rs[i], rs[j] = rs[j], rs[i]
	}
	return string(rs), nil
}

// fnT2s / fnS2t / fnPinyin: P2 stubs. Real conversion tables will be
// wired in a later phase; today they pass through unchanged so that
// downstream expressions keep working.
func fnT2s(_ cmdbar.EvalContext, args []string) (string, error) {
	// TODO(cmdbar): plug in OpenCC / project's t2s mapping table.
	return args[0], nil
}
func fnS2t(_ cmdbar.EvalContext, args []string) (string, error) {
	// TODO(cmdbar): plug in OpenCC / project's s2t mapping table.
	return args[0], nil
}
func fnPinyin(_ cmdbar.EvalContext, args []string) (string, error) {
	// TODO(cmdbar): plug in pinyin conversion (likely via dict package).
	return args[0], nil
}

func fnURL(_ cmdbar.EvalContext, args []string) (string, error) {
	return url.QueryEscape(args[0]), nil
}

func fnHTML(_ cmdbar.EvalContext, args []string) (string, error) {
	return html.EscapeString(args[0]), nil
}

func fnJSON(_ cmdbar.EvalContext, args []string) (string, error) {
	b, err := json.Marshal(args[0])
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func fnBase64(_ cmdbar.EvalContext, args []string) (string, error) {
	return base64.StdEncoding.EncodeToString([]byte(args[0])), nil
}

func fnDefault(_ cmdbar.EvalContext, args []string) (string, error) {
	if args[0] == "" {
		return args[1], nil
	}
	return args[0], nil
}
