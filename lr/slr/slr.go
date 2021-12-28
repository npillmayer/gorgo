/*
Package slr provides an SLR(1)-parser. Clients have to use the tools
of package lr to prepare the necessary parse tables. The SLR parser
utilizes these tables to create a right derivation for a given input,
provided through a scanner interface.

This parser is intended for small to moderate grammars, e.g. for configuration
input or small domain-specific languages. It is *not* intended for full-fledged
programming languages (there are superb other tools around for these kinds of
usages, usually creating LALR(1)-parsers, which are able to recognize a super-set
of SLR-languages).

The main focus for this implementation is adaptability and on-the-fly usage.
Clients are able to construct the parse tables from a grammar and use the
parser directly, without a code-generation or compile step. If you want, you
can create a grammar from user input and use a parser for it in a couple of
lines of code.

Package slr can only handle SLR(1) grammars. All SLR-grammars are deterministic
(but not vice versa). For parsing ambiguous grammars, see package glr.

Usage

Clients construct a grammar, usually by using a grammar builder:

	b := lr.NewGrammarBuilder("Signed Variables Grammar")
	b.LHS("Var").N("Sign").T("a", scanner.Ident).End()  // Var  --> Sign Id
	b.LHS("Sign").T("+", '+').End()                     // Sign --> +
	b.LHS("Sign").T("-", '-').End()                     // Sign --> -
	b.LHS("Sign").Epsilon()                             // Sign -->
	g, err := b.Grammar()

This grammar is subjected to grammar analysis and table generation.

	ga := lr.NewGrammarAnalysis(g)
	lrgen := lr.NewTableGenerator(ga)
	lrgen.CreateTables()
	if lrgen.HasConflicts { ... }  // cannot use an SLR parser

Finally parse some input:

	p := slr.NewParser(g, lrgen.GotoTable(), lrgen.ActionTable())
	scanner := slr.NewStdScanner(string.NewReader("+a")
	accepted, err := p.Parse(lrgen.CFSM().S0, scanner)

Clients may instrument the grammar with semantic operations or let the
parser create a parse tree. See the examples below.

Warning

This is a very early implementation. Currently you should use it for study purposes
only. The API may change significantly without prior notice.

___________________________________________________________________________

License

Governed by a 3-Clause BSD license. License file may be found in the root
folder of this module.

Copyright © 2017–2021 Norbert Pillmayer <norbert@pillmayer.com>

*/
package slr

import (
	"fmt"

	"github.com/npillmayer/gorgo"
	"github.com/npillmayer/schuko/tracing"

	"github.com/npillmayer/gorgo/lr"
	"github.com/npillmayer/gorgo/lr/scanner"
)

// tracer traces with key 'gorgo.lr'.
func tracer() tracing.Trace {
	return tracing.Select("gorgo.lr")
}

// Parser is an SLR(1)-parser type. Create and initialize one with slr.NewParser(...)
type Parser struct {
	G       *lr.Grammar
	stack   []stackitem // parser stack
	gotoT   *lr.Table   // GOTO table
	actionT *lr.Table   // ACTION table
}

// We store pairs of state-IDs and symbol-IDs on the parse stack.
type stackitem struct {
	stateID uint       // ID of a CFSM state
	symID   int        // ID of a grammar symbol (terminal or non-terminal)
	span    gorgo.Span // input span over which this symbol reaches
	//span    span // input span over which this symbol reaches
}

// span is a small type for capturing a length of input token run. For every
// terminal and non-terminal, a parse tree/forest will track which input positions
// this symbol covers.
// Some useful operations on spans are to be found further down in this file.
//
//type span [2]uint64 // start and end positions in the input string

// NewParser creates an SLR(1) parser.
func NewParser(g *lr.Grammar, gotoTable *lr.Table, actionTable *lr.Table) *Parser {
	parser := &Parser{
		G:       g,
		stack:   make([]stackitem, 0, 512),
		gotoT:   gotoTable,
		actionT: actionTable,
	}
	return parser
}

// Scanner is a scanner-interface the parser relies on to receive the next input token.
// type Scanner interface {
// 	MoveTo(position uint64)
// 	NextToken(expected []int) (tokval int, token interface{}, start, len uint64)
// }

