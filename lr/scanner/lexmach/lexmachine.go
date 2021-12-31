package lexmach

import (
	"strings"

	"github.com/npillmayer/gorgo"
	"github.com/npillmayer/gorgo/lr/scanner"
	"github.com/npillmayer/schuko/tracing"

	"github.com/timtadh/lexmachine"
	"github.com/timtadh/lexmachine/machines"
)

// lexmachine adapter

// tracer traces with key 'gorgo.scanner'.
func tracer() tracing.Trace {
	return tracing.Select("gorgo.scanner")
}

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
		tracer().Errorf("Error compiling DFA: %v", err)
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

var _ scanner.Tokenizer = (*LMScanner)(nil)

// SetErrorHandler sets an error handler for the scanner.
func (lms *LMScanner) SetErrorHandler(h func(error)) {
	if h == nil {
		lms.Error = logError
		return
	}
	lms.Error = h
}

// Default error reporting function for lexmachine-based scanners
func logError(e error) {
	tracer().Errorf("scanner error: " + e.Error())
}

// NextToken is part of the Tokenizer interface.
//
// Warning: The current implementation will ignore the 'expected'-argument.
func (lms *LMScanner) NextToken() gorgo.Token {
	tok, err, eof := lms.scanner.Next()
	for err != nil {
		lms.Error(err)
		if ui, is := err.(*machines.UnconsumedInput); is {
			lms.scanner.TC = ui.FailTC
		}
		tok, err, eof = lms.scanner.Next()
	}
	if eof {
		return scanner.MakeDefaultToken(scanner.EOF, "", gorgo.Span{0, 0})
	}
	tracer().Debugf("tok is %T | %v", tok, tok)
	token := tok.(*lexmachine.Token)
	return scanner.MakeDefaultToken(
		gorgo.TokType(token.Type),
		string(string(token.Lexeme)),
		gorgo.Span{uint64(token.StartColumn), uint64(token.EndColumn)},
	)
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
