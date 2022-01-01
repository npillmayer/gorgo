package scanner

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/npillmayer/gorgo"
)

// --- Category codes --------------------------------------------------------

// CatCode is a type for character category codes. Clients should define a
// RuneCategorizer to map input characters to category codes.
type CatCode int16

const IllegalCatCode CatCode = 0

// RuneCategorizer maps input characters to category codes. This is used with a
// CatSeqReader to split the input stream into sequences of characters, where each
// character will be in the same category.
//
// Sometimes it is necessary to prohibit forming sequences for certain categories.
// For example, the MetaFont programs needs `(` characters to always be single-character
// tokens (avoid having a token of "((("). For these cases, a RuneCategorizer should
// use true as the second return value.
//
type RuneCategorizer interface {
	Cat(r rune) (cat CatCode, isLoner bool)
}

// CatSeq marks a sequence of characters where each character has the same category code.
// Clients will use such sequences to create small automatons to recognize input tokens.
type CatSeq struct {
	Cat    CatCode // catcode of all runes in this sequence
	Length int     // length of sequence in terms of runes
}

// --- Category sequence reader ----------------------------------------------

// CatSeqReader is a type to split input streams into sequences of characters with the
// same category code.
type CatSeqReader struct {
	isEof      bool
	next       rune
	rlen       int
	start, end uint64 // as bytes index
	reader     io.RuneReader
	writer     bytes.Buffer
}

// NewCatSeqReader creates a CatSeqReader from a given rune reader.
func NewCatSeqReader(r io.RuneReader) *CatSeqReader {
	csr := &CatSeqReader{
		reader: r,
	}
	return csr
}

// Next returns the next character sequence from the underlying input stream.
//
// If the end of the input stream has been reached, a CatSeq with csq.Cat == io.EOF will
// be returned.
//
func (rs CatSeqReader) Next(rc RuneCategorizer) (csq CatSeq, err error) {
	var r rune
	r, err = rs.lookahead()
	if err != nil && err != io.EOF {
		csq.Length = 0
		return csq, fmt.Errorf("scanner cannot read sequence (%w)", err)
	} else if err == io.EOF {
		return csq, io.EOF
	}
	var isLoner bool
	csq.Cat, isLoner = rc.Cat(r)
	if isLoner { // rune category is not allowed to form sequences
		rs.match(r)
		csq.Length = 1
		return
	}
	cc := csq.Cat
	for cc == csq.Cat {
		rs.match(r)
		r, err = rs.lookahead()
		csq.Length++
		if err != nil && (err != io.EOF || r == 0) {
			return
		}
		cc, _ = rc.Cat(r)
	}
	return
}

// OutputString returns the character sequence as a string, corresponding to the
// `csq` return value of the last call to Next.
// Scanners usually will use aggregates of rs.OutputString and rs.Span to create a token.
func (rs CatSeqReader) OutputString() string {
	return rs.writer.String()
}

// ResetOutput resets the internal character collection of a CatSeqReader to the
// empty string. This is usually called by scanners after a complete token has been
// recognized and returned to the parser.
func (rs *CatSeqReader) ResetOutput() {
	if rs == nil {
		return
	}
	rs.writer.Reset()
	rs.start = rs.end
}

// Span returns the span of the character sequence corresponding to the
// `csq` return value of the last call to Next.
// Scanners usually will use aggregates of rs.OutputString and rs.Span to create a token.
func (rs CatSeqReader) Span() gorgo.Span {
	return gorgo.Span{rs.start, rs.end}
}

func (rs *CatSeqReader) lookahead() (r rune, err error) {
	if rs == nil || rs.isEof {
		return utf8.RuneError, io.EOF
	}
	if rs.next != 0 {
		r = rs.next
		tracer().Debugf("read LA %#U", r)
		return
	}
	var sz int
	r, sz, err = rs.reader.ReadRune()
	rs.next = r
	rs.rlen += sz
	if err == io.EOF {
		tracer().Debugf("EOF for MetaPost input")
		rs.isEof = true
		r = utf8.RuneError
		return
	} else if err != nil {
		return 0, err
	}
	tracer().Debugf("read rune %#U", r)
	return
}

func (rs *CatSeqReader) match(r rune) {
	tracer().Debugf("match %#U", r)
	if rs == nil {
		return
	}
	if r == utf8.RuneError {
		panic("EOF matched")
	}
	rs.writer.WriteRune(r)
	s := string(r)
	rs.end += uint64(len([]byte(s)))
	if rs.isEof {
		return
	}
	if rs.next != 0 {
		rs.next = 0
		return
	}
}

// --- Utilities -------------------------------------------------------------

func unsignedValue(s string) float64 {
	var f float64 = 1.0
	if strings.HasPrefix(s, "+") {
		s = s[1:]
	} else if strings.HasPrefix(s, "-") {
		f *= -1.0
		s = s[1:]
	}
	if strings.Contains(s, "/") {
		a := strings.Split(s, "/")
		if len(a) != 2 {
			panic(fmt.Sprintf("malformed fraction: %q", s))
		}
		nom, err1 := strconv.Atoi(a[0])
		denom, err2 := strconv.Atoi(a[1])
		if err1 != nil || err2 != nil {
			panic(fmt.Sprintf("malformed fraction: %q", s))
		}
		f = f * (float64(nom) / float64(denom))
	} else if strings.Contains(s, ".") {
		a, err := strconv.ParseFloat(s, 64)
		if err != nil {
			panic(fmt.Sprintf("malformed fraction: %q", s))
		}
		f = f * a
	}
	return f
}

/*
func makeToken(state scstate, lexeme string) (gorgo.TokType, gorgo.Token) {
	tracer().Debugf("scanner.makeToken state=%d, lexeme=%q", state, lexeme)
	toktype := tokval4state[state-accepting_states]
	if toktype == SymTok {
		if id, ok := tokenTypeFromLexeme[lexeme]; ok {
			toktype = id // symbolic token has a pre-defined meaning
		} else {
			// TODO lookup in symbol table
			// if not entry => Tag
			// if variable => Tag
			// otherwise => Spark
			toktype = Tag
		}
	} else if toktype == Literal {
		toktype = gorgo.TokType(lexeme[0])
	}
	return toktype, MPToken{
		lexeme: lexeme,
		kind:   toktype,
	}
}
*/

/*
// TODO
//
// ⟨scalar multiplication op⟩ → +
//     | −
//     | ⟨‘ ⟨number or fraction⟩ ’ not followed by ‘ ⟨add op⟩  ⟨number⟩ ’⟩
//
func numberToken(lexeme string, la rune) (gorgo.TokType, MPToken) {
	if !unicode.IsLetter(la) {
		return Unsigned, MPToken{
			lexeme: lexeme,
			kind:   Unsigned,
			Val:    unsignedValue(lexeme),
		}
	}
	return ScalarMulOp, MPToken{
		lexeme: lexeme,
		kind:   ScalarMulOp,
		Val:    unsignedValue(lexeme),
	}
}
*/
