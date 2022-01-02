/*
Package scanner defines an interface for scanners to be used with parsers of package lr.
For example, an Earley parser will have a scanner plugged into it like this:

	scan := scanner.GoTokenizer(…)           // create a scanner for Go tokens
	accept, err := earley.Parse(scan, nil)   // use it with a parser

The main interface defined in package scanner is the Tokenizer. All the parsers of package
gorgo.lr need a Tokenizer to be plugged in for reading and splitting the input stream.

Two default scanner implementations are provided: (1) a thin wrapper over the Go standard
library's 'text/scanner', and (2) an adapter for lexmachine, living in sub-package `lexmach`.

Some supporting functions are provided for option (3), which is tokenization in terms of
“category codes”. This is a concept used most prominently in Donald E. Knuth's TeX and
MetaFont programs. Input characters are grouped into categories, and in turn tokens are
constructed from sequences of categorized characters. This hugely simplyfies pattern
recogntion and avoids having to deal with complex regular expressions. For average DSL
definitions a simple automaton on top of category sequences can readily be implemented by hand.

As an example, consider the lexical recognition of relational operators.
We define a category for them:

	const RelOp CatCode = 1

Then we define a RuneCategorizer which will return `RelOp` for each of input characters
'<', '>', '=' and '!'.

	func myRuneCategorizer(r rune) {
	    …
	    case '<', '>', '=', '!':
	        return RelOp, false    // category 'RelOp', no loners
	    …
	}

With this, we're set to instantiate a CatSeqReader and use it:

	r := NewCatSeqReader(bufio.NewReader( ⟨my input …⟩ ))
	for … {
	    csq, err := r.Next(myRuneCategorizer)
	    if csq.Cat == RelOp {    // matches ">=", "!=", "<", "=", "<=>" etc.
	        …                    // usually identify legal ones by map-lookup
	    }
	}

For a discussion of a real-life example for this concept, please refer to
this blog-entry
(https://npillmayer.github.io/GoRGO/)
on a scanner for the MetaFont/MetaPost language.

___________________________________________________________________________

License

Governed by a 3-Clause BSD license. License file may be found in the root
folder of this module.

Copyright © 2017–2022 Norbert Pillmayer <norbert@pillmayer.com>

*/
package scanner

import (
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
// Clients are not required to use any of these, except respect `EOF` as a signal
// that a tokenzier has reached the end of its input.
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

// Tokenizer is a scanner interface used by the parsers of package gorgo.lr.
// Please refer to sub-packages of lr for examples on how to plug a scanner.Tokenizer
// into parsers.
type Tokenizer interface {
	NextToken() gorgo.Token      // read the next token from the input stream
	SetErrorHandler(func(error)) // instruct the tokenizer on how to process errors
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

// GoTokenizer creates a Tokenizer accepting tokens similar to the Go language.
// It is a thin wrapper around text/scanner.
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
// Individual scanners may use this to recover from error conditions, e.g. by
// skipping tokens.
func (t *DefaultTokenizer) SetErrorHandler(h func(error)) {
	if h == nil {
		t.Error = logError
		return
	}
	t.Error = h
}

// NextToken is part of the Tokenizer interface.
// It is called by parsers to receive the next input token.
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
// tokenizer as well as the LexMachine adapter.
//
// DefaultToken implements interface gorgo.Token.
//
type DefaultToken struct {
	kind   gorgo.TokType
	lexeme string
	Val    interface{} // application-specific token value
	span   gorgo.Span
}

func MakeDefaultToken(typ gorgo.TokType, lexeme string, span gorgo.Span) DefaultToken {
	return DefaultToken{
		kind:   typ,
		lexeme: lexeme,
		span:   span,
	}
}

// TokType returns the token type for a token created by a scanner.
func (t DefaultToken) TokType() gorgo.TokType {
	return t.kind
}

// Value returns an application-specific value for a token.
func (t DefaultToken) Value() interface{} {
	return t.Val
}

// Lexeme returns the string representation of an input token.
func (t DefaultToken) Lexeme() string {
	return t.lexeme
}

// Span denotes the extent of a token within the input stream.
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
