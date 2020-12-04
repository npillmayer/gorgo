package dss

import (
	"io/ioutil"
	"testing"

	"github.com/npillmayer/gorgo/lr"
	"github.com/npillmayer/schuko/tracing"
)

func traceOn() {
	T().SetTraceLevel(tracing.LevelDebug)
}

func TestRoot(t *testing.T) {
	r := NewRoot("G", -999)
	if r.Name != "G" {
		t.Fail()
	}
}

func TestCreateStack(t *testing.T) {
	r := NewRoot("G", -999)
	s := NewStack(r)
	if s.Pop() != nil {
		t.Fail()
	}
}

func TestPush(t *testing.T) {
	r := NewRoot("G", -999)
	s := NewStack(r)
	A := pseudosym("A")
	s.Push(1, A)
	if _, top := s.Peek(); top != A {
		t.Fail()
	}
}

func TestPush2(t *testing.T) {
	A := pseudosym("A")
	r := NewRoot("G", -999)
	s1 := NewStack(r)
	s2 := NewStack(r)
	s1.Push(1, A)
	if s1.tos.pathcnt != 1 {
		T().Errorf("pathcount at tos expected to be 1, is %d", s1.tos.pathcnt)
		t.Fail()
	}
	s2.Push(1, A)
	if s1.tos != s2.tos {
		t.Fail()
	}
}

func TestPush3(t *testing.T) {
	A, B := pseudosym("A"), pseudosym("B")
	r := NewRoot("G", -999)
	s1 := NewStack(r)
	s1.Push(1, A)
	s2 := NewStack(r)
	s2.Push(2, B).Push(3, B)
	s1.Push(3, B)
	if s1.tos.pathcnt != 2 {
		T().Errorf("pathcount at join expected to be 2, is %d", s1.tos.pathcnt)
		t.Fail()
	}
}

func TestPush4(t *testing.T) {
	A, B, C := pseudosym("A"), pseudosym("B"), pseudosym("C")
	r := NewRoot("G", -999)
	s1 := NewStack(r)
	s2 := NewStack(r)
	s3 := NewStack(r)
	s1.Push(1, A).Push(2, B).Push(3, C)
	s2.Push(1, A).Push(3, C)
	s3.Push(3, C)
	if s1.tos.pathcnt != 3 {
		T().Errorf("pathcount at join expected to be 3, is %d", s1.tos.pathcnt)
		t.Fail()
	}
}

func TestPath1(t *testing.T) {
	A, B, C := pseudosym("A"), pseudosym("B"), pseudosym("C")
	r := NewRoot("G", -999)
	s1 := NewStack(r)
	s1.Push(1, C).Push(3, A)
	s2 := NewStack(r)
	s2.Push(2, B).Push(3, A).Push(4, C)
	if s1.tos.pathcnt != 2 {
		T().Errorf("pathcount at join expected to be 2, is %d", s1.tos.pathcnt)
		t.Fail()
	}
	handle := []*lr.Symbol{A, C}
	path := s2.FindHandlePath(handle, 0)
	T().Debugf("path = %v", path)
	if path == nil || len(path) != 2 || path[1] == nil {
		T().Errorf("path not found or incorrect: ", path)
		t.Fail()
	}
}

func TestPath2(t *testing.T) {
	A, B, C := pseudosym("A"), pseudosym("B"), pseudosym("C")
	r := NewRoot("G", -999)
	s1 := NewStack(r)
	s1.Push(1, A).Push(2, B)
	path := s1.FindHandlePath([]*lr.Symbol{B}, 0)
	s2 := s1.splitOff(path)
	s2.tos.State = 3
	s1.Push(4, C)
	s2.Push(4, C)
	if s1.tos.pathcnt != 2 {
		T().Errorf("pathcount at join expected to be 2, is %d", s1.tos.pathcnt)
		t.Fail()
	}
	handle := []*lr.Symbol{B, C}
	path = s2.FindHandlePath(handle, 0)
	T().Debugf("path = %v", path)
	if path == nil || len(path) != 2 || path[1] == nil {
		T().Errorf("path not found or incorrect: %v", path)
		t.Fail()
	}
	/*
		tmp, _ := ioutil.TempFile("", "stack_")
		T().Infof("writing DOT to %s", tmp.Name())
		DSS2Dot(r, path, tmp)
	*/
	path2 := s2.FindHandlePath(handle, 1)
	/*
		T().Debugf("path = %v", path2)
		tmp, _ = ioutil.TempFile("", "stack_")
		T().Infof("writing DOT to %s", tmp.Name())
		DSS2Dot(r, path2, tmp)
	*/
	if path[0] == path2[0] {
		T().Errorf("did not find 2nd handle")
		t.Fail()
	}
}

func TestPush5(t *testing.T) {
	A, B, C := pseudosym("A"), pseudosym("B"), pseudosym("C")
	r := NewRoot("G", -999)
	s1 := NewStack(r)
	s2 := NewStack(r)
	s1.Push(1, A).Push(2, C).Push(5, A)
	s2.Push(1, A).Push(3, B).Push(5, A)
	if s1.tos.pathcnt != 2 {
		T().Errorf("pathcount at join expected to be 2, is %d", s1.tos.pathcnt)
		t.Fail()
	}
	handle := []*lr.Symbol{A, B, A}
	path := s2.FindHandlePath(handle, 0)
	if len(path[0].succs) != 2 {
		T().Errorf("inverse fork at %v incorrect", path[0])
		t.Fail()
	}
	if !path[0].isInverseFork() {
		T().Errorf("%v not recognized as inverse fork", path[0])
		t.Fail()
	}
}

