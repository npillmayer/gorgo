package gorgo

import "fmt"

// --- A general purpose interface for tokens --------------------------------

// TokType is a category type for a Token. We do not define any constants here, as
// it is up to applications to define them.
type TokType int

// TokTypeStringer is a type to be provided by a scanner/parser combination to be able
// to print out token categories.
type TokTypeStringer func(TokType) string

// Tokens represent input tokens. They are usually produced by a scanner and
// reflect terminals in a language.
//
// An example would be a token for a floating point numer:
//
//    TokType = Float       // identifier for this kind of tokens (appliation specific)
//    Lexeme  = "3.1316"    // lexeme how it appreared in the input stream
//    Value   = 3.1416      // is a float64 value
//    Span    = 67…73       // occured from position 67 in the input stream
//
// Token.Value() could either have been set by the scanner, or converted from Token.Lexeme()
// by a parsetree-listener (see e.g. `sppf.Listener`)
type Token interface {
	TokType() TokType
	Lexeme() string
	Value() interface{}
	Span() Span
}

// TokenRetriever is a type for getting tokens at an input position.
// Most scanner/parser combinations will keep track of input tokens. However, this is not
// a must. Factoring it out into a type helps model this design-decision.
type TokenRetriever func(uint64) Token

// --- Spans ------------------------------------------------------------

// Span is a small type for capturing a length of input token run. For every
// terminal and non-terminal, a parse tree/forest will track which input positions
// this symbol covers. A span denotes a start position and the position just
// behind the end.
type Span [2]uint64 // (x…y)

// From returns the start value of a span.
func (s Span) From() uint64 {
	return s[0]
}

// To returns the end value of a span.
func (s Span) To() uint64 {
	return s[1]
}

// Len returns the length of (x…y)
func (s Span) Len() uint64 {
	return s[1] - s[0]
}

func (s Span) IsNull() bool {
	return s == Span{}
}

func (s Span) Extend(other Span) Span {
	if other[0] < s[0] {
		s[0] = other[0]
	}
	if other[1] > s[1] {
		s[1] = other[1]
	}
	return s
}

func (s Span) String() string {
	return fmt.Sprintf("(%d…%d)", s[0], s[1])
}
