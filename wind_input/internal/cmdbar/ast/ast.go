// Package ast defines the abstract syntax tree node types used by the
// command bar parser. The grammar is documented in
// docs/design/2026-05-12-command-bar-design.md §2.
package ast

import (
	"strconv"
	"strings"
)

// Expr is the common interface for all expression-level AST nodes.
type Expr interface {
	exprNode()
	String() string
}

// Part is a fragment inside a StringLit. It is either a LiteralPart with
// raw text or an InterpPart wrapping a nested expression.
type Part interface {
	partNode()
	String() string
}

// LiteralPart is a raw text segment of a string literal.
type LiteralPart struct {
	Text string
}

func (LiteralPart) partNode() {}
func (p LiteralPart) String() string {
	// Re-escape special characters for round-trippable debug output.
	r := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		`{`, `\{`,
		`}`, `\}`,
	)
	return r.Replace(p.Text)
}

// InterpPart is a `{expr}` interpolation segment inside a string literal.
type InterpPart struct {
	Expr Expr
}

func (InterpPart) partNode() {}
func (p InterpPart) String() string {
	if p.Expr == nil {
		return "{}"
	}
	return "{" + p.Expr.String() + "}"
}

// StringLit is a double-quoted string literal, optionally containing
// `{expr}` interpolations.
type StringLit struct {
	Parts []Part
}

func (StringLit) exprNode() {}
func (s StringLit) String() string {
	var b strings.Builder
	b.WriteByte('"')
	for _, p := range s.Parts {
		b.WriteString(p.String())
	}
	b.WriteByte('"')
	return b.String()
}

// NumberLit is a numeric literal. Raw keeps the original lexeme so we can
// emit it back verbatim; Value is the parsed float.
type NumberLit struct {
	Value float64
	Raw   string
}

func (NumberLit) exprNode() {}
func (n NumberLit) String() string {
	if n.Raw != "" {
		return n.Raw
	}
	return strconv.FormatFloat(n.Value, 'f', -1, 64)
}

// Ident is a bare identifier reference. Per §2.4 a bare identifier is
// semantically equivalent to a zero-argument call.
type Ident struct {
	Name string
}

func (Ident) exprNode()        {}
func (i Ident) String() string { return i.Name }

// Call is a function invocation. Name may contain a single dot for a
// namespace prefix (e.g. "clip.copy").
type Call struct {
	Name string
	Args []Expr
}

func (Call) exprNode() {}
func (c Call) String() string {
	var b strings.Builder
	b.WriteString(c.Name)
	b.WriteByte('(')
	for i, a := range c.Args {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(a.String())
	}
	b.WriteByte(')')
	return b.String()
}

// Phrase is a top-level AST node. A phrase is one of LiteralPhrase,
// TemplatePhrase or CommandPhrase.
type Phrase interface {
	phraseNode()
	String() string
}

// LiteralPhrase is a phrase with no interpolation and no `$CC(...)`
// wrapper. It surfaces as plain text.
type LiteralPhrase struct {
	Text string
}

func (LiteralPhrase) phraseNode()      {}
func (p LiteralPhrase) String() string { return p.Text }

// TemplatePhrase is a phrase containing `{expr}` interpolations but no
// `$CC(...)` wrapper.
type TemplatePhrase struct {
	Expr Expr // always a StringLit holding the template body
}

func (TemplatePhrase) phraseNode() {}
func (p TemplatePhrase) String() string {
	if p.Expr == nil {
		return ""
	}
	return p.Expr.String()
}

// CommandPhrase represents a `$CC(display, action...)` invocation.
// Display is the candidate display expression; Actions is the (possibly
// empty) ordered list of side-effecting action expressions.
type CommandPhrase struct {
	Display Expr
	Actions []Expr
}

func (CommandPhrase) phraseNode() {}
func (p CommandPhrase) String() string {
	var b strings.Builder
	b.WriteString("$CC(")
	if p.Display != nil {
		b.WriteString(p.Display.String())
	}
	for _, a := range p.Actions {
		b.WriteString(", ")
		b.WriteString(a.String())
	}
	b.WriteByte(')')
	return b.String()
}
