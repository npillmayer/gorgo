/*
Package scanner defines an interface for scanners to be used with parsers of package lr.

Two default scanner implementations are provided: (1) a thin wrapper over the Go std lib
'text/scanner', and (2) an adapter for lexmachine, living in sub-package `lexmach`.

License

Governed by a 3-Clause BSD license. License file may be found in the root
folder of this module.

Copyright © 2017–2022 Norbert Pillmayer <norbert@pillmayer.com>

*/
package scanner

import (
	"fmt"
	"io"
	"text/scanner"

	"github.com/npillmayer/gorgo"
	"github.com/npillmayer/schuko/tracing"
)

// tracer traces with key 'gorgo.scanner'.
func tracer() tracing.Trace {
	return tracing.Select("gorgo.scanner")
}

// EOF is identical to text/scanner.EOF.
// Token types are replicated here for practical reasons.
const (
	EOF       = scanner.EOF
	Ident     = scanner.Ident
	Int       = scanner.Int
	Float     = scanner.Float
	Char      = scanner.Char
	String    = scanner.String
	RawString = scanner.RawString
	Comment   = scanner.Comment
)

// Tokenizer is a scanner interface.
type Tokenizer interface {
	NextToken() gorgo.Token
	SetErrorHandler(func(error))
}

// DefaultTokenizer is a default implementation, backed by scanner.Scanner.
// Create one with GoTokenizer.
type DefaultTokenizer struct {
	scanner.Scanner
	lastToken    rune        // last token this scanner has produced
	Error        func(error) // error handler
	unifyStrings bool        // convert single chars to strings
}

var _ Tokenizer = (*DefaultTokenizer)(nil)

// Default error reporting function for scanners
func logError(e error) {
	tracer().Errorf("scanner error: " + e.Error())
}

// GoTokenizer creates a scanner/tokenizer accepting tokens similar to the Go language.
func GoTokenizer(sourceID string, input io.Reader, opts ...Option) *DefaultTokenizer {
	t := &DefaultTokenizer{}
	t.Error = logError
	t.Init(input)
	t.Filename = sourceID
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// SetErrorHandler sets an error handler for the scanner.
func (t *DefaultTokenizer) SetErrorHandler(h func(error)) {
	if h == nil {
		t.Error = logError
		return
	}
	t.Error = h
}

// NextToken is part of the Tokenizer interface.
func (t *DefaultTokenizer) NextToken() gorgo.Token {
	t.lastToken = t.Scan()
	if t.lastToken == scanner.EOF {
		tracer().Debugf("DefaultTokenizer reached end of input")
	}
	if t.unifyStrings &&
		(t.lastToken == scanner.RawString || t.lastToken == scanner.Char) {
		t.lastToken = scanner.String
	}
	return DefaultToken{
		kind:   gorgo.TokType(t.lastToken),
		lexeme: t.TokenText(),
		span:   gorgo.Span{uint64(t.Position.Offset), uint64(t.Pos().Offset)},
	}
}

// --- Default tokens --------------------------------------------------------

// DefaultToken is a very unsophisticated token type, used as default for the Go
// tokenizer as well as the LexMachine scanner.
type DefaultToken struct {
	kind   gorgo.TokType
	lexeme string
	Val    interface{}
	span   gorgo.Span
}

func MakeDefaultToken(typ gorgo.TokType, lexeme string, span gorgo.Span) DefaultToken {
	return DefaultToken{
		kind:   typ,
		lexeme: lexeme,
		span:   span,
	}
}

func (t DefaultToken) TokType() gorgo.TokType {
	return t.kind
}

func (t DefaultToken) Value() interface{} {
	return t.Val
}

func (t DefaultToken) Lexeme() string {
	return t.lexeme
}

func (t DefaultToken) Span() gorgo.Span {
	return t.span
}

// --- Scanner options for the default (Go) tokenizer ---------------------------

// Option configures a default tokenier.
type Option func(p *DefaultTokenizer)

const (
	optionSkipComments uint = 1 << 1 // do not pass comments
	optionUnifyStrings uint = 1 << 2 // treat raw strings and single chars as strings
)

// SkipComments set or clears mode-flag SkipComments.
func SkipComments(b bool) Option {
	return func(t *DefaultTokenizer) {
		if !t.hasmode(optionSkipComments) && b ||
			t.hasmode(optionSkipComments) && !b {
			t.Mode |= scanner.SkipComments
		}
	}
}

// UnifyStrings sets or clears option UnifyStrings:
// treat raw strings and single chars as strings.
func UnifyStrings(b bool) Option {
	return func(t *DefaultTokenizer) {
		t.unifyStrings = b
	}
}

func (t *DefaultTokenizer) hasmode(m uint) bool {
	if m == optionUnifyStrings {
		return t.unifyStrings
	}
	return t.Mode&m > 0
}

// Lexeme is a helper function to receive a string from a token.
func Lexeme(token interface{}) string {
	switch t := token.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}
