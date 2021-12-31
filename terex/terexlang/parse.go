package terexlang

/*
License

Governed by a 3-Clause BSD license. License file may be found in the root
folder of this module.

Copyright © 2017–2021 Norbert Pillmayer <norbert@pillmayer.com>

*/

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/npillmayer/gorgo"
	"github.com/npillmayer/gorgo/lr"
	"github.com/npillmayer/gorgo/lr/earley"
	"github.com/npillmayer/gorgo/lr/scanner"
	"github.com/npillmayer/gorgo/lr/scanner/lexmach"
	"github.com/npillmayer/gorgo/lr/sppf"
	"github.com/npillmayer/gorgo/terex"
	"github.com/npillmayer/gorgo/terex/termr"
	"github.com/npillmayer/schuko/tracing"
)

// --- Grammar ---------------------------------------------------------------

// Atom       ::=  '\'' Atom         // currently un-ambiguated by QuoteOrAtom
// Atom       ::=  ident             // a
// Atom       ::=  string            // "abc"
// Atom       ::=  number            // 123.45
// Atom       ::=  variable          // $a
// Atom       ::=  List
// List       ::=  '(' Sequence ')'
// Sequence   ::=  Sequence Atom
// Sequence   ::=  Atom
//
// Comments starting with ';' will be filtered by the scanner.
//
func makeTeRExGrammar() (*lr.LRAnalysis, error) {
	b := lr.NewGrammarBuilder("TeREx S-Expr")
	b.LHS("QuoteOrAtom").N("Quote").End()
	b.LHS("QuoteOrAtom").N("Atom").End()
	b.LHS("Quote").T(Token("'")).N("Atom").End()
	b.LHS("Atom").T(Token("ID")).End()
	b.LHS("Atom").T(Token("STRING")).End()
	b.LHS("Atom").T(Token("NUM")).End()
	b.LHS("Atom").T(Token("VAR")).End()
	b.LHS("Atom").N("List").End()
	b.LHS("List").T(Token("(")).N("Sequence").T(Token(")")).End()
	b.LHS("Sequence").N("QuoteOrAtom").N("Sequence").End()
	b.LHS("Sequence").Epsilon()
	g, err := b.Grammar()
	if err != nil {
		return nil, err
	}
	return lr.Analysis(g), nil
}

var grammar *lr.LRAnalysis
var lexer *lexmach.LMAdapter

var startOnce sync.Once // monitors one-time creation of grammar and lexer

func createParser() *earley.Parser {
	startOnce.Do(func() {
		var err error
		tracer().Infof("Creating lexer")
		if lexer, err = Lexer(); err != nil { // MUST be called before grammar builing !
			panic("Cannot create lexer")
		}
		tracer().Infof("Creating grammar")
		if grammar, err = makeTeRExGrammar(); err != nil {
			panic("Cannot create global grammar")
		}
		initRewriters()
	})
	return earley.NewParser(grammar, earley.GenerateTree(true))
}

// NewASTBuilder returns a new AST builder for the TeREx language
func newASTBuilder() *termr.ASTBuilder {
	ab := termr.NewASTBuilder(grammar.Grammar())
	//ab.AddTermR(opOp)
	ab.AddRewriter(quoteOp.name, quoteOp)
	ab.AddRewriter(seqOp.name, seqOp)
	ab.AddRewriter(listOp.name, listOp)
	atomOp := makeASTTermR("Atom", "atom")
	atomOp.rewrite = func(l *terex.GCons, env *terex.Environment) terex.Element {
		// rewrite: (:atom x)  => x
		e := terex.Elem(l.Cdar())
		// in (atom x), check if x is terminal
		tracer().Infof("atomOp.rewrite: l.Cdr = %v", e)
		if l.Cdr.IsLeaf() {
			e = setTerminalTokenValue(e, ab.Env)
		}
		tracer().Infof("atomOp.rewrite => %s", e.String())
		return e
	}
	ab.AddRewriter(atomOp.name, atomOp)
	return ab
}

