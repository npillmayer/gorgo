package glr

import (
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"testing"
	"text/scanner"

	"github.com/npillmayer/gorgo/lr"
	"github.com/npillmayer/schuko/gtrace"
	"github.com/npillmayer/schuko/tracing"
	"github.com/npillmayer/schuko/tracing/gotestingadapter"
)

/*
https://cs.au.dk/~amoeller/papers/ambiguity/ambiguity.pdf  -> Example 4
http://citeseerx.ist.psu.edu/viewdoc/download;jsessionid=A6FB43374BBE6D3041EF573C2A65C2C6?doi=10.1.1.36.4448&rep=rep1&type=pdf

  1: S  ::= [A -]
  2: S  ::= [+ B]
  3: A  ::= [+ a]
  4: B  ::= [a -]
*/
func TestGLR1(t *testing.T) {
	//gtrace.SyntaxTracer = gologadapter.New()
	teardown := gotestingadapter.QuickConfig(t, "tyse.fonts")
	defer teardown()
	//
	b := lr.NewGrammarBuilder("G1")
	b.LHS("S").N("A").T("-", '-').End()
	b.LHS("S").T("+", '+').N("B").End()
	b.LHS("A").T("+", '+').T("a", scanner.Ident).End()
	b.LHS("B").T("a", scanner.Ident).T("-", '-').End()
	g, err := b.Grammar()
	if err != nil {
		t.Error(err)
	}
	parse(t, g, false, "+a-")
}

// ----------------------------------------------------------------------

func parse(t *testing.T, g *lr.Grammar, doDump bool, input ...string) bool {
	gtrace.SyntaxTracer.SetTraceLevel(tracing.LevelInfo)
	ga := lr.Analysis(g)
	lrgen := lr.NewTableGenerator(ga)
	lrgen.CreateTables()
	if lrgen.HasConflicts {
		t.Logf("Grammar %s has conflicts", g.Name)
	}
	gtrace.SyntaxTracer.SetTraceLevel(tracing.LevelDebug)
	if doDump {
		dump(t, g, lrgen)
	}
	var ok bool
	for _, inp := range input {
		//p := NewParser(g, lrgen.GotoTable(), lrgen.ActionTable(), lrgen.AcceptingStates())
		p := NewParser(g, lrgen.GotoTable(), lrgen.ActionTable())
		r := strings.NewReader(inp)
		scanner := NewStdScanner(r)
		ok, err := p.Parse(lrgen.CFSM().S0, scanner)
		if err != nil {
			t.Errorf("parser returned error: %v", err)
		}
		if !ok {
			t.Errorf("parser did not accept input='%s'", inp)
		}
	}
	return ok
}

func dump(t *testing.T, g *lr.Grammar, lrgen *lr.TableGenerator) {
	g.Dump()
	tmpfile, err := ioutil.TempFile(".", fmt.Sprintf("%s_goto_*.html", g.Name))
	if err != nil {
		log.Fatal(err)
	}
	lr.GotoTableAsHTML(lrgen, tmpfile)
	tmpfile, err = ioutil.TempFile(".", fmt.Sprintf("%s_action_*.html", g.Name))
	if err != nil {
		log.Fatal(err)
	}
	lr.ActionTableAsHTML(lrgen, tmpfile)
	lrgen.CFSM().CFSM2GraphViz(fmt.Sprintf("./%s_cfsm.dot", g.Name))
}
