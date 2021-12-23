/*
Package earley provides an Earley-Parser.

Earleys algorithm for parsing ambiguous grammars has been known since 1968.
Despite its benefits, until recently it has lead a reclusive life outside
the mainstream discussion about parsers. Many textbooks on parsing do not even
discuss it (the "Dragon book" only mentions it in the appendix).

A very accessible and practical discussion has been done by Loup Vaillant
in a superb blog series (http://loup-vaillant.fr/tutorials/earley-parsing/),
and it even boasts an implementation in Lua/OcaML. (A port of Loup's ideas
in Go is available at https://github.com/jakub-m/gearley.)

I can do no better than Loup to explain the advantages of Earley-parsing:

----------------------------------------------------------------------

The biggest advantage of Earley Parsing is its accessibility. Most other tools such as
parser generators, parsing expression grammars, or combinator libraries feature
restrictions that often make them hard to use. Use the wrong kind of grammar, and your
PEG will enter an infinite loop. Use another wrong kind of grammar, and most parser
generators will fail. To a beginner, these restrictions feel most arbitrary: it looks
like it should work, but it doesn't. There are workarounds of course, but they make
these tools more complex.

Earley parsing Just Works™.

On the flip side, to get this generality we must sacrifice some speed. Earley parsing cannot
compete with speed demons such as Flex/Bison in terms of raw speed.

----------------------------------------------------------------------

If speed (or the lack thereof) is critical to your project, you should probably grab ANTLR or
Bison. I used both a lot in my programming life. However, there are many scenarios where
I wished I had a more lightweight alternative at hand. Oftentimes I found myself writing
recursive-descent parsers for small ad-hoc languages by hand, sometimes mixing them with
the lexer-part of one of the big players. My hope is that an Earley parser will prove
to be handy in these kinds of situations.

A thorough introduction to Earley-parsing may be found in
"Parsing Techniques" by  Dick Grune and Ceriel J.H. Jacobs
(https://dickgrune.com/Books/PTAPG_2nd_Edition/), section 7.2.
A recent evaluation has been done by Mark Fulbright in
"An Evaluation of Two Approaches to Parsing"
(https://apps.cs.utexas.edu/tech_reports/reports/tr/TR-2199.pdf). It references
an interesting approach to view parsing as path-finding in graphs,
by Keshav Pingali and Gianfranco Bilardi
(https://apps.cs.utexas.edu/tech_reports/reports/tr/TR-2102.pdf).

___________________________________________________________________________

License

Governed by a 3-Clause BSD license. License file may be found in the root
folder of this module.

Copyright © 2017–2021 Norbert Pillmayer <norbert@pillmayer.com>

*/
package earley

import (
	"fmt"

	"github.com/cnf/structhash"

	"github.com/npillmayer/gorgo"
	"github.com/npillmayer/gorgo/lr"
	"github.com/npillmayer/gorgo/lr/iteratable"
	"github.com/npillmayer/gorgo/lr/scanner"
	"github.com/npillmayer/gorgo/lr/sppf"
	"github.com/npillmayer/schuko/tracing"
)

// tracer traces with key 'gorgo.lr'.
func tracer() tracing.Trace {
	return tracing.Select("gorgo.lr")
}

// Parser is an Earley-parser type. Create and initialize one with earley.NewParser(...)
type Parser struct {
	ga        *lr.LRAnalysis              // the analyzed grammar we operate on
	scanner   scanner.Tokenizer           // scanner deliveres tokens
	states    []*iteratable.Set           // list of states, each a set of Earley-items
	tokens    []gorgo.Token               // we remember all input tokens, if requested
	sc        uint64                      // state counter
	mode      uint                        // flags controlling some behaviour of the parser
	Error     func(p *Parser, msg string) // Error is called for each error encountered
	forest    *sppf.Forest                // parse forest, if generated
	backlinks map[string]lr.Item          // stores backlinks for parsetree-generation
}

