package parser

import (
	"testing"
)

func TestLexer_BasicTokens(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []TokenKind
	}{
		{"empty", "", []TokenKind{tkEOF}},
		{"paren", "()", []TokenKind{tkLParen, tkRParen, tkEOF}},
		{"comma", ",", []TokenKind{tkComma, tkEOF}},
		{"dot", ".", []TokenKind{tkDot, tkEOF}},
		{"ident", "foo", []TokenKind{tkIdent, tkEOF}},
		{"call", "foo(1, 2)", []TokenKind{tkIdent, tkLParen, tkNumber, tkComma, tkNumber, tkRParen, tkEOF}},
		{"ns_call", "clip.copy()", []TokenKind{tkIdent, tkDot, tkIdent, tkLParen, tkRParen, tkEOF}},
		{"string", `"hi"`, []TokenKind{tkString, tkEOF}},
		{"neg_num", "-1.5", []TokenKind{tkNumber, tkEOF}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			toks, err := NewLexer(c.src).Tokenize()
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if len(toks) != len(c.want) {
				t.Fatalf("want %d tokens, got %d: %+v", len(c.want), len(toks), toks)
			}
			for i, k := range c.want {
				if toks[i].Kind != k {
					t.Fatalf("token %d: want %v got %v (%q)", i, k, toks[i].Kind, toks[i].Lexeme)
				}
			}
		})
	}
}

func TestLexer_StringInterp(t *testing.T) {
	toks, err := NewLexer(`"hello {name}!"`).Tokenize()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if toks[0].Kind != tkString {
		t.Fatalf("want string, got %v", toks[0].Kind)
	}
	parts := toks[0].Parts
	if len(parts) != 3 {
		t.Fatalf("want 3 parts, got %d: %+v", len(parts), parts)
	}
	if parts[0].IsInterp || parts[0].Lit != "hello " {
		t.Errorf("part0 = %+v", parts[0])
	}
	if !parts[1].IsInterp || parts[1].Interp != "name" {
		t.Errorf("part1 = %+v", parts[1])
	}
	if parts[2].IsInterp || parts[2].Lit != "!" {
		t.Errorf("part2 = %+v", parts[2])
	}
}

func TestLexer_StringEscapes(t *testing.T) {
	toks, err := NewLexer(`"a\"b\\c\{d\}"`).Tokenize()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(toks[0].Parts) != 1 {
		t.Fatalf("want 1 part, got %+v", toks[0].Parts)
	}
	if got := toks[0].Parts[0].Lit; got != `a"b\c{d}` {
		t.Errorf("decoded = %q", got)
	}
}

func TestLexer_Errors(t *testing.T) {
	cases := []string{
		`"unterminated`,
		`"bad {nested"`,
		`"oops}"`,
		`@`,
	}
	for _, src := range cases {
		t.Run(src, func(t *testing.T) {
			if _, err := NewLexer(src).Tokenize(); err == nil {
				t.Errorf("want error for %q", src)
			}
		})
	}
}