// Parse parses an input string, given in TeREx language format. It returns the
// parse forest and a TokenRetriever, or an error in case of failure.
//
// Clients may use a terex.ASTBuilder to create an abstract syntax tree
// from the parse forest.
//
func Parse(input string) (*sppf.Forest, gorgo.TokenRetriever, error) {
	parser := createParser()
	scan, err := lexer.Scanner(input)
	if err != nil {
		return nil, nil, err
	}
	accept, err := parser.Parse(scan, nil)
	if err != nil {
		return nil, nil, err
	} else if !accept {
		return nil, nil, fmt.Errorf("not a valid TeREx expression")
	}
	return parser.ParseForest(), earleyTokenReceiver(parser), nil
}

func earleyTokenReceiver(parser *earley.Parser) gorgo.TokenRetriever {
	return func(pos uint64) gorgo.Token {
		return parser.TokenAt(pos)
	}
}

// AST creates an abstract syntax tree from a parse tree/forest.
//
// Returns a homogenous AST, a TeREx-environment and an error status.
func AST(parsetree *sppf.Forest, tokRetr gorgo.TokenRetriever) (*terex.GCons, *terex.Environment, error) {
	ab := newASTBuilder()
	env := ab.AST(parsetree, tokRetr)
	if env == nil {
		tracer().Errorf("Cannot create AST from parsetree")
		return nil, nil, fmt.Errorf("error while creating AST")
	}
	ast := env.AST
	tracer().Infof("AST: %s", env.AST.ListString())
	return ast, env, nil
}

// ---------------------------------------------------------------------------

// QuoteAST returns an AST, which should be the result of parsing an s-expr, as
// pure data.
//
// If the environment contains any symbol's value, quoting will replace the symbol
// by its value. For example, if the s-expr contains a symbol 'str' with a value
// of "this is a string", the resulting data structure will contain the string,
// not the name of the symbol. If you do not have use for this kind of substitution,
// simply call Quote(…) for the global environment.
//
func QuoteAST(ast terex.Element, env *terex.Environment) (terex.Element, error) {
	// ast *terex.GCons
	if env == nil {
		env = terex.GlobalEnvironment
	}
	quEnv := terex.NewEnvironment("quoting", env)
	quEnv.Defn("list", listOp.call)
	//quEnv.Defn("quote", quoteOp.call)
	quEnv.Resolver = symbolPreservingResolver{}
	q := terex.Eval(ast, quEnv)
	tracer().Debugf("QuotAST returns Q = %v", q)
	q.Dump(tracing.LevelDebug)
	return q, quEnv.LastError()
}

// --- S-expr AST-builder listener -------------------------------------------

var quoteOp *sExprRewriter // for Quote -> ... productions
var seqOp *sExprRewriter   // for Sequence -> ... productions
var listOp *sExprRewriter  // for List -> ... productions

type sExprRewriter struct {
	name    string
	opname  string
	rewrite func(*terex.GCons, *terex.Environment) terex.Element
	call    func(terex.Element, *terex.Environment) terex.Element
}

var _ terex.Operator = &sExprRewriter{}
var _ termr.TermRewriter = &sExprRewriter{}

//func makeASTTermR(name string, opname string, quoter bool) *sExprTermR {
func makeASTTermR(name string, opname string) *sExprRewriter {
	termr := &sExprRewriter{
		name:   name,
		opname: opname,
	}
	return termr
}

func (trew *sExprRewriter) String() string {
	return trew.name
}

func (trew *sExprRewriter) OperatorFor(gsym string) terex.Operator {
	return trew
}

