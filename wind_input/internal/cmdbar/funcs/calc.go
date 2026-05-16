package funcs

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/huanfeng/wind_input/internal/cmdbar"
)

// §3.3 calc + num. 全部 Pure=true 且 Deterministic=true。
func calcFuncs() []cmdbar.FuncSpec {
	c := cmdbar.CategoryCalc
	return []cmdbar.FuncSpec{
		{Name: "calc", Category: c, MinArgs: 1, MaxArgs: 1, Pure: true, Deterministic: true,
			Description: "数学表达式求值 (支持 + - * / % 与括号; 空输入静默返回空)",
			ExampleSrc:  `calc(tail(code, 2))`, Eval: fnCalc},
		{Name: "num", Category: c, MinArgs: 2, MaxArgs: 2, Pure: true, Deterministic: true,
			Description: "进制转换 (2/8/10/16); num('0xff', 10) → '255'",
			ExampleSrc:  `num("0xff", 10)`, Eval: fnNum},
	}
}

func fnCalc(_ cmdbar.EvalContext, args []string) (string, error) {
	// 空输入或纯空白静默返回, 不构成错误。
	// 这样用户在用 `calc($input)` 这类模板时, 编码尚为空时也不会刷错误候选。
	if strings.TrimSpace(args[0]) == "" {
		return "", nil
	}
	v, err := evalArith(args[0])
	if err != nil {
		return "", fmt.Errorf("calc: %w", err)
	}
	return formatNumber(v), nil
}

func fnNum(_ cmdbar.EvalContext, args []string) (string, error) {
	s := strings.TrimSpace(args[0])
	base, err := parseArgInt(args[1])
	if err != nil {
		return "", fmt.Errorf("num: %w", err)
	}
	if base != 2 && base != 8 && base != 10 && base != 16 {
		return "", fmt.Errorf("num: unsupported base %d", base)
	}
	// Parse the source allowing standard Go prefixes (0x/0o/0b) so the
	// caller doesn't have to strip them manually. Output uses the
	// requested base without any prefix.
	v, err := strconv.ParseInt(s, 0, 64)
	if err != nil {
		// Try unsigned for very large hex literals.
		uv, uerr := strconv.ParseUint(s, 0, 64)
		if uerr != nil {
			return "", fmt.Errorf("num: %w", err)
		}
		return strconv.FormatUint(uv, base), nil
	}
	return strconv.FormatInt(v, base), nil
}

// formatNumber emits f without a trailing ".0" for integral values.
func formatNumber(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return strconv.FormatFloat(f, 'g', -1, 64)
	}
	if f == math.Trunc(f) && math.Abs(f) < 1e16 {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// evalArith parses and evaluates a small arithmetic grammar:
//
//	expr   = term (("+" | "-") term)*
//	term   = unary (("*" | "/" | "%") unary)*
//	unary  = ("+" | "-")? primary
//	primary= NUMBER | "(" expr ")"
//
// This is a recursive-descent (Pratt-style) evaluator with no dynamic
// allocation beyond what the tokeniser produces. It deliberately does
// not pull in go/parser to keep the dependency surface minimal.
func evalArith(src string) (float64, error) {
	p := &arithParser{src: src}
	p.next()
	v, err := p.parseExpr()
	if err != nil {
		return 0, err
	}
	if p.tok != tkEnd {
		return 0, fmt.Errorf("unexpected character %q at offset %d", p.lex, p.pos-len(p.lex))
	}
	return v, nil
}

type arithTok int

const (
	tkEnd arithTok = iota
	tkNum
	tkPlus
	tkMinus
	tkStar
	tkSlash
	tkPercent
	tkLP
	tkRP
)

type arithParser struct {
	src string
	pos int
	tok arithTok
	num float64
	lex string // current token text (for error messages)
}

func (p *arithParser) next() {
	for p.pos < len(p.src) && unicode.IsSpace(rune(p.src[p.pos])) {
		p.pos++
	}
	if p.pos >= len(p.src) {
		p.tok = tkEnd
		p.lex = ""
		return
	}
	c := p.src[p.pos]
	switch c {
	case '+':
		p.tok, p.lex = tkPlus, "+"
		p.pos++
	case '-':
		p.tok, p.lex = tkMinus, "-"
		p.pos++
	case '*':
		p.tok, p.lex = tkStar, "*"
		p.pos++
	case '/':
		p.tok, p.lex = tkSlash, "/"
		p.pos++
	case '%':
		p.tok, p.lex = tkPercent, "%"
		p.pos++
	case '(':
		p.tok, p.lex = tkLP, "("
		p.pos++
	case ')':
		p.tok, p.lex = tkRP, ")"
		p.pos++
	default:
		if (c >= '0' && c <= '9') || c == '.' {
			start := p.pos
			for p.pos < len(p.src) {
				ch := p.src[p.pos]
				if (ch >= '0' && ch <= '9') || ch == '.' {
					p.pos++
					continue
				}
				break
			}
			lex := p.src[start:p.pos]
			f, err := strconv.ParseFloat(lex, 64)
			if err != nil {
				p.tok = tkEnd
				p.lex = lex
				return
			}
			p.tok = tkNum
			p.num = f
			p.lex = lex
			return
		}
		// Unknown character: leave for the parser to report.
		p.tok = tkEnd
		p.lex = string(c)
		p.pos++
	}
}

func (p *arithParser) parseExpr() (float64, error) {
	lhs, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for p.tok == tkPlus || p.tok == tkMinus {
		op := p.tok
		p.next()
		rhs, err := p.parseTerm()
		if err != nil {
			return 0, err
		}
		if op == tkPlus {
			lhs += rhs
		} else {
			lhs -= rhs
		}
	}
	return lhs, nil
}

func (p *arithParser) parseTerm() (float64, error) {
	lhs, err := p.parseUnary()
	if err != nil {
		return 0, err
	}
	for p.tok == tkStar || p.tok == tkSlash || p.tok == tkPercent {
		op := p.tok
		p.next()
		rhs, err := p.parseUnary()
		if err != nil {
			return 0, err
		}
		switch op {
		case tkStar:
			lhs *= rhs
		case tkSlash:
			if rhs == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			lhs /= rhs
		case tkPercent:
			if rhs == 0 {
				return 0, fmt.Errorf("modulo by zero")
			}
			lhs = math.Mod(lhs, rhs)
		}
	}
	return lhs, nil
}

func (p *arithParser) parseUnary() (float64, error) {
	switch p.tok {
	case tkPlus:
		p.next()
		return p.parseUnary()
	case tkMinus:
		p.next()
		v, err := p.parseUnary()
		if err != nil {
			return 0, err
		}
		return -v, nil
	}
	return p.parsePrimary()
}

func (p *arithParser) parsePrimary() (float64, error) {
	switch p.tok {
	case tkNum:
		v := p.num
		p.next()
		return v, nil
	case tkLP:
		p.next()
		v, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		if p.tok != tkRP {
			return 0, fmt.Errorf("expected ')'")
		}
		p.next()
		return v, nil
	case tkEnd:
		return 0, fmt.Errorf("unexpected end of expression")
	}
	return 0, fmt.Errorf("unexpected token %q", p.lex)
}
