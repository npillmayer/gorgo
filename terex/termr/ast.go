package termr

/*
License

Governed by a 3-Clause BSD license. License file may be found in the root
folder of this module.

Copyright © 2017–2021 Norbert Pillmayer <norbert@pillmayer.com>

*/

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/npillmayer/gorgo"
	"github.com/npillmayer/gorgo/lr"
	"github.com/npillmayer/gorgo/lr/sppf"
	"github.com/npillmayer/gorgo/terex"
)

// ASTBuilder is a parse tree listener for building ASTs. It will construct an
// abstract syntax tree (AST) while walking the parse tree. As we may support
// ambiguous grammars, the parse tree may happen to be a parse forest, containing
// more than one derivation for a sentence.
type ASTBuilder struct {
	G                *lr.Grammar             // input grammar the parse forest stems from
	Env              *terex.Environment      // environment for symbols of the AST to create
	forest           *sppf.Forest            // input parse tree/forest
	conflictStrategy sppf.Pruner             // how to deal with parse-forest ambiguities
	toks             gorgo.TokenRetriever    // retriever for parse tree leafs
	rewriters        map[string]TermRewriter // term rewriters to apply
	Error            func(error)             // user supplied handler for errors
	stack            []*terex.GCons          // used for recursive operator walking
}

// ErrorHandler is an interface to process errors occuring during parsing.
type ErrorHandler interface {
	Error(error) bool
}

// NewASTBuilder creates an AST builder from a parse forest/tree.
// It is initialized with the grammar, which has been used to create the parse tree.
//
// Clients will first create an ASTBuilder, then initialize it with all the
// term rewriters and variables/symbols necessary, and finally call ASTBuilder.AST(…).
func NewASTBuilder(g *lr.Grammar) *ASTBuilder {
	if g == nil {
		tracer().Errorf("Grammar may not be nil")
		return nil
	}
	ab := &ASTBuilder{
		G:         g,
		Env:       terex.NewEnvironment("AST "+g.Name, terex.GlobalEnvironment),
		stack:     make([]*terex.GCons, 0, 256),
		rewriters: make(map[string]TermRewriter),
	}
	return ab
}

// TermRewriter is a type for a rewriter for AST creation and transformation.
type TermRewriter interface {
	Rewrite(*terex.GCons, *terex.Environment) terex.Element // term rewriting
	Descend(sppf.RuleCtxt) bool                             // predicate wether to descend to children nodes
	OperatorFor(string) terex.Operator                      // operator (for symbol) to place as sub-tree node
}

// AddRewriter adds an AST rewriter for a non-terminal grammar symbol to the builder.
func (ab *ASTBuilder) AddRewriter(grammarSymbol string, trew TermRewriter) {
	if trew != nil {
		tracer().Infof("Adding term rewriter for symbol %s", grammarSymbol)
		ab.rewriters[grammarSymbol] = trew
	}
}

// AST creates an abstract syntax tree from a parse tree/forest.
// The type of ASTs we create is a homogenous abstract syntax tree.
func (ab *ASTBuilder) AST(parseTree *sppf.Forest, tokRetr gorgo.TokenRetriever) *terex.Environment {
	if parseTree == nil || tokRetr == nil {
		return nil
	}
	ab.forest = parseTree
	ab.toks = tokRetr
	cursor := ab.forest.SetCursor(nil, nil) // TODO set Pruner
	value := cursor.TopDown(ab, sppf.LtoR, sppf.Break)
	tracer().Infof("AST creation return value = %v", value)
	if value != nil {
		ab.Env.AST = value.(terex.Element).AsList()
		tracer().Infof("AST = %s", ab.Env.AST.ListString())
	}
	return ab.Env
}

// --- sppf.Listener interface -----------------------------------------------

var _ sppf.Listener = (*ASTBuilder)(nil)

// EnterRule is part of sppf.Listener interface.
// Not intended for direct client use.
func (ab *ASTBuilder) EnterRule(sym *lr.Symbol, rhs []*sppf.RuleNode, ctxt sppf.RuleCtxt) bool {
	if rew, ok := ab.rewriters[sym.Name]; ok {
		if !rew.Descend(ctxt) {
			return false
		}
		tracer().Debugf("-------> enter operator symbol: %v", sym)
		op := rew.OperatorFor(sym.Name)
		opSymListStart := terex.Cons(terex.Atomize(op), nil)
		ab.stack = append(ab.stack, opSymListStart) // put '(op ... ' on stack
	} else {
		tracer().Debugf("-------> enter grammar symbol: %v", sym)
	}
	return true
}

