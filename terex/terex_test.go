package terex

import (
	"testing"

	"github.com/npillmayer/gorgo"
	"github.com/npillmayer/schuko/tracing/gotestingadapter"
)

func TestAssignability(t *testing.T) {
	var op interface{} = &internalOp{}
	switch x := op.(type) {
	case Operator:
		t.Logf("internalOp %v assignable to Operator", x)
	default:
		t.Errorf("Expected internalOp to be assignable to Operator")
	}
}

func TestAtomizeToken(t *testing.T) {
	token := token{}
	a := Atomize(token)
	if a.Type() != TokenType {
		t.Errorf("expected atom to be of type 'token', is %d", a.Type())
	}
}

func TestAtomizeOp(t *testing.T) {
	op := GlobalEnvironment.Defn("Hello", nil)
	a := Atomize(op)
	t.Logf("atom = %v", a)
	if a.Type() != VarType {
		t.Errorf("expected symbol to be of var type")
	}
	if op.Value.Type() != OperatorType {
		t.Errorf("expected symbol-value to be of operator type")
	}
}

func TestList1(t *testing.T) {
	l1 := List(1, 4, 6, 8, 12)
	if l1.Length() != 5 {
		t.Errorf("length of list expected to be 5, is %d", l1.Length())
	}
	if l1.Car.Data != 1.0 {
		t.Errorf("element #1 expected to be 1, is %v", l1.Car.Data)
	}
}
func TestList2(t *testing.T) {
	InitGlobalEnvironment()
	l := List(GlobalEnvironment.FindSymbol("list", false), 1, 2)
	t.Logf("l=%s", l.ListString())
	if l.Car.Type() != OperatorType {
		t.Errorf("expected 'list' to be retrieved as operator")
	}
}

func TestFirst(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "gorgo.terex")
	defer teardown()
	//
	l := List(1, 2, 3, 4, 5)
	f := l.FirstN(3)
	if f.Length() != 3 || f.Car.Data != 1.0 {
		t.Errorf("Expected f to be (1 2 3), is %s", f.ListString())
	}
	l = List(1)
	f = l.FirstN(1)
	if f.Length() != 1 || f.Car.Data != 1.0 {
		t.Errorf("Expected f to be (1), is %s", f.ListString())
	}
	l = List(1)
	f = l.FirstN(5)
	if f.Length() != 1 || f.Car.Data != 1.0 {
		t.Errorf("Expected f to be (1), is %s", f.ListString())
	}
}

func TestNth(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "gorgo.terex")
	defer teardown()
	//
	l := List(1, 2, 3, 4, 5)
	f := l.Nth(3)
	if f.Data != 3.0 {
		t.Errorf("Expected 3rd element to be 3, is %v", f.Data)
	}
}

func TestDrop(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "gorgo.terex")
	defer teardown()
	//
	l := List(1, 2, 3, 4, 5)
	f := l.Drop(func(a Atom) bool {
		return a.Data == 3.0
	})
	t.Logf("f = %v", f.ListString())
	if f.Length() < 3 || f.Nth(3).Data != 4.0 {
		t.Errorf("Expected 3rd element to be 4, is %v", f.Nth(3).Data)
	}
}

func TestMatch1(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "gorgo.terex")
	defer teardown()
	//
	l1 := List("a", 1, 2)
	l2 := List("a", 1, 2)
	if !l1.Match(l2, GlobalEnvironment) {
		t.Errorf("l1 and l2 expected to match, don't")
	}
	l3 := List("a", 1, 2)
	l4 := List("a", "b", 2)
	if l3.Match(l4, GlobalEnvironment) {
		t.Errorf("l3 and l4 not expected to match, do")
	}
	l51 := List("b")
	l5 := List("a", l51, 2)
	l61 := List("b")
	l6 := List("a", l61, 2)
	t.Logf("l5 = %s, l6 = %s", l5.ListString(), l6.ListString())
	if !l5.Match(l6, GlobalEnvironment) {
		t.Errorf("l5 and l6 expected to match, don't")
	}
	t.Logf(GlobalEnvironment.Dump())
}

func TestMatch2(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "gorgo.terex")
	defer teardown()
	//
	S := GlobalEnvironment.Intern("S", false)
	l1 := QuotedList("a", S, 2)
	l2 := List("a", 7, 2)
	t.Logf("l1 = %s, l2 = %s", l1.ListString(), l2.ListString())
	if !l1.Match(l2, GlobalEnvironment) {
		t.Errorf("l1 and l2 expected to match, don't")
	}
	t.Logf(GlobalEnvironment.Dump())
	if S.ValueType() != NumType { // S expected to be bound to 7
		t.Errorf("Symbol a expected to be of number type now, is %s", S.ValueType().String())
	}
}

func TestString(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "gorgo.terex")
	defer teardown()
	//
	l2 := List("+", 5, 7, 12)
	l := Cons(Atomize(1), nil)
	l.Cdr = Cons(Atomize(2), nil)
	x := Cons(Atomize(l2), nil)
	l.Cdr.Cdr = x
	x.Cdr = Cons(Atomize("Hello"), nil)
	s := l.ListString()
	t.Logf("l = %s", s)
	if s != `(1 2 ("+" 5 7 12) "Hello")` {
		t.Errorf(`Expected list to be (1 2 ("+" 5 7 12) "Hello"), is "%s"`, s)
	}
	t.Logf("\n\n" + l.IndentedListString())
}

func TestListAPI(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "gorgo.terex")
	defer teardown()
	//
	l := List(1, 2, 3)
	t.Logf("last = %s", l.Last())
	l.Append(List(5))
	t.Logf("l = %s", l.ListString())
}

func TestNilElement(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "gorgo.terex")
	defer teardown()
	//
	n := Elem(nil)
	if !n.IsNil() {
		t.Errorf("nil-element expected to be recognized with isNil()")
	}
}

// ---------------------------------------------------------------------------

type token struct{}

func (t token) TokType() gorgo.TokType {
	return 1
}

func (t token) Value() interface{} {
	return nil
}

func (t token) Lexeme() string {
	return "-"
}

func (t token) Span() gorgo.Span {
	return gorgo.Span{0, 1}
}
