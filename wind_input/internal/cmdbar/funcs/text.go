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

// §3.2 文本处理函数. 全部 Pure=true 且 Deterministic=true (无外部状态依赖)。
// t2s/s2t/pinyin 当前是占位 stub (原样返回, 见各 fn 实现的 TODO 注释)。
func textFuncs() []cmdbar.FuncSpec {
	c := cmdbar.CategoryText
	return []cmdbar.FuncSpec{
		{Name: "len", Category: c, MinArgs: 1, MaxArgs: 1, Pure: true, Deterministic: true,
			Description: "字符串字符数 (按 rune)", ExampleSrc: `len(last())`, Eval: fnLen},
		{Name: "upper", Category: c, MinArgs: 1, MaxArgs: 1, Pure: true, Deterministic: true,
			Description: "转大写", ExampleSrc: `upper("abc")`, Eval: fnUpper},
		{Name: "lower", Category: c, MinArgs: 1, MaxArgs: 1, Pure: true, Deterministic: true,
			Description: "转小写", ExampleSrc: `lower("ABC")`, Eval: fnLower},
		{Name: "trim", Category: c, MinArgs: 1, MaxArgs: 2, Pure: true, Deterministic: true,
			Description: "去首尾空白; trim(s, chars) 去指定字符",
			ExampleSrc:  `trim(last())`, Eval: fnTrim},
		{Name: "sub", Category: c, MinArgs: 2, MaxArgs: 3, Pure: true, Deterministic: true,
			Description: "切片, 索引 1 起, 支持负数; sub(s, start, end) 双闭区间",
			ExampleSrc:  `sub(code, 2)`, Eval: fnSub},
		{Name: "replace", Category: c, MinArgs: 3, MaxArgs: 3, Pure: true, Deterministic: true,
			Description: "字面替换", ExampleSrc: `replace(last(), "a", "b")`, Eval: fnReplace},
		{Name: "regex", Category: c, MinArgs: 3, MaxArgs: 3, Pure: true, Deterministic: true,
			Description: "正则替换 (Go RE2 语法)", ExampleSrc: `regex(last(), "\\d+", "N")`, Eval: fnRegex},
		{Name: "split", Category: c, MinArgs: 3, MaxArgs: 3, Pure: true, Deterministic: true,
			Description: "按 sep 拆分, 取第 n 段 (1 起, 支持负数)",
			ExampleSrc:  `split(last(), ",", 1)`, Eval: fnSplit},
		{Name: "concat", Category: c, MinArgs: 0, MaxArgs: -1, Pure: true, Deterministic: true,
			Description: "字符串拼接", ExampleSrc: `concat(last(), " ", clip())`, Eval: fnConcat},
		{Name: "reverse", Category: c, MinArgs: 1, MaxArgs: 1, Pure: true, Deterministic: true,
			Description: "反转字符串 (按 rune)", ExampleSrc: `reverse("abc")`, Eval: fnReverse},
		{Name: "t2s", Category: c, MinArgs: 1, MaxArgs: 1, Pure: true, Deterministic: true,
			Description: "(stub) 繁→简转换; 暂占位原样返回", ExampleSrc: `t2s(last())`, Eval: fnT2s},
		{Name: "s2t", Category: c, MinArgs: 1, MaxArgs: 1, Pure: true, Deterministic: true,
			Description: "(stub) 简→繁转换; 暂占位原样返回", ExampleSrc: `s2t(last())`, Eval: fnS2t},
		{Name: "pinyin", Category: c, MinArgs: 1, MaxArgs: 1, Pure: true, Deterministic: true,
			Description: "(stub) 汉字转拼音 (空格分隔); 暂占位原样返回", ExampleSrc: `pinyin(last())`, Eval: fnPinyin},
		{Name: "url", Category: c, MinArgs: 1, MaxArgs: 1, Pure: true, Deterministic: true,
			Description: "URL 编码 (component)", ExampleSrc: `url(last())`, Eval: fnURL},
		{Name: "html", Category: c, MinArgs: 1, MaxArgs: 1, Pure: true, Deterministic: true,
			Description: "HTML 实体编码", ExampleSrc: `html(last())`, Eval: fnHTML},
		{Name: "json", Category: c, MinArgs: 1, MaxArgs: 1, Pure: true, Deterministic: true,
			Description: "JSON 字符串字面量化 (含外层引号)", ExampleSrc: `json(last())`, Eval: fnJSON},
		{Name: "base64", Category: c, MinArgs: 1, MaxArgs: 1, Pure: true, Deterministic: true,
			Description: "Base64 编码", ExampleSrc: `base64(last())`, Eval: fnBase64},
		{Name: "default", Category: c, MinArgs: 2, MaxArgs: 2, Pure: true, Deterministic: true,
			Description: "s 为空时返回 fallback", ExampleSrc: `default(last(), "(empty)")`, Eval: fnDefault},
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
