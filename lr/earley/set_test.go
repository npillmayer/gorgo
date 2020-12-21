package earley

import (
	"testing"

	"github.com/npillmayer/gorgo/lr"
	"github.com/npillmayer/schuko/gtrace"
	"github.com/npillmayer/schuko/tracing"
	"github.com/npillmayer/schuko/tracing/gotestingadapter"
)

func TestSet1(t *testing.T) {
	gtrace.SyntaxTracer = gotestingadapter.New()
	gtrace.SyntaxTracer.SetTraceLevel(tracing.LevelDebug)
	teardown := gotestingadapter.RedirectTracing(t)
	defer teardown()
	//
	set := ruleset{}
	if set.contains(nil, 0) {
		t.Errorf("set contains nil, no set should")
	}
}

func TestSet2(t *testing.T) {
	gtrace.SyntaxTracer = gotestingadapter.New()
	gtrace.SyntaxTracer.SetTraceLevel(tracing.LevelDebug)
	teardown := gotestingadapter.RedirectTracing(t)
	defer teardown()
	//
	b := lr.NewGrammarBuilder("G") // build a grammar of 3 rules
	b.LHS("S").N("A").End()        // [1]: S → A
	b.LHS("A").N("B").End()        // [2]: A → B
	b.LHS("A").N("A").N("B").End() // [3]: A → A B
	g, _ := b.Grammar()            // [0]: S' → S
	//
	var set ruleset
	set = set.add(g.Rule(1), 5)
	if !set.contains(g.Rule(1), 5) {
		t.Errorf("Expected rule[1] to be contained in set, isn't")
		set.dump()
	}
	set = set.add(g.Rule(2), 3)
	set.delete(g.Rule(2))
	if set.contains(g.Rule(2), 3) {
		t.Errorf("Expected rule[2] to not be contained in set, yet is")
		set.dump()
	}
	set = set.add(g.Rule(3), 5)
	if _, ok := set.containsSibling(g.Rule(2), 5); !ok {
		t.Errorf("Expected sibling of rule[2] to be contained in set, yet isn't")
		set.dump()
	}
}
