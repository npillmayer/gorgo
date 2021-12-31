package scanner

import (
	"bufio"
	"bytes"
	"io"
	"strings"
	"testing"
	"unicode"

	"github.com/npillmayer/gorgo"
	"github.com/npillmayer/schuko/tracing/gotestingadapter"
)

func TestConvertUnsigned(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "pmmp.grammar")
	defer teardown()
	//
	for i, pair := range []struct {
		s string
		v float64
	}{
		{s: "1", v: 1.0},
		{s: "1.0", v: 1.0},
		{s: "1.567", v: 1.567},
		{s: "-1.567", v: -1.567},
		{s: "1/2", v: 0.5},
		{s: "-1/20", v: -0.05},
		{s: "-.5", v: -0.5},
	} {
		if f := unsignedValue(pair.s); f != pair.v {
			t.Errorf("test %d: unsigned %q not recognized: %f", i, pair.s, pair.v)
		}
	}
}

func TestLexerPeek(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "pmmp.grammar")
	defer teardown()
	//
	input := "test!"
	stream := CatSeqReader{
		reader: bufio.NewReader(strings.NewReader(input)),
		writer: bytes.Buffer{},
	}
	for i := 0; i < 5; i++ {
		r, err := stream.lookahead()
		if err != nil {
			t.Error(err)
		}
		if r != []rune(input)[i] {
			t.Errorf("expected rune #%d to be %#U, is %#U", i, input[i], r)
		}
		stream.match(r)
	}
	_, err := stream.lookahead()
	if err != io.EOF {
		t.Logf("err = %q", err.Error())
		t.Error("expected rune to be 0 and error to be EOF; isn't")
	}
}

func TestLexerCatSeq(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "pmmp.grammar")
	defer teardown()
	//
	for i, test := range []struct {
		input string
		cat   CatCode
		l     int
	}{
		{input: "abc ;", cat: 1, l: 3},
		{input: "123 ;", cat: 2, l: 3},
		{input: ">= ;", cat: 3, l: 2},
		{input: "+-+ ;", cat: 4, l: 3},
		{input: "();", cat: 5, l: 1},
	} {
		strm := CatSeqReader{reader: bufio.NewReader(strings.NewReader(test.input))}
		csq, err := strm.Next(testCategorizer("abcdef", "1234567890", "<>=", "+-", "("))
		if err != nil {
			t.Error(err)
		}
		if csq.Length != test.l || csq.Cat != test.cat {
			t.Errorf("test %d failed: exepected %d|%d, have %d|%d", i+1, test.cat, test.l, csq.Cat, csq.Length)
		}
	}
}

// ---------------------------------------------------------------------------

func eofToken(pos uint64) gorgo.Token {
	return DefaultToken{
		kind: EOF,
		span: gorgo.Span{pos, pos},
	}
}

const (
	cat0  CatCode = iota // letter
	cat1                 // <=>:|≤≠≥
	cat2                 // `'´
	cat3                 // +-
	cat4                 // /*\
	cat5                 // !?
	cat6                 // #&@$
	cat7                 // ^~
	cat8                 // [
	cat9                 // ]
	cat10                // {}
	cat11                // .
	cat12                // , ; ( )
	cat13                // "
	cat14                // digit
	cat15                // %
	catNL
	catSpace
	catErr
)

var catcodeTable = []string{
	"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ", // use unicode.IsLetter
	`<=>:|≤≠≥`, "`'", `+-`, `/*\`, `!?`, `#&@$`, `^~`, `[`, `]`, `{}`, `.`, `,;()`, `"`,
	"0123456789", // use unicod.IsDigit
	`%`, "\n\r", " \t",
}

func cat(r rune) CatCode {
	if unicode.IsLetter(r) {
		return cat0
	}
	if unicode.IsDigit(r) {
		return cat14
	}
	for c, cat := range catcodeTable {
		if strings.ContainsRune(cat, r) {
			return CatCode(c)
		}
	}
	return catErr
}

type ctgrzr []string

func testCategorizer(c ...string) ctgrzr {
	return ctgrzr(c)
}

func (ct ctgrzr) Cat(r rune) (CatCode, bool) {
	for i, s := range ct {
		if strings.ContainsRune(s, r) {
			return CatCode(i + 1), len(s) == 1
		}
	}
	return IllegalCatCode, true
}
