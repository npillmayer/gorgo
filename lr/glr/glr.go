/*
Package glr implements a small-scale GLR(1)-parser.
It is mainly intended for Markdown parsing, but may be of use for
other purposes, too.

Clients have to use the tools
of package lr to prepare the necessary parse tables. The GLR parser
utilizes these tables to create a right derivation for a given input,
provided through a scanner interface.

This parser is intended for small to moderate grammars, e.g. for configuration
input or small domain-specific languages. It is *not* intended for full-fledged
programming languages (there are superb other tools around for these kinds of
usages, usually creating LL(k)- or LALR(1)-parsers, or in the case of Bison,
even a GLR(1)-parser).

The main focus for this implementation is adaptability and on-the-fly usage.
Clients are able to construct the parse tables from a grammar and use the
parser directly, without a code-generation or compile step. If you want, you
can create a grammar from user input and use a parser for it in a couple of
lines of code.

Package glr can handle ambiguous grammars, i.e. grammars which will
have shift/reduce- or reduce/reduce-conflicts in their parse tables.
For simpler parsing of deterministic SLR grammars, see package slr.

Warning

The API is still very much in flux! Currently it is something like:

	scanner := glr.NewStdScanner(strings.NewReader("some input text"))
	p := glr.NewParser(grammar, gotoTable, actionTable)
	p.Parse(startState, scanner)

___________________________________________________________________________

License

Governed by a 3-Clause BSD license. License file may be found in the root
folder of this module.

Copyright © 2017–2021 Norbert Pillmayer <norbert@pillmayer.com>

*/
package glr

import (
	"fmt"
	"io"
	"text/scanner"

	"github.com/npillmayer/gorgo"
	"github.com/npillmayer/gorgo/lr"
	"github.com/npillmayer/gorgo/lr/dss"
	"github.com/npillmayer/schuko/tracing"
)

// tracer traces with key 'gorgo.lr'.
func tracer() tracing.Trace {
	return tracing.Select("gorgo.lr")
}

// A Parser type for GLR parsing.
// Create and initialize one with glr.NewParser(...)
type Parser struct {
	G       *lr.Grammar // grammar to use; do not alter after initialization
	dss     *dss.Root   // DSS stack, i.e. multiple parse stacks
	gotoT   *lr.Table   // GOTO table
	actionT *lr.Table   // ACTION table
	//accepting []int             // slice of accepting states
}

// NewParser creates and initializes a parser object, given information from an
// lr.LRTableGenerator. Clients have to provide a link to the grammar and the
// parser tables.
func NewParser(g *lr.Grammar, gotoTable *lr.Table, actionTable *lr.Table) *Parser {
	parser := &Parser{
		G:       g,
		gotoT:   gotoTable,
		actionT: actionTable,
	}
	return parser
}

// From https://people.eecs.berkeley.edu/~necula/Papers/elkhound_cc04.pdf
//
// As with LR-parsing, the GLR algorithm uses a parse stack and a finite control.
// The finite control dictates what parse action (shift or reduce)to take,
// based on what the next token is, and the stack summarizes the leftcontext as
// a sequence of finite control state numbers. But unlike LR, GLR’s parse “stack”
// is not a stack at all: it is a graph which encodes all of the possible
// stack configurations that an LR parser could have. Each encoded stack is treated
// like a separate potential LR parser, and all stacks are processed in parallel, kept
// synchronized by always shifting a given token together.
//
// [..]
//
// The GLR algorithm proceeds as follows: On each token, for each stack top,
// every enabled LR action is performed. There may be more than one enabled
// action, corresponding to a shift/reduce or reduce/reduce conflict in ordinary
// LR. A shift adds a new node at the top of some stack node. A reduce also adds
// a new node, but depending on the length of the production’s right-hand side, it
// might point to the top or into the middle of a stack. The latter case corresponds
// to the situation where LR would pop nodes off the stack; but the GLR algorithm
// cannot in general pop reduced nodes because it might also be possible to shift.
// If there is more than one path of the required length from the origin node, the
// algorithm reduces along all such paths. If two stacks shift or reduce into the
// same state, then the stack tops are merged into one node.