func (trew *sExprRewriter) Rewrite(l *terex.GCons, env *terex.Environment) terex.Element {
	tracer().Debugf("%s:trew.Rewrite[%s] called", trew.String(), l.ListString())
	e := trew.rewrite(l, env)
	// T().Debugf("%s:Op.Rewrite[%s] called, %d rules", op.Name(), l.ListString(), len(op.rules))
	// for _, rule := range op.rules {
	// 	T().Infof("match: trying %s %% %s ?", rule.Pattern.ListString(), l.ListString())
	// 	if rule.Pattern.Match(l, env) {
	// 		T().Infof("Op %s has a match", op.Name())
	// 		//T().Debugf("-> pre rewrite: %s", l.ListString())
	// 		v := rule.Rewrite(l, env)
	// 		//T().Debugf("<- post rewrite:")
	// 		terex.DumpElement(v)
	// 		T().Infof("Op %s rewrite -> %s", op.Name(), v.String())
	// 		//return rule.Rewrite(l, env)
	// 		return v
	// 	}
	// }
	return e
}

func (trew *sExprRewriter) Descend(sppf.RuleCtxt) bool {
	return true
}

func (trew *sExprRewriter) Call(e terex.Element, env *terex.Environment) terex.Element {
	opsym := env.FindSymbol(trew.opname, true)
	if opsym == nil {
		tracer().Errorf("Cannot find parsing operation %s", trew.opname)
		return e
	}
	operator, ok := opsym.Value.AsAtom().Data.(terex.Operator)
	if !ok {
		tracer().Errorf("Cannot call parsing operation %s", trew.opname)
		return e
	}
	return operator.Call(e, env)
}

func initRewriters() {
	// SingleTokenArg = terex.Cons(terex.Atomize(terex.OperatorType), termr.AnySymbol())
	// opOp = makeASTTermR("Op", "").Rule(termr.Anything(), func(l *terex.GCons, env *terex.Environment) terex.Element {
	// 	if l.Length() <= 1 || l.Cdar().Type() != terex.TokenType {
	// 		return terex.Elem(l)
	// 	}
	// 	// (op "x") => x:op
	// 	tname := l.Cdar().Data.(*terex.Token).String()
	// 	if sym := terex.GlobalEnvironment.FindSymbol(tname, true); sym != nil {
	// 		if sym.Value.Type() == terex.OperatorType {
	// 			op := terex.Atomize(&globalOpInEnv{tname})
	// 			return terex.Elem(op)
	// 		}
	// 	}
	// 	return terex.Elem(l)
	// })
	// _, tokval := Token("'")
	// p := terex.Cons(terex.Atomize(terex.OperatorType),
	// 	terex.Cons(terex.Atomize(&terex.Token{Name: "'", TokType: tokval}), termr.AnySymbol()))
	quoteOp = makeASTTermR("Quote", "list")
	quoteOp.rewrite = func(l *terex.GCons, env *terex.Environment) terex.Element {
		// (:quote ' ⟨atom⟩) =>  (:list quote ⟨atom⟩)
		q := env.Intern("quote", false)
		qu := terex.Cons(terex.Atomize(q), l.Cddr())
		//quotedList := terex.Elem(terex.Cons(l.Car, qu))
		op := listOp.OperatorFor("quote")
		quotedList := terex.Elem(terex.Cons(terex.Atomize(op), qu))
		//quotedList := terex.Elem(terex.Cons(terex.Atomize(quoteOp.Operator()), qu))
		//quotedList.Dump(tracing.LevelDebug)
		return quotedList
	}
	// quoteOp.call = func(e terex.Element, env *terex.Environment) terex.Element {
	// 	T().Debugf("Un-QUOTE of %v", e)
	// 	// :quote(atom) =>  atom
	// 	return e
	// }
	seqOp = makeASTTermR("Sequence", "seq")
	seqOp.rewrite = func(l *terex.GCons, env *terex.Environment) terex.Element {
		switch l.Length() {
		case 0:
			return terex.Elem(nil)
		case 1:
			return terex.Elem(nil)
		case 2:
			if l.Cdar().Type() == terex.ConsType {
				return terex.Elem(l.Cdr.Tee())
			} // TODO: keep or drop this? Unclear
			return terex.Elem(l.Cdar())
		}
		return terex.Elem(l.Cdr)
	}
	listOp = makeASTTermR("List", "list")
	listOp.rewrite = func(l *terex.GCons, env *terex.Environment) terex.Element {
		// list '(' x y ... ')'  => (:list x y ...)
		// if l.Length() <= 3 { // ( )
		// 	return terex.Elem(l.Car)
		// }
		content := l.Cddr()                            // strip '('
		content = content.FirstN(content.Length() - 1) // strip ')'
		tracer().Debugf("List content = %v", content)
		return terex.Elem(terex.Cons(l.Car, content)) // (List:Op ...)
	}
	listOp.call = func(e terex.Element, env *terex.Environment) terex.Element {
		// (:list a b c) =>  (a b c)
		list := e.AsList()
		tracer().Debugf("========= Un-LIST of %v  =  %s", e, list.ListString())
		if list.Length() == 0 { //  () => nil  [empty list is nil]
			return terex.Elem(nil)
		}
		// if list.Length() == 1 {
		// 	mapped := terex.EvalAtom(terex.Elem(list.Car), env)
		// 	T().Errorf("========= => %v", mapped)
		// 	return mapped
		// }
		//list := args.Map(terex.Eval, env) // eval arguments
		//e = terex.Eval(terex.Elem(list), env)
		listElements := list.Map(terex.EvalAtom, env)
		terex.Elem(listElements).Dump(tracing.LevelDebug)
		// if e.IsAtom() {
		// 	T().Errorf("========= type is atom %s", e.AsAtom().Type().String())
		// 	return e
		// }
		// T().Errorf("========= type is list")
		//ee := terex.Cons(terex.Atomize(e.AsList()), nil)
		tracer().Debugf("========= => %v", terex.Elem(listElements))
		return terex.Elem(listElements)
	}
}

