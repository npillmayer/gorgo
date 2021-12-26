package fp_test

import (
	"strings"
	"testing"

	"github.com/npillmayer/gorgo/terex"
	"github.com/npillmayer/gorgo/terex/fp"
	"github.com/npillmayer/schuko/tracing/gotestingadapter"
)

func TestN(t *testing.T) {
	a := make([]int64, 0, 10)
	for n, N := fp.N().First(); n < 10; n = N.Next() {
		t.Logf("n=%d", n)
		a = append(a, n)
	}
	t.Logf("a=%v", a)
	if a[0] != 0 || a[9] != 9 || len(a) != 10 {
		t.Errorf("Generating 10 n in N failed")
	}
}

func TestR(t *testing.T) {
	a := make([]float64, 0, 10)
	for x, R := fp.R(0.0, 1.0).First(); x < 10; x = R.Next() {
		t.Logf("x=%.1f", x)
		a = append(a, x)
	}
	t.Logf("a=%v", a)
	if a[0] != 0.0 || a[9] != 9.0 || len(a) != 10 {
		t.Errorf("Generating 10 x in R failed")
	}
}

func TestF(t *testing.T) {
	seq := fp.N().Where(fp.EvenN()).Map(fp.SquareN())
	t.Logf("seq=%v", seq)
	n, N := fp.N().Where(fp.EvenN()).Map(fp.SquareN()).First()
	t.Logf("n=%d, N.n=%d", n, N.N())
	n = N.Next()
	t.Logf("n=%d, N.n=%d", n, N.N())
	n = N.Next()
	t.Logf("n=%d, N.n=%d", n, N.N())
	n = N.Next()
	t.Logf("n=%d, N.n=%d", n, N.N())
}

func TestIntFilter(t *testing.T) {
	a := make([]int64, 0, 10)
	for n, N := fp.N().Where(fp.EvenN()).First(); n < 20; n = N.Next() {
		//t.Logf("n=%d", n)
		a = append(a, n)
	}
	if a[0] != 0 || a[9] != 18 || len(a) != 10 {
		t.Errorf("Generating 10 even(n) failed")
	}
	t.Logf("a=%v", a)
}

func TestIntMapper(t *testing.T) {
	a := make([]int64, 0, 10)
	i := 0
	for n, N := fp.N().Map(fp.SquareN()).First(); i < 10; n = N.Next() {
		i++
		//t.Logf("n=%d", n)
		a = append(a, n)
	}
	if a[0] != 0 || a[9] != 81 || len(a) != 10 {
		t.Errorf("Generating 10 squares failed")
	}
	t.Logf("a=%v", a)
}

func TestListSeq(t *testing.T) {
	l := terex.List("a", "b", "c")
	//var seq fp.ListSeq
	var a terex.Atom
	if a, _ = fp.Seq(l).First(); a.Data != "a" {
		t.Errorf("first element expected to be \"a\", is %v", a)
	}
	L := fp.Seq(l).List()
	if L == nil || L.Length() != 3 {
		t.Errorf("expected L to be of length 3, is %v", L.ListString())
	}
	uppercase := func(atom terex.Atom) terex.Atom {
		if atom == terex.NilAtom {
			return atom
		}
		return terex.Atomize(strings.ToUpper(atom.Data.(string)))
	}
	U := fp.Seq(l).Map(uppercase).List()
	if U == nil || U.Length() != 3 {
		t.Errorf("expected U to be of length 3, is %v", U.ListString())
	}
	if U.Car.Data != "A" {
		t.Errorf("expected U to be uppercase, is %s", U.ListString())
	}
}

func TestTreeTraverse1(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "gorgo.terex")
	defer teardown()
	//
	tree := makeTree()
	t.Logf("tree = %s", tree.ListString())
	i := 0
	for node := range fp.TreeDepthFirstCh(makeTree()) {
		t.Logf("node=%s", node)
		i++
	}
	if i != 7 {
		t.Errorf("Expected traversal to produce all 7 nodes, have %d", i)
	}
}

func TestTreeTraverse2(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "gorgo.terex")
	defer teardown()
	//
	tree := makeTree()
	t.Logf("tree = %s", tree.ListString())
	l := fp.Traverse(tree, fp.DepthFirstDir).List()
	t.Logf("list = %s", l.ListString())
	if l.ListString() != "(4 5 2 6 7 3 1)" {
		t.Errorf("Depth-first traversal to result in (4 5 2 6 7 3 1), is %s", l.ListString())
	}
}

func TestTreeTraverse3(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "gorgo.terex")
	defer teardown()
	//
	tree := makeTree()
	t.Logf("tree = %s", tree.ListString())
	l := fp.Traverse(tree, fp.TopDownDir).List()
	t.Logf("list = %s", l.ListString())
	if l.ListString() != "(1 2 4 5 3 6 7)" {
		t.Errorf("Top-down traversal to result in (1 2 4 5 3 6 7), is %s", l.ListString())
	}
}

func TestTreeRange(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "gorgo.terex")
	defer teardown()
	//
	tree := makeTree()
	t.Logf("tree = %s", tree.ListString())
	nodes := make([]fp.TreeNode, 0, 7)
	for node := range fp.Traverse(tree, fp.DepthFirstDir).Range() {
		nodes = append(nodes, node)
	}
	t.Logf("nodes=%v", nodes)
	if len(nodes) != 7 || nodes[6].Node.Car.Data != 1.0 {
		t.Errorf("Expected nodes range to be [4 5 2 6 7 3 1], is %v", nodes)
	}
}

func TestTreeFilter(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "gorgo.terex")
	defer teardown()
	//
	tree := makeTree()
	t.Logf("tree = %s", tree.ListString())
	count = 0
	l := fp.Traverse(tree, fp.DepthFirstDir).Where(fp.IsLeaf()).Map(counter).List()
	t.Logf("list = %s", l.ListString())
	if l.Length() != 4 {
		t.Errorf("Filtered list expected to be of length 4, is %s", l.ListString())
	}
	if count != 4 {
		t.Errorf("Mapper did not count to 4, is %d", count)
	}
}

// ---------------------------------------------------------------------------

// see https://www.geeksforgeeks.org/iterative-postorder-traversal-using-stack/
func makeTree() *terex.GCons {
	l2 := terex.List(2, terex.Atomize(terex.List(4)), 5)
	l6 := terex.Atomize(terex.List(6))
	root := terex.List(1, l2, 3, l6, 7)
	return root
}

var count int
var counter fp.NodeMapper = func(node fp.TreeNode) fp.TreeNode {
	count++
	return node
}