// ExitRule is part of sppf.Listener interface.
// Not intended for direct client use.
func (ab *ASTBuilder) ExitRule(sym *lr.Symbol, rhs []*sppf.RuleNode, ctxt sppf.RuleCtxt) interface{} {
	tracer().Debugf("<------- exit symbol: %v, now rewriting", sym)
	if op, ok := ab.rewriters[sym.Name]; ok {
		env, err := ab.EnvironmentForGrammarRule(sym.Name, rhs)
		if err != nil && ab.Error != nil {
			ab.Error(err)
		}
		rhsList := ab.stack[len(ab.stack)-1] // operator is TOS ⇒ first element of RHS list
		for _, r := range rhs {              // collect all the value of RHS symbols
			tracer().Debugf("r = %v", r)
			rhsList = appendRHSResult(rhsList, r)
		}
		tracer().Infof("%s: Rewrite of %s", sym.Name, rhsList.ListString())
		rewritten := op.Rewrite(rhsList, env) // returns a terex.Element
		ab.stack = ab.stack[:len(ab.stack)-1] // pop initial '(op ...'
		tracer().Infof("%s returns %s", sym.Name, rewritten.String())
		return rewritten
	}
	var list *terex.GCons
	tracer().Infof("%s will rewrite |rhs| = %d symbols", sym.Name, len(rhs))
	for _, r := range rhs {
		list = appendRHSResult(list, r)
	}
	rew := noOpRewrite(list)
	tracer().Infof("%s returns %s", sym.Name, rew.String())
	tracer().Infof("done with rewriting grammar symbol: %v", sym)
	return rew
}

func noOpRewrite(list *terex.GCons) terex.Element {
	tracer().Debugf("no-op rewrite of %v", list)
	if list != nil && list.Length() == 1 {
		return terex.Elem(list.Car)
	}
	return terex.Elem(list)
}

func appendRHSResult(list *terex.GCons, r *sppf.RuleNode) *terex.GCons {
	if _, ok := r.Value.(terex.Element); !ok {
		tracer().Errorf("r.Value=%v", r.Value)
		panic("RHS symbol is not of type Element")
	}
	e := r.Value.(terex.Element) // value of rule-node r is either atom or list
	// T().Debugf("RHS-appending value = %s", e.String())
	if e.IsNil() {
		return list
	}
	if e.IsAtom() {
		list = list.Append(terex.Cons(e.AsAtom(), nil))
		return list
	}
	l := e.AsList()
	if l.Car.Type() == terex.OperatorType {
		list = list.Branch(l)
	} else {
		list = list.Append(l)
	}
	return list
}

//func growRHSList(start, end *terex.GCons, r *sppf.RuleNode, env *terex.Environment) (*terex.GCons, *terex.GCons) {
/* func growRHSList(start, end *terex.GCons, r *sppf.RuleNode, env *terex.Environment) (*terex.GCons, *terex.GCons) {
	if _, ok := r.Value.(terex.Element); !ok {
		T().Errorf("r.Value=%v", r.Value)
		panic("RHS symbol is not of type Element")
	}
	e := r.Value.(terex.Element) // value of rule-node r is either atom or list
	if e.IsNil() {
		return start, end
	}
	if e.IsAtom() {
		end = (appendAtom(end, e.AsAtom()))
		if start == nil {
			start = end
		}
	} else {
		l := e.AsList()
		if l.Car.Type() == terex.OperatorType {
			//T().Infof("%s: tee appending %v", sym, v.ListString())
			end = appendTee(end, l)
			if start == nil {
				start = end
			}
		} else { // append l at end of current list
			var concat *terex.GCons
			//T().Infof("%s: inline appending %v", sym, v.ListString())
			concat, end = appendList(end, l)
			if start == nil {
				start = concat
			}
		}
	}
	return start, end
} */

// Terminal is part of sppf.Listener interface.
// Not intended for direct client use.
func (ab *ASTBuilder) Terminal(tokval gorgo.TokType, terminal *lr.Symbol, ctxt sppf.RuleCtxt) interface{} {
	// T().Errorf("TERMINAL TOKEN FOR %d ------------------", tokval)
	// T().Errorf("SPAN = %d", ctxt.Span.Len())
	if ctxt.Span.Len() == 0 { // RHS is epsilon
		return terex.Elem(nil)
	}
	//terminal := ab.G.Terminal(tokval)
	tokpos := ctxt.Span.Start()
	t := ab.toks(tokpos) // opaque token type
	//atom := terex.Atomize(&terex.Token{Name: terminal.Name, TokType: tokval, Token: t})
	if t == nil {
		t = ersatzToken{
			kind:   gorgo.TokType(tokval),
			lexeme: terminal.Name,
			span:   ctxt.Span,
		}
	}
	//atom := terex.Atomize(&terex.Token{Name: terminal.Name, Token: t})
	atom := terex.Atomize(t)
	tracer().Debugf("CONS(terminal=%s) = %v @%d", terminal.Name, atom, tokpos)
	return terex.Elem(atom)
}

