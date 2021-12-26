package termr

import (
	"strings"
	"testing"

	"github.com/npillmayer/gorgo"
	"github.com/npillmayer/gorgo/lr"
	"github.com/npillmayer/gorgo/lr/earley"
	"github.com/npillmayer/gorgo/lr/scanner"
	"github.com/npillmayer/gorgo/lr/sppf"
	"github.com/npillmayer/gorgo/terex"
	"github.com/npillmayer/schuko/tracing"
	"github.com/npillmayer/schuko/tracing/gotestingadapter"
)

func TestAST1(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "gorgo.terex")
	defer teardown()
	//
	b := lr.NewGrammarBuilder("TermR")
	b.LHS("E").N("E").T("+", '+').T("a", scanner.Ident).End()
	b.LHS("E").T("a", scanner.Ident).End()
	G, _ := b.Grammar()
	ga := lr.Analysis(G)
	parser := earley.NewParser(ga, earley.GenerateTree(true))
	input := strings.NewReader("a+a")
	scanner := scanner.GoTokenizer("TestAST", input)
	acc, err := parser.Parse(scanner, nil)
	if !acc || err != nil {
		t.Errorf("parser could not parse input")
	}
	// tmpfile, _ := ioutil.TempFile(".", "tree-*.dot")
	// sppf.ToGraphViz(parser.ParseForest(), tmpfile)
	tracing.Select("gorgo.terex").SetTraceLevel(tracing.LevelDebug)
	ab := NewASTBuilder(G)
	env := ab.AST(parser.ParseForest(), earleyTokenReceiver(parser))
	//expected := `(:a :+ :a :#eof)`
	expected := `(-2 43 -2 -1)`
	if env == nil || env.AST == nil || env.AST.Cdr == nil {
		t.Errorf("AST is empty")
	} else {
		if env.AST.ListString() != expected {
			t.Errorf("AST should be %s, is %s", expected, env.AST.ListString())
		}
	}
}

func TestAST2(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "gorgo.terex")
	defer teardown()
	//
	b := lr.NewGrammarBuilder("TermR")
	b.LHS("E").N("E").T("+", '+').T("a", scanner.Ident).End()
	b.LHS("E").T("a", scanner.Ident).End()
	G, _ := b.Grammar()
	ga := lr.Analysis(G)
	parser := earley.NewParser(ga, earley.GenerateTree(true))
	input := strings.NewReader("a+a")
	scanner := scanner.GoTokenizer("TestAST", input)
	acc, err := parser.Parse(scanner, nil)
	if !acc || err != nil {
		t.Errorf("parser could not parse input")
	}
	// tmpfile, _ := ioutil.TempFile(".", "tree-*.dot")
	// sppf.ToGraphViz(parser.ParseForest(), tmpfile)
	tracing.Select("gorgo.terex").SetTraceLevel(tracing.LevelDebug)
	builder := NewASTBuilder(G)
	builder.AddTermR(makeOp("E"))
	env := builder.AST(parser.ParseForest(), earleyTokenReceiver(parser))
	//expected := `((#E (#E :a) :+ :a) :#eof)`
	expected := `((#E (#E -2) 43 -2) -1)`
	if env == nil || env.AST.Cdr == nil {
		t.Errorf("AST is empty")
	} else if env.AST.ListString() != expected {
		t.Errorf("AST should be %s, is %s", expected, env.AST.ListString())
	}
}

func earleyTokenReceiver(parser *earley.Parser) gorgo.TokenRetriever {
	return func(pos uint64) gorgo.Token {
		return parser.TokenAt(pos)
	}
}

// ---------------------------------------------------------------------------

type testOp struct {
	name string
}

func (op *testOp) Rewrite(list *terex.GCons, env *terex.Environment) terex.Element {
	tracer().Debugf(env.Dump())
	return terex.Elem(list)
}

func (op *testOp) Descend(sppf.RuleCtxt) bool {
	return true
}

func (op *testOp) Name() string {
	return op.name
}

func (op *testOp) String() string {
	return op.name
}

func (op *testOp) Operator() terex.Operator {
	return op
}

func (op *testOp) Call(el terex.Element, env *terex.Environment) terex.Element {
	return terex.Elem(nil)
}

func (op *testOp) Quote(el terex.Element, env *terex.Environment) terex.Element {
	return el
}

func makeOp(name string) *testOp {
	return &testOp{
		name: name,
	}
}

var _ terex.Operator = makeOp("Hello")
