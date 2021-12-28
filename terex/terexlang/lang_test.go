package terexlang

import (
	"testing"

	"github.com/npillmayer/gorgo/terex"
	"github.com/npillmayer/gorgo/terex/termr"
	"github.com/npillmayer/schuko/tracing"
	"github.com/npillmayer/schuko/tracing/gotestingadapter"
)

func TestScanner(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "gorgo.terex")
	defer teardown()
	//
	lex, _ := Lexer()
	input := "a + '(1.2 world !) #var nil ;"
	scan, err := lex.Scanner(input)
	if err != nil {
		t.Errorf(err.Error())
	}
	scan.SetErrorHandler(func(e error) {
		t.Error(e)
	})
	done := false
	for !done {
		token := scan.NextToken()
		if token.TokType() == -1 {
			done = true
		} else {
			t.Logf("token = %q with value = %d", token.Lexeme(), token.TokType())
		}
	}
}

func TestAssignability(t *testing.T) {
	var e interface{} = &sExprRewriter{name: "Hello"}
	switch x := e.(type) {
	case termr.TermRewriter:
		t.Logf("sExprTermR %v accepted as termr.TermR", x)
		switch o := x.OperatorFor("Hello").(type) {
		case terex.Operator:
			t.Logf("sExprTermR.Operator() %v accepted as terex.Operator", o)
		default:
			t.Errorf("Expected %v to implement terex.Operator interface", o)
		}
	default:
		t.Errorf("Expected terexlang.sExprTermR to implement termr.TermR interface")
	}
}

func TestMatchAnything(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "gorgo.terex")
	defer teardown()
	//
	initRewriters()
	l := terex.List(1, 2, 3)
	if !termr.Anything().Match(l, terex.GlobalEnvironment) {
		t.Errorf("Expected !Anything to match (1 2 3)")
	}
}

func TestParse(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "gorgo.terex")
	defer teardown()
	//
	terex.InitGlobalEnvironment()
	input := `((1 2))`
	//input := "(Hello 'World 1)"
	parser := createParser()
	scan, _ := lexer.Scanner(input)
	tracer().SetTraceLevel(tracing.LevelDebug)
	accept, err := parser.Parse(scan, nil)
	t.Logf("accept=%v, input=%s", accept, input)
	if err != nil {
		t.Error(err)
	}
	if !accept {
		t.Errorf("No accept. Not a valid TeREx expression")
	}
}

func TestAST(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "gorgo.terex")
	defer teardown()
	//
	terex.InitGlobalEnvironment()
	input := `a`
	//input := `(+ '(1 "Hi" 3) 4)`
	parsetree, retr, err := Parse(input)
	if err != nil {
		t.Error(err)
	}
	if parsetree == nil || retr == nil {
		t.Errorf("parse tree or  token retriever is nil")
	}
	tracer().SetTraceLevel(tracing.LevelDebug)
	tracer().Infof("####################################################")
	ab := newASTBuilder()
	env := ab.AST(parsetree, retr)
	if env == nil {
		t.Errorf("Cannot create AST from parsetree")
	}
	ast := env.AST
	tracer().SetTraceLevel(tracing.LevelInfo)
	tracer().Infof("AST: %s", ast.ListString())
	tracer().Infof("####################################################")
}

func TestQuoteAST(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "gorgo.terex")
	defer teardown()
	//
	terex.InitGlobalEnvironment()
	//input := `(Hello 'World (+ 1 2) "string")`
	input := `'((1))`
	//input := `(((1)))`
	tree, retr, err := Parse(input)
	if err != nil {
		t.Errorf(err.Error())
	}
	ast, env, err := AST(tree, retr)
	//t.Logf("\n\n" + debugString(terex.Elem(ast.Car)))
	//t.Logf("\n\n" + debugString(terex.Elem(ast)))
	tracer().SetTraceLevel(tracing.LevelInfo)
	terex.Elem(ast).First().Dump(tracing.LevelInfo)
	env.Def("a", terex.Elem(7))
	q, err := QuoteAST(terex.Elem(ast).First(), env)
	if err != nil {
		t.Errorf(err.Error())
	}
	//t.Logf("\n\n" + debugString(q))
	q.Dump(tracing.LevelInfo)
}

func debugString(e terex.Element) string {
	if e.IsNil() {
		return "nil"
	}
	if e.IsAtom() {
		if e.AsAtom().Type() == terex.ConsType {
			return e.AsList().IndentedListString()
		}
		return e.String()
	}
	return e.AsList().IndentedListString()
}
