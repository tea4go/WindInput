package parser

import (
	"errors"
	"fmt"
	"strings"

	"github.com/huanfeng/wind_input/internal/cmdbar/ast"
)

// Parse parses a phrase source string into an ast.Phrase. The top-level
// classification rule is:
//
//  1. If the source contains a `$CC(` or `$CC1(` token at brace/paren
//     depth 0 (and not inside a string), parse it as a CommandPhrase.
//     `$CC1(` 与 `$CC(` 产出完全相同的 AST, 区别只在前缀搜索阶段:
//     `$CC(` 仅精确匹配, `$CC1(` 同时参与前缀展开 (见 dict.IsCmdbarExactOnly)。
//  2. Otherwise if the source contains an unescaped `{` at top level,
//     treat the whole source as a TemplatePhrase with an implicit
//     surrounding string literal.
//  3. Otherwise produce a LiteralPhrase carrying the raw text.
//
// Errors include the offending byte offset.
func Parse(src string) (ast.Phrase, error) {
	idx, openOff := findTopLevelCC(src)
	if idx >= 0 {
		// Slice before the $CC/$CC1 marker is treated as a literal prefix; we
		// require it to be empty or whitespace-only per the design (a
		// phrase is either literal / template / command — they don't
		// concatenate at top level).
		prefix := src[:idx]
		if strings.TrimSpace(prefix) != "" {
			return nil, fmt.Errorf("unexpected text before $CC at offset 0")
		}
		return parseCommandPhrase(src, idx, openOff)
	}
	if hasTopLevelBrace(src) {
		return parseTemplatePhrase(src)
	}
	return ast.LiteralPhrase{Text: src}, nil
}

// findTopLevelCC returns (startOff, openParenOff) for `$CC(` 或 `$CC1(` token
// in src that lies outside any string literal. startOff 指向 '$', openParenOff
// 指向 '(' 字符。未找到时返回 (-1, -1)。
//
// 检测顺序: 必须先匹配 `$CC1(` 再匹配 `$CC(`, 否则 `$CC1(` 会被吃成 `$CC(` + 残留 "1(".
func findTopLevelCC(src string) (int, int) {
	i := 0
	for i < len(src) {
		c := src[i]
		if c == '\\' && i+1 < len(src) {
			i += 2
			continue
		}
		if c == '"' || c == '\'' {
			// skip string
			q := c
			i++
			for i < len(src) {
				if src[i] == '\\' && i+1 < len(src) {
					i += 2
					continue
				}
				if src[i] == q {
					i++
					break
				}
				i++
			}
			continue
		}
		if c == '$' && i+4 < len(src) && src[i+1] == 'C' && src[i+2] == 'C' && src[i+3] == '1' && src[i+4] == '(' {
			return i, i + 4
		}
		if c == '$' && i+3 < len(src) && src[i+1] == 'C' && src[i+2] == 'C' && src[i+3] == '(' {
			return i, i + 3
		}
		i++
	}
	return -1, -1
}

// hasTopLevelBrace reports whether src contains an unescaped `{` that is
// not inside a string literal.
func hasTopLevelBrace(src string) bool {
	i := 0
	for i < len(src) {
		c := src[i]
		if c == '\\' && i+1 < len(src) {
			i += 2
			continue
		}
		if c == '{' {
			return true
		}
		i++
	}
	return false
}

