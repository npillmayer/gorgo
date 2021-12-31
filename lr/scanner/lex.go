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

type CatCode int16

const IllegalCatCode CatCode = 0

type RuneCategorizer interface {
	Cat(r rune) (cat CatCode, isLoner bool)
}

type CatSeq struct {
	Cat    CatCode // catcode of all runes in this sequence
	Length int     // length of sequence in terms of runes
}

// --- Category sequence reader ----------------------------------------------

type CatSeqReader struct {
	isEof      bool
	next       rune
	rlen       int
	start, end uint64 // as bytes index
	reader     io.RuneReader
	writer     bytes.Buffer
}

func NewCatSeqReader(r io.RuneReader) *CatSeqReader {
	csr := &CatSeqReader{
		reader: r,
	}
	return csr
}

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

func (rs CatSeqReader) OutputString() string {
	return rs.writer.String()
}

func (rs *CatSeqReader) ResetOutput() {
	if rs == nil {
		return
	}
	rs.writer.Reset()
	rs.start = rs.end
}

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