// Conflict is part of sppf.Listener interface.
// Not intended for direct client use.
func (ab *ASTBuilder) Conflict(sym *lr.Symbol, ctxt sppf.RuleCtxt) (int, error) {
	panic("Conflict of AST building not yet implemented")
	//return 0, nil
}

// MakeAttrs is part of sppf.Listener interface.
// Not intended for direct client use.
func (ab *ASTBuilder) MakeAttrs(*lr.Symbol) interface{} {
	return nil // TODO
}

// EnvironmentForGrammarSymbol creates an empty new environment, suitable for the
// grammar symbols at a given tree node of a parse-tree or AST.
func (ab *ASTBuilder) environmentForGrammarSymbol(symname string) (*terex.Environment, error) {
	if ab.G == nil {
		return terex.GlobalEnvironment, errors.New("Grammar is null")
	}
	envname := "#" + symname
	//if env := terex.GlobalEnvironment.FindSymbol(envname, false); env != nil {
	/* 	if env := ab.Env.FindSymbol(envname, false); env != nil {
		if env.Value.Type() != terex.EnvironmentType {
			panic(fmt.Errorf("Internal error, environment misconstructed: %s", envname))
		}
		return env.Value.Data.(*terex.Environment), nil
	} */
	env := terex.NewEnvironment(envname, ab.Env)
	gsym := ab.G.SymbolByName(symname)
	if gsym == nil || gsym.IsTerminal() {
		//return terex.GlobalEnvironment, fmt.Errorf("Non-terminal not found in grammar: %s", symname)
		return env, fmt.Errorf("Non-terminal not found in grammar: %s", symname)
	}
	/* 	rhsSyms := iteratable.NewSet(0)
	   	rules := ab.G.FindNonTermRules(gsym, false)
	   	rules.IterateOnce()
	   	for rules.Next() {
	   		rule := rules.Item().(lr.Item).Rule()
	   		for _, s := range rule.RHS() {
	   			rhsSyms.Add(s)
	   		}
	   	}
	   	rhsSyms.IterateOnce()
	   	for rhsSyms.Next() {
	   		gsym := rhsSyms.Item().(*lr.Symbol)
	   		sym := env.Intern(gsym.Name, false)
	   		if gsym.IsTerminal() {
	   			sym.Value = terex.Atomize(gsym.Value)
	   		}
	   	} */
	// e := globalEnvironment.Intern(envname, false)
	// e.atom.typ = EnvironmentType
	// e.atom.Data = env
	return env, nil
}

// EnvironmentForGrammarRule creates a new environment, suitable for the
// grammar symbols at a given tree node of a parse-tree or AST.
//
// Given a grammar production
//
//     A -> B C D
//
// it will create an environment #A for A, with pre-interned (but empty) symbols
// for A, B, C and D. If any of the right-hand-side symbols are terminals, they will
// be created as nodes with an appropriate atom type.
//
// For recurring non-terminal RHS symbols, as in
//
//     A -> B + B
//
// sequence number suffixes will be created. The grammar rule above will produce an
// environment containing
//
//     B, B.1   for the 1st B
//     B.2      for the 2nd B
//
func (ab *ASTBuilder) EnvironmentForGrammarRule(symname string, rhs []*sppf.RuleNode) (*terex.Environment, error) {
	env, err := ab.environmentForGrammarSymbol(symname)
	if err != nil {
		return env, err
	}
	symtrack := make(map[string]int)
	for _, r := range rhs { // set value of RHS vars in Env
		// T().Debugf("RHS: r = %v", r)
		symname := r.Symbol().Name
		count := symtrack[symname]
		envsym := env.Intern(symname+"."+strconv.Itoa(count), true)
		setSymbolValue(envsym, r)
		if count == 0 {
			//envsym := env.Intern(r.Symbol().Name, true)
			envsym = env.Intern(symname, true)
			setSymbolValue(envsym, r)
		}
		symtrack[symname] = count + 1
		tracer().Debugf("env sym = %v", envsym)
	}
	return env, nil
}

// --- Helpers ---------------------------------------------------------------

func setSymbolValue(envsym *terex.Symbol, r *sppf.RuleNode) {
	envsym.Value = terex.Elem(r.Value)
	// if r.Symbol().IsTerminal() {
	// 	envsym.Value = terex.Elem(r.Value)
	// } else {
	// 	envsym.Value = terex.Atomize(r.Value)
	// }
}

// ersatzToken is a very unsophisticated token type used for terminal tokens.
// It is used in case the caller is unable to provide the initially read input token.
type ersatzToken struct {
	kind   gorgo.TokType
	lexeme string
	span   gorgo.Span
}

func (t ersatzToken) TokType() gorgo.TokType {
	return t.kind
}

func (t ersatzToken) Lexeme() string {
	return t.lexeme
}

func (t ersatzToken) Value() interface{} {
	return nil
}

func (t ersatzToken) Span() gorgo.Span {
	return t.span
}
