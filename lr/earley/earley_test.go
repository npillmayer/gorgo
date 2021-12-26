package earley

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/npillmayer/gorgo"
	"github.com/npillmayer/schuko/tracing"
	"github.com/npillmayer/schuko/tracing/gotestingadapter"

	"github.com/npillmayer/gorgo/lr"
	"github.com/npillmayer/gorgo/lr/scanner"
	"github.com/npillmayer/gorgo/lr/sppf"
)

// We use a small unambiguous expression grammar for testing.
// It is slightly adapted from
//
//      http://loup-vaillant.fr/tutorials/earley-parsing/recogniser
//
// This way we will be able to follow the examples there.
//
//     Sum     = Sum     '+' Product
//             | Product
//     Product = Product '*' Factor
//             | Factor
//     Factor  = '(' Sum ')'
//             | number
//
// 'number' is a terminal symbol recognizing Go integers.
//
func makeGrammar(t *testing.T) *lr.LRAnalysis {
	b := lr.NewGrammarBuilder("Expressions")
	b.LHS("Sum").N("Sum").T("+", '+').N("Product").End()
	b.LHS("Sum").N("Product").End()
	b.LHS("Product").N("Product").T("*", '*').N("Factor").End()
	b.LHS("Product").N("Factor").End()
	b.LHS("Factor").T("(", '(').N("Sum").T(")", ')').End()
	b.LHS("Factor").T("number", scanner.Int).End()
	g, err := b.Grammar()
	if err != nil {
		t.Error(err)
	}
	ga := lr.Analysis(g)
	if ga == nil {
		t.Errorf("Could not analyze grammar")
	}
	return ga
}

func makeParser(t *testing.T, test string, input string) (*Parser, scanner.Tokenizer) {
	level := tracing.Select("gorgo.lr").GetTraceLevel()
	tracing.Select("gorgo.lr").SetTraceLevel(tracing.LevelInfo)
	reader := strings.NewReader(input)
	scanner := scanner.GoTokenizer(fmt.Sprintf("test '%s'", test), reader)
	ga := makeGrammar(t)
	tracing.Select("gorgo.lr").SetTraceLevel(level)
	return NewParser(ga), scanner
}

var inputStrings = []string{
	"1", "1+2", "1*2", "1+2*3", "1*(2+3)", "1+2+3+4", "1*2+3*4",
}

// --- the Tests -------------------------------------------------------------

func TestParser1(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "gorgo.lr")
	defer teardown()
	//
	for n, input := range inputStrings {
		tracer().Infof("=== '%s' ========================", input)
		parser, scanner := makeParser(t, "Parser1", input)
		tracer().SetTraceLevel(tracing.LevelDebug)
		accept, err := parser.Parse(scanner, nil)
		if err != nil {
			t.Error(err)
		}
		if !accept {
			t.Errorf("Valid input string #%d not accepted: '%s'", n+1, input)
		}
	}
}

func TestTree1(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "gorgo.lr")
	defer teardown()
	//
	input := "1+2*3"
	parser, scanner := makeParser(t, "Tree1", input)
	tracing.Select("gorgo.lr").SetTraceLevel(tracing.LevelInfo)
	accept, err := parser.Parse(scanner, nil)
	if err != nil {
		t.Error(err)
	}
	if !accept {
		t.Errorf("Valid input string not accepted: '%s'", input)
	}
	tracer().SetTraceLevel(tracing.LevelError)
	v := parser.WalkDerivation(NewExprListener(t))
	value, ok := v.Value.(int)
	if !ok || value != 7 {
		t.Errorf("Expected %s to be 7, is %d", input, value)
	}
}

func TestSPPF1(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "gorgo.lr")
	defer teardown()
	//
	input := "1+2*3"
	parser, scanner := makeParser(t, "SPPF1", input)
	tracing.Select("gorgo.lr").SetTraceLevel(tracing.LevelInfo)
	accept, err := parser.Parse(scanner, nil)
	if err != nil {
		t.Error(err)
	}
	if !accept {
		t.Errorf("Valid input string not accepted: '%s'", input)
	}
	tracer().SetTraceLevel(tracing.LevelDebug)
	walker := NewTreeBuilder(parser.ga.Grammar())
	root := parser.WalkDerivation(walker)
	_, ok := root.Value.(*sppf.SymbolNode)
	if !ok || root.Symbol().Name != "S'" { // should have reduced top level rule
		if root == nil {
			t.Errorf("returned parse forest is empty")
		} else {
			t.Errorf("Expected root node of forest to be S', is %v", root.Symbol())
		}
	}
}