// Parse startes a new parse, given a start state and a scanner tokenizing the input.
// The parser must have been initialized.
//
// The parser returns true if the input string has been accepted.
func (p *Parser) Parse(S *lr.CFSMState, scan scanner.Tokenizer) (bool, error) {
	tracer().Debugf("~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~")
	if p.G == nil || p.gotoT == nil {
		tracer().Errorf("SLR(1)-parser not initialized")
		return false, fmt.Errorf("SLR(1)-parser not initialized")
	}
	var accepting bool
	//p.stack = append(p.stack, stackitem{S.ID, 0, span{0, 0}}) // push S
	p.stack = append(p.stack, stackitem{S.ID, 0, gorgo.Span{0, 0}}) // push S
	// http://www.cse.unt.edu/~sweany/CSCE3650/HANDOUTS/LRParseAlg.pdf
	//tokval, token, pos, length := scan.NextToken(nil)
	token := scan.NextToken()
	tokval := token.TokType()
	done := false
	for !done {
		tracer().Debugf("got token %q/%d from scanner", token.Lexeme(), tokval)
		state := p.stack[len(p.stack)-1] // TOS
		action := p.actionT.Value(state.stateID, tokval)
		tracer().Debugf("action(%d,%d)=%s", state.stateID, tokval, valstring(action, p.actionT))
		if action == p.actionT.NullValue() {
			return false, fmt.Errorf("syntax error at %d/%v", tokval, token)
		}
		if action == lr.AcceptAction {
			accepting, done = true, true
			// TODO patch start symbol to have span(0,pos)
		} else if action == lr.ShiftAction {
			nextstate := uint(p.gotoT.Value(state.stateID, tokval))
			tracer().Debugf("shifting, next state = %d", nextstate)
			p.stack = append(p.stack, // push a terminal state onto stack
				stackitem{nextstate, int(tokval), token.Span()}) //span{pos, pos + length - 1}})
			//tokval, token, pos, length = scan.NextToken(nil)
			token = scan.NextToken()
			tokval = token.TokType()
		} else if action > 0 { // reduce action
			rule := p.G.Rule(int(action))
			nextstate, handlespan := p.reduce(state.stateID, rule)
			if handlespan.IsNull() { // resulted from an epsilon production
				//handlespan = span{pos - 1, pos - 1} // epsilon was just before lookahead
				pos := token.Span().From()
				handlespan = gorgo.Span{pos - 1, pos - 1} // epsilon was just before lookahead
			}
			tracer().Debugf("reduced to next state = %d", nextstate)
			p.stack = append(p.stack, // push a non-terminal state onto stack
				stackitem{nextstate, rule.LHS.Value, handlespan})
		} else { // no action found
			done = true
		}
	}
	return accepting, nil
}

// reduce performs a reduce action for a rule
//
//    LHS --> X1 ... Xn   (with X being terminals or non-terminals)
//
// Symbols X1 to Xn should be represented on the stack as states
//
//    [TOS]  Sn(Xn, span_n) ... S1(X1, span1)  ...
//
// TODO: perform semantic action on reduce, either by calling a user-provided
// function from the grammar, or by constructing a node in a parse tree/forest.
func (p *Parser) reduce(stateID uint, rule *lr.Rule) (uint, gorgo.Span) {
	tracer().Infof("reduce %v", rule)
	handle := reverse(rule.RHS())
	//var handlespan span
	var handlespan gorgo.Span
	for _, sym := range handle {
		p.stack = p.stack[:len(p.stack)-1] // pop TOS
		tos := p.stack[len(p.stack)-1]
		if tos.symID != sym.Value {
			tracer().Errorf("Expected %v on top of stack, got %d", sym, tos.symID)
		}
		handlespan = handlespan.Extend(tos.span)
	}
	lhs := rule.LHS
	// TODO: now perform sematic action or parse-tree build
	state := p.stack[len(p.stack)-1] // TOS
	nextstate := p.gotoT.Value(state.stateID, lhs.TokenType())
	return uint(nextstate), handlespan
}

// --- Helpers ----------------------------------------------------------

// reverse the symbols of a RHS of a rule (i.e., a handle)
func reverse(syms []*lr.Symbol) []*lr.Symbol {
	r := append([]*lr.Symbol(nil), syms...) // make copy first
	for i := len(syms)/2 - 1; i >= 0; i-- {
		opp := len(syms) - 1 - i
		r[i], r[opp] = r[opp], r[i]
	}
	return r
}

// valstring is a short helper to stringify an action table entry.
func valstring(v int32, m *lr.Table) string {
	if v == m.NullValue() {
		return "<none>"
	} else if v == lr.AcceptAction {
		return "<accept>"
	} else if v == lr.ShiftAction {
		return "<shift>"
	}
	return fmt.Sprintf("%d", v)
}