// Parse startes a new parse, given a start state and a scanner tokenizing the input.
// The parser must have been initialized.
//
// Parse returns true, if the input was successfully recognized by the parse, false
// otherwise.
func (p *Parser) Parse(S *lr.CFSMState, scan Scanner) (bool, error) {
	if p.G == nil || p.gotoT == nil {
		tracer().Errorf("GLR parser not initialized")
		return false, fmt.Errorf("GLR parser not initialized")
	}
	p.dss = dss.NewRoot("G", -1)       // drops existing stacks for new run
	start := dss.NewStack(p.dss)       // create first stack instance in DSS
	start.Push(int(S.ID), p.G.Epsilon) // push the start state onto the stack
	accepting := false
	done := false
	tokval, token := scan.NextToken(nil)
	for !done && !accepting {
		if token == nil {
			tokval = scanner.EOF
		}
		tracer().Debugf("got token %v from scanner", token)
		activeStacks := p.dss.ActiveStacks()
		tracer().P("glr", "parse").Debugf("currently %d active stack(s)", len(activeStacks))
		for _, stack := range activeStacks {
			accepting = accepting || p.reducesAndShiftsForToken(stack, tokval)
		}
		tracer().Debugf("~~~~~ processed token %v ~~~~~~~~~~~~~~~~~~~~", token)
		if tokval == scanner.EOF {
			done = true
		} else {
			tokval, token = scan.NextToken(nil)
		}
	}
	return accepting, nil
}

// With a new lookahead (tokval): execute all possible reduces and shifts,
// cascading. The general outline is as follows:
//
//   1. do until no more reduces:
//      1.a if action(s) =
//           | shift: store stack and params in set S
//           | reduce: do reduce and store stack and params in set R
//           | conflict: shift/reduce or reduce/reduce
//              | do reduce(s) and store stack(s) in S or R respectively
//      1.b iterate again with R
//   2. shifts are now collected in S => execute
func (p *Parser) reducesAndShiftsForToken(stack *dss.Stack, tokval int) bool {
	var heads [2]*dss.Stack
	var actions [2]int32
	accepting := false
	S := newStackSet() // will collect shift actions
	R := newStackSet() // re-consider stack/action for reduce
	R = R.add(stack)   // start with this active stack
	for !R.empty() {
		heads[0] = R.get()
		stateID, sym := heads[0].Peek()
		tracer().P("dss", "TOS").Debugf("state = %d, symbol = %v", stateID, sym)
		actions[0], actions[1] = p.actionT.Values(uint(stateID), gorgo.TokType(tokval))
		if actions[0] == lr.AcceptAction {
			accepting = true
		}
		if actions[0] == p.actionT.NullValue() {
			tracer().Infof("no entry in ACTION table found, parser dies")
			heads[0].Die()
		} else {
			headcnt := 1
			tracer().Debugf("action 1 = %s, action 2 = %s",
				valstring(actions[0], p.actionT), valstring(actions[1], p.actionT))
			conflict := actions[1] != p.actionT.NullValue()
			if conflict { // shift/reduce or reduce/reduce conflict
				tracer().Infof("conflict, forking stack")
				heads[1] = stack.Fork() // must happen before action 1 !
				headcnt = 2
			}
			for i := 0; i < headcnt; i++ {
				if actions[i] >= 0 { // reduce action
					stacks := p.reduce(stateID, p.G.Rule(int(actions[i])), heads[i])
					R = R.add(stacks...)
				} else { // shift action
					S = S.add(heads[i])
				}
			}
		}
		tracer().Infof("%d shift operations in S", len(S))
		for !S.empty() {
			heads[0] = S.get()
			p.shift(stateID, tokval, heads[0])
		}
	}
	return accepting
}