func TestAmbiguity1(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "gorgo.lr")
	defer teardown()
	//
	b := lr.NewGrammarBuilder("Test-G")
	b.LHS("X").T("+", '+').N("X").End()
	b.LHS("X").N("X").T("*", '*').N("X").End()
	b.LHS("X").T("x", -2).End()
	g, err := b.Grammar()
	if err != nil {
		t.Error(err)
	}
	ga := lr.Analysis(g)
	if ga == nil {
		t.Errorf("Could not analyze grammar")
	}
	tracing.Select("gorgo.lr").SetTraceLevel(tracing.LevelInfo)
	input := "+x*x"
	reader := strings.NewReader(input)
	sc := scanner.GoTokenizer(fmt.Sprintf("test '%s'", input), reader)
	parser := NewParser(ga)
	tracer().SetTraceLevel(tracing.LevelDebug)
	accept, err := parser.Parse(sc, nil)
	if err != nil {
		t.Error(err)
	}
	if !accept {
		t.Errorf("Valid input string not accepted: '%s'", input)
	}
	tracer().SetTraceLevel(tracing.LevelDebug)
	walker := NewTreeBuilder(parser.ga.Grammar())
	root := parser.WalkDerivation(walker)
	if root == nil || root.Value == nil {
		t.Fatalf("returned parse forest is empty")
	}
	_, ok := root.Value.(*sppf.SymbolNode)
	if !ok || root.Symbol().Name != "S'" { // should have reduced top level rule
		if root == nil {
			t.Errorf("returned parse forest is empty")
		} else {
			t.Errorf("Expected root node of forest to be S', is %v", root.Symbol())
		}
	}
}

// --- Expression Listener for testing ---------------------------------------

type reducer func(*lr.Symbol, int, []*RuleNode, int) interface{}
type ExprListener struct {
	total    int
	t        *testing.T
	dispatch map[string]reducer
}

func NewExprListener(t *testing.T) *ExprListener {
	el := &ExprListener{t: t}
	el.dispatch = map[string]reducer{
		"Sum":     el.ReduceSum,
		"Product": el.ReduceProduct,
		"Factor":  el.ReduceFactor,
	}
	return el
}

func (el *ExprListener) Reduce(lhs *lr.Symbol, rule int, children []*RuleNode, extent gorgo.Span,
	level int) interface{} {
	//
	if r, ok := el.dispatch[lhs.Name]; ok {
		return r(lhs, rule, children, level)
	}
	el.t.Logf("%sReduce of grammar symbol: %v", indent(level), lhs)
	return children[0].Value
}

func (el *ExprListener) ReduceSum(lhs *lr.Symbol, rule int, children []*RuleNode, level int) interface{} {
	v := children[0].Value // Product
	if len(children) > 1 {
		v = children[0].Value.(int) + children[2].Value.(int) // Sum + Product
	}
	el.t.Logf("%sSUM %v\n", indent(level), v)
	return v
}

func (el *ExprListener) ReduceProduct(lhs *lr.Symbol, rule int, children []*RuleNode, level int) interface{} {
	v := children[0].Value // Factor
	if len(children) > 1 {
		v = children[0].Value.(int) * children[2].Value.(int) // Product * Factor
	}
	el.t.Logf("%sPRODUCT %v\n", indent(level), v)
	return v
}

func (el *ExprListener) ReduceFactor(lhs *lr.Symbol, rule int, children []*RuleNode, level int) interface{} {
	v := children[0].Value // number
	if len(children) > 1 {
		v = children[1].Value // ( Sum )
	}
	el.t.Logf("%sFACTOR %v\n", indent(level), v)
	return v
}

func (el *ExprListener) Terminal(token gorgo.Token, level int) interface{} {
	//func (el *ExprListener) Terminal(tokval int, token interface{}, extent gorgo.Span, level int) interface{} {
	el.t.Logf("%sToken %q|%d\n", indent(level), token.Lexeme(), token.TokType())
	if token.TokType() == scanner.Int {
		n, _ := strconv.Atoi(token.Lexeme())
		el.t.Logf("%svalue n = %d", indent(level), n)
		return n
	}
	return token.TokType()
}

func indent(level int) string {
	in := ""
	for level > 0 {
		in = in + ". "
		level--
	}
	return in
}
