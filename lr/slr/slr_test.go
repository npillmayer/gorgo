package slr

import (
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"testing"

	"github.com/npillmayer/gorgo/lr"
	"github.com/npillmayer/gorgo/lr/scanner"
	"github.com/npillmayer/schuko/gtrace"
	"github.com/npillmayer/schuko/tracing"
	"github.com/npillmayer/schuko/tracing/gotestingadapter"
)

func TestSLR1(t *testing.T) {
	//gtrace.SyntaxTracer = gologadapter.New()
	gtrace.SyntaxTracer = gotestingadapter.New()
	teardown := gotestingadapter.RedirectTracing(t)
	defer teardown()
	gtrace.SyntaxTracer.SetTraceLevel(tracing.LevelInfo)
	b := lr.NewGrammarBuilder("G1")
	b.LHS("S").T("a", scanner.Ident).End()
	g, err := b.Grammar()
	if err != nil {
		t.Error(err)
	}
	parse(t, g, false, "a")
}

func TestSLR2(t *testing.T) {
	//gtrace.SyntaxTracer = gologadapter.New()
	gtrace.SyntaxTracer = gotestingadapter.New()
	teardown := gotestingadapter.RedirectTracing(t)
	defer teardown()
	gtrace.SyntaxTracer.SetTraceLevel(tracing.LevelInfo)
	b := lr.NewGrammarBuilder("G2")
	b.LHS("S").T("a", scanner.Ident).End()
	b.LHS("S").Epsilon()
	g, err := b.Grammar()
	if err != nil {
		t.Error(err)
	}
	parse(t, g, false, "a", "")
}

func TestSLR3(t *testing.T) {
	//gtrace.SyntaxTracer = gologadapter.New()
	gtrace.SyntaxTracer = gotestingadapter.New()
	teardown := gotestingadapter.RedirectTracing(t)
	defer teardown()
	gtrace.SyntaxTracer.SetTraceLevel(tracing.LevelInfo)
	b := lr.NewGrammarBuilder("G3")
	b.LHS("S").N("A").T("a", scanner.Ident).End()
	b.LHS("A").T("+", '+').End()
	b.LHS("A").T("-", '-').End()
	b.LHS("A").Epsilon()
	g, err := b.Grammar()
	if err != nil {
		t.Error(err)
	}
	parse(t, g, false, "a", "+a", "-a")
}

// ----------------------------------------------------------------------

func parse(t *testing.T, g *lr.Grammar, doDump bool, input ...string) bool {
	gtrace.SyntaxTracer.SetTraceLevel(tracing.LevelDebug)
	ga := lr.Analysis(g)
	lrgen := lr.NewTableGenerator(ga)
	lrgen.CreateTables()
	if lrgen.HasConflicts {
		t.Errorf("Grammar %s has conflicts", g.Name)
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
		scanner := scanner.GoTokenizer("test", r)
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
