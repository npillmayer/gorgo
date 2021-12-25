package scanner

import (
	"strings"

	"github.com/npillmayer/gorgo"
	"github.com/npillmayer/schuko/gtrace"

	"github.com/timtadh/lexmachine"
	"github.com/timtadh/lexmachine/machines"
)

// lexmachine adapter

// LMAdapter is a lexmachine adapter to use lexmachine as a scanner.
type LMAdapter struct {
	Lexer *lexmachine.Lexer
}

// NewLMAdapter creates a new lexmachine adapter. It receives a list of
// literals ('[', ';', …), a list of keywords ("if", "for", …) and a
// map for translating token strings to their values.
//
// NewLMAdapter will return an error if compiling the DFA failed.
func NewLMAdapter(init func(*lexmachine.Lexer), literals []string, keywords []string, tokenIds map[string]int) (*LMAdapter, error) {
	adapter := &LMAdapter{}
	adapter.Lexer = lexmachine.NewLexer()
	init(adapter.Lexer)
	for _, lit := range literals {
		r := "\\" + strings.Join(strings.Split(lit, ""), "\\")
		adapter.Lexer.Add([]byte(r), MakeToken(lit, tokenIds[lit]))
	}
	for _, name := range keywords {
		adapter.Lexer.Add([]byte(strings.ToLower(name)), MakeToken(name, tokenIds[name]))
	}
	if err := adapter.Lexer.Compile(); err != nil {
		gtrace.SyntaxTracer.Errorf("Error compiling DFA: %v", err)
		return nil, err
	}
	return adapter, nil
}

// Scanner creates a scanner for a given input. The scanner will implement the
// Tokenizer interface.
func (lm *LMAdapter) Scanner(input string) (*LMScanner, error) {
	s, err := lm.Lexer.Scanner([]byte(input))
	if err != nil {
		return &LMScanner{}, err
	}
	return &LMScanner{s, logError}, nil
}

// LMScanner is a scanner type for lexmachine scanners, implementing the
// Tokenizer interface.
type LMScanner struct {
	scanner *lexmachine.Scanner
	Error   func(error)
}

var _ Tokenizer = (*LMScanner)(nil)

// SetErrorHandler sets an error handler for the scanner.
func (lms *LMScanner) SetErrorHandler(h func(error)) {
	if h == nil {
		lms.Error = logError
		return
	}
	lms.Error = h
}

// NextToken is part of the Tokenizer interface.
//
// Warning: The current implementation will ignore the 'expected'-argument.
func (lms *LMScanner) NextToken(expected []int) gorgo.Token {
	//func (lms *LMScanner) NextToken(expected []int) (int, interface{}, uint64, uint64) {
	tok, err, eof := lms.scanner.Next()
	for err != nil {
		lms.Error(err)
		if ui, is := err.(*machines.UnconsumedInput); is {
			lms.scanner.TC = ui.FailTC
		}
		tok, err, eof = lms.scanner.Next()
	}
	if eof {
		//return EOF, nil, 0, 0
		return defaultToken{kind: EOF, lexeme: "", span: gorgo.Span{0, 0}}
	}
	tracer().Debugf("tok is %T | %v", tok, tok)
	token := tok.(*lexmachine.Token)
	//tokval := token.Type
	//start := uint64(token.StartColumn)
	//length := uint64(len(token.Lexeme))
	//return tokval, token, start, length
	return defaultToken{
		kind:   gorgo.TokType(token.Type),
		lexeme: string(string(token.Lexeme)),
		span:   gorgo.Span{uint64(token.StartColumn), uint64(token.EndColumn)},
	}
}

// ---------------------------------------------------------------------------

// Skip is a pre-defined action which ignores the scanned match.
func Skip(*lexmachine.Scanner, *machines.Match) (interface{}, error) {
	return nil, nil
}

// MakeToken is a pre-defined action which wraps a scanned match into a token.
func MakeToken(name string, id int) lexmachine.Action {
	return func(s *lexmachine.Scanner, m *machines.Match) (interface{}, error) {
		return s.Token(id, string(m.Bytes), m), nil
	}
}
