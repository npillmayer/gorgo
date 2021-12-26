package sppf

import (
	"fmt"
	"testing"
	"text/scanner"

	"github.com/npillmayer/schuko/gtrace"
	"github.com/npillmayer/schuko/tracing"
	"github.com/npillmayer/schuko/tracing/gotestingadapter"

	"github.com/npillmayer/gorgo/lr"
)

func TestSignature(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "gorgo.lr")
	defer teardown()
	//
	b := lr.NewGrammarBuilder("G")
	b.LHS("S").N("A").End()
	b.LHS("A").N("B").End()
	b.LHS("B").T("x", 10).End()
	g, _ := b.Grammar()
	s1 := makeSym(g.SymbolByName("A")).spanning(1, 2)
	rhs1 := []*SymbolNode{s1}
	t.Logf("rhs=%v", rhs1)
	s2 := makeSym(g.SymbolByName("A")).spanning(11, 12)
	rhs2 := []*SymbolNode{s2}
	t.Logf("rhs=%v", rhs2)
	s3 := makeSym(g.SymbolByName("A")).spanning(15, 16)
	rhs3 := []*SymbolNode{s3}
	t.Logf("rhs=%v", rhs3)
	sigma1 := rhsSignature(rhs1, 1)
	sigma2 := rhsSignature(rhs2, 11)
	sigma3 := rhsSignature(rhs3, 15)
	t.Logf("Σ1 = %d", sigma1)
	t.Logf("Σ2 = %d", sigma2)
	t.Logf("Σ3 = %d", sigma3)
	if sigma1 == sigma2 || sigma1 == sigma3 || sigma2 == sigma3 {
		t.Errorf("Expected Σ[1…3] to be different from each other, aren't")
	}
}

func TestSigma(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "gorgo.lr")
	defer teardown()
	//
	b := lr.NewGrammarBuilder("G")
	b.LHS("S").T("<", '<').N("A").N("Z").T(">", '>').End()
	g, _ := b.Grammar()
	s1 := makeSym(g.SymbolByName("A")).spanning(1, 8)
	s2 := makeSym(g.SymbolByName("Z")).spanning(8, 9)
	rhs := []*SymbolNode{s1, s2}
	t.Logf("rhs=%v", rhs)
	sigma := rhsSignature(rhs, 0)
	if sigma != 113896 {
		t.Errorf("sigma expected to be 113896, is %d", sigma)
	}
}

// S' ⟶ S
// S  ⟶ A
// A  ⟶ a
func TestSPPFInsert(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "gorgo.lr")
	defer teardown()
	//
	b := lr.NewGrammarBuilder("G")
	b.LHS("S").N("A").End()
	r2 := b.LHS("A").T("a", scanner.Ident).End()
	g, err := b.Grammar()
	if err != nil {
		t.Error(err)
	}
	gtrace.SyntaxTracer.SetTraceLevel(tracing.LevelDebug)
	g.Dump()
	f := NewForest()
	a := f.AddTerminal(r2.RHS()[0], 0)
	A := r2.LHS
	R := f.AddReduction(A, 2, []*SymbolNode{a})
	t.Logf("node A=%v for rule %v", R, g.Rule(2))
	if R == nil {
		t.Errorf("Expected symbol node A=%v for rule %v", R, g.Rule(2))
	}
}

// S' ⟶ S
// S  ⟶ A | B
// A  ⟶ a
// B  ⟶ a
func TestSPPFAmbiguous(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "gorgo.lr")
	defer teardown()
	//
	b := lr.NewGrammarBuilder("G")
	b.LHS("S").N("A").End()
	b.LHS("S").N("B").End()
	b.LHS("A").T("a", scanner.Ident).End()
	b.LHS("B").T("a", scanner.Ident).End()
	g, err := b.Grammar()
	if err != nil {
		t.Error(err)
	}
	gtrace.SyntaxTracer.SetTraceLevel(tracing.LevelDebug)
	g.Dump()
	//f := NewForest()
}

// S' ⟶ S
// S  ⟶ A
// A  ⟶ a
func TestTraverse(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "gorgo.lr")
	defer teardown()
	//
	b := lr.NewGrammarBuilder("G")
	r1 := b.LHS("S").N("A").End()
	r2 := b.LHS("A").T("a", scanner.Ident).End()
	G, err := b.Grammar()
	if err != nil {
		t.Error(err)
	}
	gtrace.SyntaxTracer.SetTraceLevel(tracing.LevelDebug)
	G.Dump()
	f := NewForest()
	a := f.AddTerminal(r2.RHS()[0], 0)
	A := f.AddReduction(r2.LHS, 2, []*SymbolNode{a})
	S := f.AddReduction(r1.LHS, 1, []*SymbolNode{A})
	f.AddReduction(G.SymbolByName("S'"), 0, []*SymbolNode{S})
	if f.Root() == nil {
		t.Errorf("Expected root node S', is nil")
	}
	l := makeListener(G, t)
	c := f.SetCursor(nil, nil)
	c.TopDown(l, LtoR, Continue)
	if !l.(*L).isBack {
		t.Errorf("Exit(S') has not been called")
	}
	if l.(*L).a.Name != "a" {
		t.Errorf("Terminal(a) has not been called")
	}
}

// ---------------------------------------------------------------------------

func makeListener(G *lr.Grammar, t *testing.T) Listener {
	return &L{G: G, t: t}
}

type L struct {
	G      *lr.Grammar
	t      *testing.T
	isBack bool
	a      *lr.Symbol
}

func (l *L) EnterRule(sym *lr.Symbol, rhs []*RuleNode, ctxt RuleCtxt) bool {
	if sym.IsTerminal() {
		return false
	}
	l.t.Logf("+ enter %v", sym)
	return true
}
func (l *L) ExitRule(sym *lr.Symbol, rhs []*RuleNode, ctxt RuleCtxt) interface{} {
	if sym.Name == "S'" {
		l.isBack = true
	}
	l.t.Logf("- exit %v", sym)
	return nil
}

func (l *L) Terminal(tokval int, token interface{}, ctxt RuleCtxt) interface{} {
	tok := l.G.Terminal(tokval)
	l.a = tok
	l.t.Logf("  terminal=%s", tok.Name)
	return tok
}

func (l *L) Conflict(sym *lr.Symbol, ctxt RuleCtxt) (int, error) {
	l.t.Error("did not expect conflict")
	return 0, fmt.Errorf("Conflict at symbol %v", sym)
}

func (l *L) MakeAttrs(*lr.Symbol) interface{} {
	return nil
}