// NewParser creates and initializes an Earley parser.
func NewParser(ga *lr.LRAnalysis, opts ...Option) *Parser {
	p := &Parser{
		ga:        ga,
		scanner:   nil,
		states:    make([]*iteratable.Set, 1, 512), // pre-alloc first state
		tokens:    make([]gorgo.Token, 1, 512),     // pre-alloc first slot
		backlinks: make(map[string]lr.Item),
		sc:        0,
		mode:      optionStoreTokens,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// The parser consumes input symbols until the token value is EOF.
type inputSymbol struct {
	tokval int         // token value
	token  interface{} // visual representation of the symbol, if any
	span   gorgo.Span  // position and extent in the input stream
}

// http://citeseerx.ist.psu.edu/viewdoc/download?doi=10.1.1.12.4254&rep=rep1&type=pdf
// From "Practical Earley Parsing" by John Aycock and R. Nigel Horspool, 2002:
//
// Earley parsers operate by constructing a sequence of sets, sometimes called
// Earley sets. Given an input
//       x1 x2 … xn,
// the parser builds n+1 sets: an initial set S0 and one set Si foreach input
// symbol xi. Elements of these sets are referred to as (Earley) items, which
// consist of three parts: a grammar rule, a position in the right-hand side
// of the rule indicating how much of that rule has been seen and a pointer
// to an earlier Earley set. Typically Earley items are written as
//       [A→α•β, j]
// where the position in the rule’s right-hand side is denoted bya dot (•) and
// j is a pointer to set Sj.
//
// […] In terms of implementation, the Earley sets are built in increasing order as
// the input is read. Also, each set is typically represented as a list of items,
// as suggested by Earley[…]. This list representation of a set is particularly
// convenient, because the list of items acts as a ‘work queue’ when building the set:
// items are examined in order, applying Scanner, Predictor and Completer as
// necessary; items added to the set are appended onto the end of the list.

// Parse starts a new parse, given a scanner tokenizing the input.
// The parser must have been initialized with an analyzed grammar.
// It returns true if the input string has been accepted.
//
// Clients may provide a Listener to perform semantic actions.
func (p *Parser) Parse(scan scanner.Tokenizer, listener Listener) (accept bool, err error) {
	if p.scanner = scan; scan == nil {
		return false, fmt.Errorf("Earley-parser needs a valid scanner, is void")
	}
	p.scanner.SetErrorHandler(func(e error) {
		err = e
	})
	p.forest = nil
	startItem, _ := lr.StartItem(p.ga.Grammar().Rule(0)) // create S′→•S
	p.states[0] = iteratable.NewSet(0)                   // S0
	p.states[0].Add(startItem)                           // S0 = { [S′→•S, 0] }
	//tokval, token, start, len := p.scanner.NextToken(scanner.AnyToken)
	token := p.scanner.NextToken(scanner.AnyToken)
	for { // outer loop over Si per input token xi
		tracer().Debugf("Scanner read '%v|%d' @ %v", token, token.TokType(), token.Span())
		x := inputSymbol{int(token.TokType()), token, token.Span()}
		i := p.setupNextState(token)
		p.innerLoop(i, x)
		if x.tokval == scanner.EOF {
			break
		}
		//tokval, token, start, len = p.scanner.NextToken(scanner.AnyToken)
		token = p.scanner.NextToken(scanner.AnyToken)
	}
	if accept = p.checkAccept(); accept && p.hasmode(optionGenerateTree) {
		p.buildTree()
	}
	return
}

// Invariant: we're in set Si and prepare Si+1
func (p *Parser) setupNextState(token gorgo.Token) uint64 {
	// first one has already been created before outer loop
	p.states = append(p.states, iteratable.NewSet(0))
	if p.hasmode(optionStoreTokens) {
		p.tokens = append(p.tokens, token)
	}
	i := p.sc
	p.sc++
	return i // ready to operate on set Si
}

// The inner loop iterates over Si, applying Scanner, Predictor and Completer.
// The variable for Si is called S and Si+1 is called S1.
func (p *Parser) innerLoop(i uint64, x inputSymbol) {
	S := p.states[i]
	S1 := p.states[i+1]
	S.IterateOnce()
	for S.Next() {
		item := S.Item().(lr.Item)
		p.scan(S, S1, item, x.tokval) // may add items to S1
		p.predict(S, S1, item, i)     // may add items to S
		p.complete(S, S1, item, i)    // may add items to S
	}
	dumpState(p.states, i)
}

// Scanner:
// If [A→…•a…, j] is in Si and a=xi+1, add [A→…a•…, j] to Si+1
func (p *Parser) scan(S, S1 *iteratable.Set, item lr.Item, tokval int) {
	//T().Debugf("Earley scan: tokval=%d", tokval)
	if a := item.PeekSymbol(); a != nil {
		if a.Value == tokval {
			//T().Debugf("Earley: scan %s", item)
			S1.Add(item.Advance())
		}
	}
}

// Predictor:
// If [A→…•B…, j] is in Si, add [B→•α, i] to Si for all rules B→α.
// If B is nullable, also add [A→…B•…, j] to Si.
func (p *Parser) predict(S, S1 *iteratable.Set, item lr.Item, i uint64) {
	B := item.PeekSymbol()
	startitemsForB := p.ga.Grammar().FindNonTermRules(B, true)
	startitemsForB.Each(func(e interface{}) { // e is a start item
		startitem := e.(lr.Item)
		startitem.Origin = i
		S.Add(startitem)
	})
	//T().Debugf("start items from B=%s: %v", B, itemSetString(startitemsForB))
	if p.ga.DerivesEpsilon(B) { // B is nullable?
		//T().Debugf("%s is nullable", B)
		S.Add(item.Advance())
	}
}

// Completer:
// If [A→…•, j] is in Si, add [B→…A•…, k] to Si for all items [B→…•A…, k] in Sj.
func (p *Parser) complete(S, S1 *iteratable.Set, item lr.Item, i uint64) {
	if item.PeekSymbol() == nil { // dot is behind RHS
		A, j := item.Rule().LHS, item.Origin
		//T().Debugf("Completing rule for %s: %s", A, item)
		Sj := p.states[j]
		//R := Sj.Copy()
		//T().Debugf("   search predecessors: %s", itemSetString(R))
		R := Sj.Copy().Subset(func(e interface{}) bool { // find all [B→…•A…, k]
			jtem := e.(lr.Item)
			// if jtem.PeekSymbol() == A {
			// 	T().Debugf("    => %s", jtem)
			// }
			return jtem.PeekSymbol() == A
		})
		//T().Debugf("   found predecessors: %s", itemSetString(R))
		R.Each(func(e interface{}) { // now add [B→…A•…, k]
			jtem := e.(lr.Item)
			if jadv := jtem.Advance(); jadv != lr.NullItem {
				if jadv.PeekSymbol() == nil {
					//T().Errorf("%v COMPLETED DUE TO %v", jadv, item)
					// store this backlink for later parsetree generation
					h := hash(jadv, i)
					p.backlinks[h] = item
				}
				S.Add(jadv)
			}
		})
	}
}

// checkAccepts searches the final state for items with a dot after #eof
// and a LHS of the start rule.
// It returns true if an accepting item has been found, indicating that the
// input has been recognized.
func (p *Parser) checkAccept() bool {
	dumpState(p.states, p.sc)
	S := p.states[p.sc] // last state should contain accept item
	S.IterateOnce()
	acc := false
	for S.Next() {
		item := S.Item().(lr.Item)
		if item.PeekSymbol() == nil && item.Rule().LHS == p.ga.Grammar().Rule(0).LHS {
			tracer().Debugf("ACCEPT: %s", item)
			acc = true
		}
	}
	return acc
}

// ParseForest returns the parse forest for the last Parse-run, if any.
// Parser option GenerateTree must have been set to true at parser-creation time.
// In case of serious parsing errors, generation of a forest may have been abandoned
// by the parser.
func (p *Parser) ParseForest() *sppf.Forest {
	return p.forest
}

// Build a parse forest from the derivation produced during a parse run.
// We use a special derivation walker TreeBuilder, which creates an SPPF.
func (p *Parser) buildTree() error {
	builder := NewTreeBuilder(p.ga.Grammar())
	root := p.WalkDerivation(builder)
	_, ok := root.Value.(*sppf.SymbolNode)
	if !ok || root.Symbol().Name != "S'" { // should have reduced top level rule
		p.forest = nil
		if root == nil {
			return fmt.Errorf("returned parse forest is empty")
		}
		return fmt.Errorf("Expected root node of forest to be start symbol, is %v", root.Symbol())
	}
	p.forest = builder.Forest()
	return nil
}

// Remark about possible optimizations:  Once again take a look at
// http://citeseerx.ist.psu.edu/viewdoc/download?doi=10.1.1.12.4254&rep=rep1&type=pdf
// "Practical Earley Parsing" by John Aycock and R. Nigel Horspool, 2002:
//
// Aycock and Horspool describe a state machine, the split 𝜖-DFA, which guides
// the parse and boosts performance for practical purposes. It stems from an LR(0)
// CFSM, which for LR-parsing we (kind of) calculate anyway (see package lr). Coding
// the split 𝜖-DFA and adapting the parse algorithm certainly seems doable.
//
// However, currently I do not plan to implement any of this. Constructing the
// parse tree would get more complicated and I'm not sure I fully comprehend the paper
// of Aycock and Horspool in this regard (actually I *am* sure: I don't). I'd certainly
// had to experiment a lot to make practical use of it. Thus I am investing my time
// elsewhere, for now.

// --- Option handling --------------------------------------------------

// Option configures a parser.
type Option func(p *Parser)

const (
	optionStoreTokens  uint = 1 << 1 // store all input tokens, defaults to true
	optionGenerateTree uint = 1 << 2 // if parse was successful, generate a parse forest (default false)
)

// StoreTokens configures the parser to remember all input tokens. This is
// necessary for listeners during tree walks to have access to the values/tokens
// of non-terminals. Defaults to true.
func StoreTokens(b bool) Option {
	return func(p *Parser) {
		if !p.hasmode(optionGenerateTree) && b ||
			p.hasmode(optionGenerateTree) && !b {
			p.mode |= optionStoreTokens
		}
	}
}

// GenerateTree configures the parser to create a parse tree/forest for
// a successful parse. Defaults to false.
func GenerateTree(b bool) Option {
	return func(p *Parser) {
		if !p.hasmode(optionGenerateTree) && b ||
			p.hasmode(optionGenerateTree) && !b {
			p.mode |= optionGenerateTree
		}
	}
}

func (p *Parser) hasmode(m uint) bool {
	return p.mode&m > 0
}

// --- Helpers ---------------------------------------------------------------

func hash(i lr.Item, stateno uint64) string {
	hash, err := structhash.Hash(struct {
		item  lr.Item
		state uint64
	}{ // put it in an anonymous struct
		item:  i,
		state: stateno,
	}, 1)
	if err != nil { // no reason for this to happend, but API demands it
		panic(err)
	}
	return hash
}