func parseCommandPhrase(src string, idx, open int) (ast.Phrase, error) {
	// idx points at '$'; `open` 指向 '(' (对 `$CC(` 是 idx+3, 对 `$CC1(` 是 idx+4)。
	// Build a sub-source representing "$CC(...)" / "$CC1(...)" and tokenize the
	// inner expression list. We reuse the expression lexer by scanning from
	// open+1 until the matching ')'.
	if open >= len(src) || src[open] != '(' {
		return nil, fmt.Errorf("expected '(' after $CC at offset %d", idx)
	}
	// Find matching ')' at depth 0 ignoring strings.
	end, err := findMatchingParen(src, open)
	if err != nil {
		return nil, err
	}
	inner := src[open+1 : end]
	// Trailing characters after the closing paren must be empty/whitespace.
	if strings.TrimSpace(src[end+1:]) != "" {
		return nil, fmt.Errorf("unexpected text after $CC(...) at offset %d", end+1)
	}
	lex := NewLexer(inner)
	toks, err := lex.Tokenize()
	if err != nil {
		return nil, fmt.Errorf("$CC body: %w", err)
	}
	p := &parser{tokens: toks, src: inner, baseOff: open + 1}
	args, err := p.parseExprList()
	if err != nil {
		return nil, err
	}
	if p.peek().Kind != tkEOF {
		return nil, p.errf(p.peek().Offset, "unexpected token %q", p.peek().Lexeme)
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("$CC requires a display expression at offset %d", idx)
	}
	return ast.CommandPhrase{Display: args[0], Actions: args[1:]}, nil
}

func findMatchingParen(src string, openIdx int) (int, error) {
	depth := 1
	i := openIdx + 1
	for i < len(src) {
		c := src[i]
		if c == '\\' && i+1 < len(src) {
			i += 2
			continue
		}
		if c == '"' || c == '\'' {
			q := c
			i++
			for i < len(src) {
				if src[i] == '\\' && i+1 < len(src) {
					i += 2
					continue
				}
				if src[i] == q {
					i++
					break
				}
				i++
			}
			continue
		}
		if c == '(' {
			depth++
		} else if c == ')' {
			depth--
			if depth == 0 {
				return i, nil
			}
		}
		i++
	}
	return -1, fmt.Errorf("unclosed '(' at offset %d", openIdx)
}

func parseTemplatePhrase(src string) (ast.Phrase, error) {
	// Treat the entire src as the body of an implicit double-quoted
	// string, sharing the same string-decoding rules: literal text and
	// `{expr}` interpolations. We hand-build a stringPart slice without
	// going through the lexer (since there are no outer quotes).
	var parts []stringPart
	var lit []byte
	flush := func(off int) {
		if len(lit) > 0 {
			parts = append(parts, stringPart{Lit: string(lit), Offset: off})
			lit = lit[:0]
		}
	}
	i := 0
	for i < len(src) {
		c := src[i]
		if c == '\\' && i+1 < len(src) {
			next := src[i+1]
			switch next {
			case '\\', '{', '}', '(', ')', '"', '\'':
				lit = append(lit, next)
			case 'n':
				lit = append(lit, '\n')
			case 't':
				lit = append(lit, '\t')
			case 'r':
				lit = append(lit, '\r')
			default:
				return nil, fmt.Errorf("unknown escape \\%c at offset %d", next, i)
			}
			i += 2
			continue
		}
		if c == '{' {
			flush(i)
			interpStart := i + 1
			depth := 1
			p := interpStart
			inInner := byte(0)
			for p < len(src) && depth > 0 {
				ch := src[p]
				if inInner != 0 {
					if ch == '\\' && p+1 < len(src) {
						p += 2
						continue
					}
					if ch == inInner {
						inInner = 0
					}
					p++
					continue
				}
				if ch == '"' || ch == '\'' {
					inInner = ch
					p++
					continue
				}
				if ch == '{' {
					depth++
				} else if ch == '}' {
					depth--
					if depth == 0 {
						break
					}
				}
				p++
			}
			if depth != 0 {
				return nil, fmt.Errorf("unclosed '{' at offset %d", i)
			}
			parts = append(parts, stringPart{IsInterp: true, Interp: src[interpStart:p], Offset: i})
			i = p + 1
			continue
		}
		if c == '}' {
			return nil, fmt.Errorf("unmatched '}' at offset %d", i)
		}
		lit = append(lit, c)
		i++
	}
	flush(len(src))
	str, err := buildStringLit(parts)
	if err != nil {
		return nil, err
	}
	return ast.TemplatePhrase{Expr: str}, nil
}