func (p *Parser) shift(stateID int, tokval int, stack *dss.Stack) []*dss.Stack {
	nextstate := p.gotoT.Value(uint(stateID), gorgo.TokType(tokval))
	tracer().Infof("shifting %v to %d", tokenString(tokval), nextstate)
	terminal := p.G.Terminal(tokval)
	head := stack.Push(int(nextstate), terminal)
	return []*dss.Stack{head}
}

func (p *Parser) reduce(stateID int, rule *lr.Rule, stack *dss.Stack) []*dss.Stack {
	tracer().Infof("reduce %v", rule)
	handle := rule.RHS()
	heads := stack.Reduce(handle)
	if heads != nil {
		tracer().Debugf("reduce resulted in %d stacks", len(heads))
		lhs := rule.LHS
		for i, head := range heads {
			state, _ := head.Peek()
			tracer().Debugf("state on stack#%d is %d", i, state)
			nextstate := p.gotoT.Value(uint(state), gorgo.TokType(lhs.Value))
			newhead := head.Push(int(nextstate), lhs)
			tracer().Debugf("new head = %v", newhead)
		}
	}
	return heads
}

// --- Sets of Stacks --------------------------------------------------------

// helper: set of stacks
type stackSet []*dss.Stack

// TODO use sync.pool
func newStackSet() stackSet {
	s := make([]*dss.Stack, 0, 5)
	return stackSet(s)
}

// add a stack to the set
func (sset stackSet) add(stack ...*dss.Stack) stackSet {
	return append(sset, stack...)
}

// get a stack from the set
func (sset *stackSet) get() *dss.Stack {
	l := len(*sset)
	if l == 0 {
		return nil
	}
	s := (*sset)[l-1]
	(*sset)[l-1] = nil
	*sset = (*sset)[:l-1]
	return s
}

// is this set empty?
func (sset stackSet) empty() bool {
	return len(sset) == 0
}

// make a stack set empty
func (sset *stackSet) clear() {
	for k := len(*sset) - 1; k >= 0; k-- {
		(*sset)[k] = nil
	}
	*sset = (*sset)[:0]
}

// --- Scanner ----------------------------------------------------------

// A Token type, if you want to use it. Tokens of this type are returned
// by StdScanner.
//
// Clients may provide their own token data type.
type Token struct {
	Value  int
	Lexeme []byte
}

// Scanner is an interface the parser relies on.
type Scanner interface {
	MoveTo(position uint64)
	NextToken(expected []int) (tokval int, token interface{})
}

func tokenString(tok int) string {
	return scanner.TokenString(rune(tok))
}

func (token *Token) String() string {
	return fmt.Sprintf("(%s:%d|\"%s\")", tokenString(token.Value), token.Value,
		string(token.Lexeme))
}

// StdScanner provides a default scanner implementation, but clients are free (and
// even encouraged) to provide their own. This implementation is based on
// stdlib's text/scanner.
type StdScanner struct {
	reader io.Reader // will be io.ReaderAt in the future
	scan   scanner.Scanner
}

// NewStdScanner creates a new default scanner from a Reader.
func NewStdScanner(r io.Reader) *StdScanner {
	s := &StdScanner{reader: r}
	s.scan.Init(r)
	s.scan.Filename = "Go symbols"
	return s
}

// MoveTo is not functional for default scanners.
// Default scanners allow sequential processing only.
func (s *StdScanner) MoveTo(position uint64) {
	tracer().Errorf("MoveTo() not yet supported by parser.StdScanner")
}

// NextToken gets the next token scanned from the input source. Returns the token
// value and a user-defined token type.
//
// Clients may provide an array of token values, one of which is expected
// at the current parse position. For the default scanner, as of now this is
// unused. In the future it will help with error-repair.
func (s *StdScanner) NextToken(expected []int) (int, interface{}) {
	tokval := int(s.scan.Scan())
	token := &Token{Value: tokval, Lexeme: []byte(s.scan.TokenText())}
	tracer().P("token", tokenString(tokval)).Debugf("scanned token at %s = \"%s\"",
		s.scan.Position, s.scan.TokenText())
	return tokval, token
}

// --- Helpers ----------------------------------------------------------

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