// ---------------------------------------------------------------------------

func setTerminalTokenValue(el terex.Element, env *terex.Environment) terex.Element {
	if !el.IsAtom() {
		return el
	}
	atom := el.AsAtom()
	if atom.Type() != terex.TokenType {
		return el
	}
	token := atom.Data.(gorgo.Token)
	deftok := token.(scanner.DefaultToken)
	tracer().Infof("set value of terminal token: '%v'", string(token.Lexeme()))
	switch int(token.TokType()) {
	case tokenIds["NUM"]:
		if f, err := strconv.ParseFloat(string(token.Lexeme()), 64); err == nil {
			tracer().Debugf("   t.Value=%g", f)
			deftok.Val = f
		} else {
			tracer().Errorf("   %s", err.Error())
			return terex.Elem(terex.Atomize(err))
		}
	case tokenIds["STRING"]:
		if (len(token.Lexeme())) <= 2 {
			deftok.Val = ""
		} else { // trim off "…"
			deftok.Val = string(token.Lexeme()[1 : len(token.Lexeme())-1])
		}
	case tokenIds["VAR"]:
		panic("VAR type tokens not yet implemented")
		//fallthrough
	case tokenIds["ID"]:
		s := string(token.Lexeme())
		//sym := terex.GlobalEnvironment.Intern(s, true)
		sym := env.Intern(s, true)
		if sym != nil {
			return terex.Elem(terex.Atomize(sym))
		}
	default:
	}
	return el
}

type symbolPreservingResolver struct{}

func (r symbolPreservingResolver) Resolve(atom terex.Atom, env *terex.Environment, asOp bool) (
	terex.Element, error) {
	if atom.Type() == terex.TokenType {
		token := atom.Data.(gorgo.Token)
		tracer().Debugf("Resolve terminal token: '%v'", string(token.Lexeme()))
		switch int(token.TokType()) {
		case tokenIds["NUM"]:
			return terex.Elem(token.Value().(float64)), nil
		case tokenIds["STRING"]:
			return terex.Elem(token.Value().(string)), nil
		}
	}
	return terex.Elem(atom), nil
}

var _ terex.SymbolResolver = symbolPreservingResolver{}