func TestSplitOff1(t *testing.T) {
	A, B, C := pseudosym("A"), pseudosym("B"), pseudosym("C")
	r := NewRoot("G", -999)
	s2 := NewStack(r)
	s2.Push(2, B).Push(3, A).Push(4, C)
	handle := []*lr.Symbol{A, C}
	path := s2.FindHandlePath(handle, 0)
	T().Debugf("path = %v", path)
	s3 := s2.splitOff(path)
	if s2.tos == s3.tos {
		T().Errorf("stacks not split")
		t.Fail()
	}
	if len(r.stacks) != 2 {
		t.Fail()
	}
}

func TestSplitOff2(t *testing.T) {
	A, B, C := pseudosym("A"), pseudosym("B"), pseudosym("C")
	r := NewRoot("G", -999)
	s1 := NewStack(r)
	s2 := NewStack(r)
	s1.Push(2, A).Push(3, B).Push(4, C)
	s2.Push(2, A).Push(5, A).Push(4, C)
	handle := []*lr.Symbol{A, A, C}
	path := s2.FindHandlePath(handle, 0)
	T().Debugf("path = %v", path)
	s3 := s2.splitOff(path)
	if s2.tos == s3.tos {
		T().Errorf("stacks not split")
		t.Fail()
	}
	if len(r.stacks) != 3 {
		t.Fail()
	}
}

func TestPush6(t *testing.T) {
	A, B, C, D := pseudosym("A"), pseudosym("B"), pseudosym("C"), pseudosym("D")
	r := NewRoot("G", -999)
	s1 := NewStack(r)
	s2 := NewStack(r)
	s1.Push(2, A).Push(3, B).Push(6, B)
	s2.Push(2, A).Push(5, A).Push(4, C)
	handle := []*lr.Symbol{A, A, C}
	path := s2.FindHandlePath(handle, 0)
	s3 := s2.splitOff(path)
	s1.Push(9, D)
	s2.Push(9, D)
	s3.Push(9, D)
	if !s1.tos.isInverseJoin() || s1.tos.predecessorCount() != 3 {
		T().Errorf("3-join not correctly recognized")
		t.Fail()
	}
}

func TestPop1(t *testing.T) {
	A, B, C := pseudosym("A"), pseudosym("B"), pseudosym("C")
	r := NewRoot("G", -999)
	s1 := NewStack(r)
	s2 := NewStack(r)
	s1.Push(2, A).Push(3, B).Push(4, C)
	s2.Push(2, A).Push(5, A).Push(4, C)
	handle := []*lr.Symbol{A, A, C}
	path := s2.FindHandlePath(handle, 0)
	s3 := s2.splitOff(path)
	s3.Pop()
	s1.Pop()
}

func TestReduce1(t *testing.T) {
	A, B, C := pseudosym("A"), pseudosym("B"), pseudosym("C")
	r := NewRoot("G", -999)
	s1 := NewStack(r)
	s2 := NewStack(r)
	s1.Push(2, A).Push(3, B).Push(4, C)
	s2.Push(2, A).Push(5, A).Push(4, C)
	handle := []*lr.Symbol{A, A, C}
	path := s2.FindHandlePath(handle, 0)
	_ = s2.splitOff(path)
	s2.reduce(path, true)
}

func TestReduce2(t *testing.T) {
	traceOn()
	A, B, C, D := pseudosym("A"), pseudosym("B"), pseudosym("C"), pseudosym("D")
	r := NewRoot("G", -999)
	s1 := NewStack(r)
	s2 := NewStack(r)
	s1.Push(1, A).Push(3, C).Push(4, D)
	s2.Push(2, B).Push(5, C).Push(4, D)
	if s1.tos != s2.tos {
		T().Errorf("TOS of stacks not merged")
		t.Fail()
	}
	if s1.tos.preds[0] == s1.tos.preds[1] {
		T().Errorf("no inverse join for symbol C in different states")
		t.Fail()
	}
	handle := []*lr.Symbol{C, D}
	path := s2.FindHandlePath(handle, 0)
	tmp, _ := ioutil.TempFile("", "stack_")
	T().Infof("writing DOT to %s", tmp.Name())
	DSS2Dot(r, path, tmp)
	stacks := s2.Reduce(handle)
	if len(stacks) != 2 {
		T().Errorf("# of stack heads after reduce: %d", len(stacks))
		t.Fail()
	}
	//tmp, _ = ioutil.TempFile("", "stack_")
	//T().Infof("writing DOT to %s", tmp.Name())
	//DSS2Dot(r, nil, tmp)
}

/*
func TestReduce1(t *testing.T) {
	traceOn()
	E, plus, epsilon := pseudosym("E"), pseudosym("+"), pseudosym("~")
	r := NewRoot("G", -999)
	s1, s2 := NewStack(r), NewStack(r)
	s2.Push(1, epsilon).Push(3, E).Push(4, plus).Push(5, E).Push(4, plus)
	s1.Push(1, epsilon).Push(3, E).Push(4, plus)
	tmp, _ := ioutil.TempFile("", "stack_")
	T().Infof("writing DOT to %s", tmp.Name())
	DSS2Dot(r, tmp)
		handle := []*lr.Symbol{E, plus, E}
		path := s1.FindHandlePath(handle, 0)
		T().Debugf("path = %v", path)
}

*/