// buildStringLit converts a parts slice into an ast.StringLit, parsing
// each interpolation body as an expression.
func buildStringLit(parts []stringPart) (ast.StringLit, error) {
	out := make([]ast.Part, 0, len(parts))
	for _, p := range parts {
		if !p.IsInterp {
			out = append(out, ast.LiteralPart{Text: p.Lit})
			continue
		}
		body := p.Interp
		lex := NewLexer(body)
		toks, err := lex.Tokenize()
		if err != nil {
			return ast.StringLit{}, fmt.Errorf("interpolation at offset %d: %w", p.Offset, err)
		}
		pr := &parser{tokens: toks, src: body, baseOff: p.Offset + 1}
		expr, err := pr.parseExpr()
		if err != nil {
			return ast.StringLit{}, err
		}
		if pr.peek().Kind != tkEOF {
			return ast.StringLit{}, pr.errf(pr.peek().Offset, "unexpected token %q in interpolation", pr.peek().Lexeme)
		}
		out = append(out, ast.InterpPart{Expr: expr})
	}
	return ast.StringLit{Parts: out}, nil
}

// parser is a tiny recursive-descent parser over the token stream
// produced for an expression-level slice of source. baseOff is the byte
// offset of token offset 0 within the original phrase source, so error
// messages can map back to the user-visible position.
type parser struct {
	tokens  []Token
	pos     int
	src     string
	baseOff int
}

func (p *parser) peek() Token { return p.tokens[p.pos] }
func (p *parser) bump() Token {
	t := p.tokens[p.pos]
	p.pos++
	return t
}

func (p *parser) errf(off int, format string, args ...any) error {
	return fmt.Errorf("%s at offset %d", fmt.Sprintf(format, args...), p.baseOff+off)
}

// parseExprList parses zero or more comma-separated expressions.
func (p *parser) parseExprList() ([]ast.Expr, error) {
	if p.peek().Kind == tkEOF || p.peek().Kind == tkRParen {
		return nil, nil
	}
	var out []ast.Expr
	for {
		e, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		out = append(out, e)
		if p.peek().Kind != tkComma {
			break
		}
		p.bump() // ,
		if p.peek().Kind == tkEOF || p.peek().Kind == tkRParen {
			return nil, p.errf(p.peek().Offset, "trailing comma")
		}
	}
	return out, nil
}

func (p *parser) parseExpr() (ast.Expr, error) {
	t := p.peek()
	switch t.Kind {
	case tkNumber:
		p.bump()
		return ast.NumberLit{Value: t.Number, Raw: t.Lexeme}, nil
	case tkString:
		p.bump()
		return buildStringLit(t.Parts)
	case tkIdent:
		p.bump()
		name := t.Lexeme
		// Optional namespace: ident "." ident.
		if p.peek().Kind == tkDot {
			p.bump()
			if p.peek().Kind != tkIdent {
				return nil, p.errf(p.peek().Offset, "expected identifier after '.'")
			}
			second := p.bump()
			name = name + "." + second.Lexeme
			if p.peek().Kind == tkDot {
				return nil, p.errf(p.peek().Offset, "function name may have at most one '.'")
			}
		}
		// Call form?
		if p.peek().Kind == tkLParen {
			p.bump()
			var args []ast.Expr
			if p.peek().Kind != tkRParen {
				var err error
				args, err = p.parseExprList()
				if err != nil {
					return nil, err
				}
			}
			if p.peek().Kind != tkRParen {
				return nil, p.errf(p.peek().Offset, "expected ')' to close call to %s", name)
			}
			p.bump()
			return ast.Call{Name: name, Args: args}, nil
		}
		// Bare identifier. If a namespace was provided, demand parens.
		if strings.Contains(name, ".") {
			return nil, p.errf(t.Offset, "namespaced function %q must be called with ()", name)
		}
		return ast.Ident{Name: name}, nil
	case tkLParen:
		p.bump()
		e, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if p.peek().Kind != tkRParen {
			return nil, p.errf(p.peek().Offset, "expected ')'")
		}
		p.bump()
		return e, nil
	case tkEOF:
		return nil, p.errf(t.Offset, "unexpected end of input")
	}
	return nil, p.errf(t.Offset, "unexpected token %q", t.Lexeme)
}

// Errors exported for callers that need to distinguish parse failure.
var ErrEmpty = errors.New("empty phrase")
