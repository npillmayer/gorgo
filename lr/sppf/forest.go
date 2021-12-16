package sppf

/*
Code for SPPFs are rare, mostly found in academic papers. One of them
is "SPPF-Style Parsing From Earley Recognisers" by Elizabeth Scott
(https://www.sciencedirect.com/science/article/pii/S1571066108001497).
It describes a binarised variant of an SPPF, which we will not follow.
A more accessible discussion of parse SPPFs may be found in
"Parsing Techniques" by  Dick Grune and Ceriel J.H. Jacobs
(https://dickgrune.com/Books/PTAPG_2nd_Edition/), Section 3.7.3.
Scott explains the downside of this simpler approach:

“We could [create] separate instances of the items for different substring matches,
so if [B→δ●,k], [B→σ●,k'] ∈ Ei where k≠k' then we create two copies of [D→τB●μ,h], one
pointing to each of the two items. In the above example we would create two items [S→SS●,0]
in E3, one in which the second S points to [S→b●,2] and the other in which the second S
points to [S→SS●,1]. This would cause correct derivations to be generated, but it also
effectively embeds all the derivation trees in the construction and, as reported by Johnson,
the size cannot be bounded by O(n^p) for any fixed integer p.
[...]
Grune has described a parser which exploits an Unger style parser to construct the
derivations of a string from the sets produced by Earley’s recogniser. However,
as noted by Grune, in the case where the number of derivations is exponential
the resulting parser will be of at least unbounded polynomial order in worst case.”
(Notation slightly modified by me to conform to notations elsewhere in my
parser packages).

Despite the shortcomings of the forest described by Grune & Jacobs, I won't
implement Scott's improvements. For practical use, the worst case spatial complexity
seems never to materialize. However, after gaining more insights in the future when
using the SPPF for more complex real word scenarios I will be prepared to reconsider.


License

Governed by a 3-Clause BSD license. License file may be found in the root
folder of this module.

Copyright © 2017–2021 Norbert Pillmayer <norbert@pillmayer.com>

*/

import (
	"fmt"
	"io"

	"github.com/npillmayer/gorgo/lr"
	"github.com/npillmayer/gorgo/lr/iteratable"
)

// Forest implements a Shared Packed Parse Forest (SPPF).
// A packed parse forest re-uses existing parse tree nodes between different
// parse trees. For a conventional non-ambiguous parse, a parse forest consists
// of a single tree. Ambiguous grammars, on the other hand, may result in parse
// runs where more than one parse tree is created. To save space these parse
// trees will share common nodes.
//
// Our task is to store nodes representing recognition of a substring of the input, i.e.,
// [A→δ,(x…y)], where A is a grammar symbol and δ denotes the right hand side (RHS)
// of a production. (x…y) is the position interval within the input covered by A.
//
// We split up these nodes in two parts: A symbol node for A, and a RHS-node for δ.
// Symbol nodes fan out via or-edges to RHS-nodes. RHS-nodes fan out to the symbols
// of the RHS in order of their appearance in the corresponding grammar rule.
// If a tree segment is unambiguous, our node
// [A→δ,(x…y)] would be split into [A (x…y)]⟶[δ (x…y)], i.e. connected by an
// or-edge without alternatives (fan-out of A is 1).
// For ambiguous parses, subtrees can be shared if [δ (x…y)] is already found in
// the forest, meaning that there is another derivation of this input span present.
//
// How can we quickly identify nodes [A (x…y)] or [δ (x…y)] to find out if they
// are already present in the forest, and thus can be re-used?
// Symbol nodes will be searched by span (x…y), followed by a check of A
// (for RHS-nodes this will be modified slightly, see remarks below).
// Searching is implemented as a tree of height 2, with the edges labeled by input position
// and the leafs being sets of nodes. The tree is implemented by a map of maps of sets.
// We introduce a small helper type for it.
type searchTree map[uint64]map[uint64]*iteratable.Set // methods below

// Forest is a data structure for a "shared packed parse forest" (SPPF).
// A packed parse forest re-uses existing parse tree nodes between different
// parse trees. For a conventional non-ambiguous parse, a parse forest consists
// of a single tree. Ambiguous grammars, on the other hand, may result in parse
// runs where more than one parse tree is created. To save space these parse
// trees will share common nodes.
type Forest struct {
	symbolNodes searchTree                      // search tree for [A (x…y)] (type SymbolNode)
	rhsNodes    searchTree                      // search tree for RHSs, see type rhsNode
	orEdges     map[*SymbolNode]*iteratable.Set // or-edges from symbols to RHSs, indexed by symbol
	andEdges    map[*rhsNode]*iteratable.Set    // and-edges
	parent      map[*SymbolNode]*SymbolNode     // parent-edges
	root        *SymbolNode
}

// NewForest returns an empty forest.
func NewForest() *Forest {
	return &Forest{
		symbolNodes: make(map[uint64]map[uint64]*iteratable.Set),
		rhsNodes:    make(map[uint64]map[uint64]*iteratable.Set),
		orEdges:     make(map[*SymbolNode]*iteratable.Set),
		andEdges:    make(map[*rhsNode]*iteratable.Set),
		parent:      make(map[*SymbolNode]*SymbolNode),
	}
}

// --- Exported Functions ----------------------------------------------------

// AddReduction adds a node for a reduced grammar rule into the forest.
// The extent of the reduction is derived from the RHS-nodes.
//
// If the RHS is void, nil is returned. For this case, clients should use
// AddEpsilonReduction instead.
func (f *Forest) AddReduction(sym *lr.Symbol, rule int, rhs []*SymbolNode) *SymbolNode {
	if len(rhs) == 0 {
		return nil
	}
	tracer().Debugf("Reduction: %s → RHS = %v", sym.Name, rhs)
	start := rhs[0].Extent.From()
	end := rhs[len(rhs)-1].Extent.To()
	rhsnode := f.addRHSNode(rule, rhs, rhs[0].Extent.From())
	f.addOrEdge(sym, rhsnode, start, end)
	for seq, d := range rhs {
		f.addAndEdge(rhsnode, uint(seq), d.Symbol, d.Extent.From(), d.Extent.To())
		f.parent[d] = f.findSymNode(sym, start, end)
	}
	symnode := f.findSymNode(sym, start, end)
	if sym.Name == "S'" { // S' usually added as start symbol during grammar analysis
		f.root = symnode
	}
	return symnode
}

// AddEpsilonReduction adds a node for a reduced ε-production.
func (f *Forest) AddEpsilonReduction(sym *lr.Symbol, rule int, pos uint64) *SymbolNode {
	rhsnode := f.addRHSNode(rule, []*SymbolNode{}, pos)
	f.addOrEdge(sym, rhsnode, pos, pos)
	symnode := f.findSymNode(sym, pos, pos)
	eps := &lr.Symbol{Name: "ε", Value: -2}
	e := f.addAndEdge(rhsnode, 0, eps, pos, pos)
	f.parent[e.toSym] = symnode
	if sym.Name == "S'" { // S' usually added as start symbol during grammar analysis
		f.root = symnode
	}
	return symnode
}

// AddTerminal adds a node for a recognized terminal into the forest.
func (f *Forest) AddTerminal(t *lr.Symbol, pos uint64) *SymbolNode {
	return f.addSymNode(t, pos, pos+1)
}

// SetRoot tells the parse forest which of the nodes will be the root node.
// This is intended for cases where no top-level artificial symbol S' has
// been wrapped around the grammar (usually done by the grammar analyzer).
func (f *Forest) SetRoot(symnode *SymbolNode) {
	f.root = symnode
}

// --- Nodes -----------------------------------------------------------------

// SymbolNode represents a node in the parse forest, referencing a
// grammar symbol which has been reduced (Earley: completed).
type SymbolNode struct { // this is [A (x…y)]
	Symbol *lr.Symbol // A
	Extent lr.Span    // (x…y), i.e., positions in the input covered by this symbol
}

func makeSym(symbol *lr.Symbol) *SymbolNode {
	return &SymbolNode{Symbol: symbol}
}

// Use as makeSym(A).spanning(x, y), resulting in [A (x…y)]
func (sn *SymbolNode) spanning(from, to uint64) *SymbolNode {
	sn.Extent = lr.Span{from, to}
	return sn
}

func (sn *SymbolNode) String() string {
	return fmt.Sprintf("%s %s", sn.Symbol, sn.Extent.String())
}

// FindSymNode finds a (shared) node for a symbol node in the forest.
func (f *Forest) findSymNode(sym *lr.Symbol, start, end uint64) *SymbolNode {
	return f.symbolNodes.findSymbol(start, end, sym)
}

// addSymNode adds a symbol node to the forest. Returns a reference to a SymbolNode,
// which may already have been in the SPPF beforehand.
func (f *Forest) addSymNode(sym *lr.Symbol, start, end uint64) *SymbolNode {
	sn := f.findSymNode(sym, start, end)
	if sn == nil {
		sn = makeSym(sym).spanning(start, end)
		f.symbolNodes.Add(start, end, sn)
	}
	return sn
}

/*
RHS-Nodes

We are handling ambiguity by inserting multiple RHS nodes per symbol. Refer
to “Parsing Techniques” by  Dick Grune and Ceriel J.H. Jacobs
(https://dickgrune.com/Books/PTAPG_2nd_Edition/), Section 3.7.3.1
Combining Duplicate Subtrees:

“We […] combine all the duplicate subtrees in the forest […] by having only
one copy of a node labeled with a non-terminal A and spanning a given substring
of the input. If A produces that substring in more than one way, more than one
or-arrow will emanate from the OR-node labeled A, each pointing to an AND-node
labeled with a rule number. In this way the AND-OR-tree turns into a directed
acyclic graph […].

It is important to note that two OR-nodes (which represent right-hand sides of
rules) can only be combined if all members of the one node are the same as the
corresponding members of the other node.”

The last remark of Grune & Jacobs leads us to the question of identity of RHS-Nodes.
It is not enough to have [δ (x…y)] as a unique label; we need identity of every sub-symbol
(including its span) as well.

To avoid iterating repeatedly over the children we use a signature-function Σ to
encode the following information:

	let RHS = [δ1 (x1…y1)] [δ2 (x2…y2)] … [δn (xn…yn)]
	then Σ(RHS) := Σ(δ1, x1, δ2, x2, … , δn, xn)

and for a reduced ε-production RHS=[ε (x)]

	Σ(RHS) := Σ(x)

Thus instead of storing [δ (x…y)] as RHS-nodes, we store [δ (x) Σ] as unique
RHS-nodes.
*/

// Nodes [δ (x) Σ] in the parse forest.
type rhsNode struct {
	rule  int    // rule of which this RHS δ is from
	start uint64 // start position in the input
	sigma int32  // signature Σ of RHS children symbol nodes
}

func makeRHS(rule int) *rhsNode {
	return &rhsNode{rule: rule}
}

// Use as makeRHS(δ).identified(x, Σ), resulting in [δ (x) Σ]
func (rhs *rhsNode) identified(start uint64, signature int32) *rhsNode {
	rhs.start = start
	rhs.sigma = signature
	return rhs
}

// rhsSignature hashes over the symbols of a RHS, given a slice of symbols and
// a start position. The latter is used only in cases where RHS=ε.
//
// To randomize input positions, we map them to an array o of offsets.
//
var o = [...]int64{107, 401, 353, 223, 811, 569, 619, 173, 433, 757, 811,
	823, 857, 863, 883, 907, 929, 947, 971, 983}

func rhsSignature(rhs []*SymbolNode, start uint64) int32 {
	const largePrime = int64(143743)
	if len(rhs) == 0 { // ε
		return int32(o[start%uint64(len(o))])
	}
	h := int64(817)
	tracer().Debugf("calc signature of RHS=%v ----------------------", rhs)
	for _, symnode := range rhs {
		if v := abs(symnode.Symbol.Value); v != 0 {
			h *= v
		}
		h %= largePrime
		from := symnode.Extent.From()
		h *= o[(from*from)%uint64(len(o))] + int64(from)
		h %= largePrime
	}
	return int32(h)
}

// FindRHSNode finds a (shared) node for a right hand side in the forest.
func (f *Forest) findRHSNode(rule int, rhs []*SymbolNode, start uint64) *rhsNode {
	signature := rhsSignature(rhs, start)
	return f.rhsNodes.findRHS(start, rule, signature)
}

// addRHSNode adds a symbol node to the forest. Returns a reference to a rhsNode,
// which may already have been in the SPPF beforehand.
func (f *Forest) addRHSNode(rule int, rhs []*SymbolNode, start uint64) *rhsNode {
	node := f.findRHSNode(rule, rhs, start)
	if node == nil {
		signature := rhsSignature(rhs, start)
		node = makeRHS(rule).identified(start, signature)
		f.rhsNodes.Add(start, uint64(rule), node)
	}
	return node
}

// --- Edges -----------------------------------------------------------------

// orEdges are ambiguity forks in the parse forest.
type orEdge struct {
	fromSym *SymbolNode
	toRHS   *rhsNode
}

// addOrEdge inserts an edge between a symbol and a RHS.
// If start or end are not already contained in the forest, they are added.
//
// If the edge already exists, nothing is done.
func (f *Forest) addOrEdge(sym *lr.Symbol, rhs *rhsNode, start, end uint64) {
	tracer().Debugf("Add OR-edge %v ----> %v", sym, rhs.rule)
	sn := f.addSymNode(sym, start, end)
	if e := f.findOrEdge(sn, rhs); e.isNull() {
		e = orEdge{sn, rhs}
		if _, ok := f.orEdges[sn]; !ok {
			f.orEdges[sn] = iteratable.NewSet(0)
		}
		f.orEdges[sn].Add(e)
	}
}

// findOrEdge finds an or-edge starting from a symbol and pointing to an
// RHS-node. If none is found, nullOrEdge is returned.
func (f *Forest) findOrEdge(sn *SymbolNode, rhs *rhsNode) orEdge {
	if edges := f.orEdges[sn]; edges != nil {
		v := edges.FirstMatch(func(el interface{}) bool {
			e := el.(orEdge)
			return e.fromSym == sn && e.toRHS == rhs
		})
		return v.(orEdge)
	}
	return nullOrEdge
}

// nullOrEdge denotes an or-edge that is not present in a graph.
var nullOrEdge = orEdge{}

// isNull checks if an edge is null, i.e. non-existent.
func (e orEdge) isNull() bool {
	return e == nullOrEdge
}

// An andEdge connects a RHS to the symbols it consists of.
type andEdge struct {
	fromRHS  *rhsNode    // RHS node starts the edge
	toSym    *SymbolNode // symbol node this edge points to
	sequence uint        // sequence number 0…n, used for ordering children
}

// addAndEdge inserts an edge between a RHS and a symbol, labeled with a seqence
// number. If start or end are not already contained in the forest, they are
// added. Note that it cannot happen that two edges between identical nodes
// exist for different sequence numbers. The function panics if such a condition
// is found.
//
// If the edge already exists, nothing is done.
func (f *Forest) addAndEdge(rhs *rhsNode, seq uint, sym *lr.Symbol, start, end uint64) andEdge {
	tracer().Debugf("Add AND-edge %v --(%d)--> %v", rhs.rule, seq, sym)
	sn := f.addSymNode(sym, start, end)
	var e andEdge
	if e = f.findAndEdge(rhs, sn); e.isNull() {
		e = andEdge{rhs, sn, seq}
		if _, ok := f.andEdges[rhs]; !ok {
			f.andEdges[rhs] = iteratable.NewSet(0)
		}
		f.andEdges[rhs].Add(e)
	} else if e.sequence != seq {
		panic(fmt.Sprintf("new edge with sequence=%d replaces sequence=%d", seq, e.sequence))
	}
	return e
}

// findAndEdge finds an and-edge starting from an RHS node and pointing to a
// symbol-node. If none is found, nullAndEdge is returned.
func (f *Forest) findAndEdge(rhs *rhsNode, sn *SymbolNode) andEdge {
	if edges := f.andEdges[rhs]; edges != nil {
		v := edges.FirstMatch(func(el interface{}) bool {
			e := el.(andEdge)
			return e.fromRHS == rhs && e.toSym == sn
		})
		if v == nil {
			return nullAndEdge
		}
		return v.(andEdge)
	}
	return nullAndEdge
}

// nullAndEdge denotes an and-edge that is not present in a graph.
var nullAndEdge = andEdge{}

// isNull checks if an edge is null, i.e. non-existent.
func (e andEdge) isNull() bool {
	return e == nullAndEdge
}

// --- searchTree -----------------------------------------------------------------

// searchTree models a tree of height 3 with the edges labeled with p1 and p2.
// Semantics of (p1, p2) differ between symbol-nodes and RHS-nodes:
// For symbols, (p1, p2) = (start, end) input positions,
// for RHS, (p1, p2) = (start, rule).
// The leaf is a parse-forest node, either a symbol node or an RHS-node.
//
// find() searches the leaf at (p1, p2) and tests all nodes there for a given criteria.
// The search criteria is given as a predicate-function, returning true for a match.
func (t searchTree) find(p1, p2 uint64, predicate func(el interface{}) bool) interface{} {
	if t1, ok := t[p1]; ok {
		if t2, ok := t1[p2]; ok {
			return t2.FirstMatch(predicate)
		}
	}
	return nil
}

// find a symbol-node for (start, end, symbol).
func (t searchTree) findSymbol(from, to uint64, sym *lr.Symbol) *SymbolNode {
	node := t.find(from, to, func(el interface{}) bool {
		s := el.(*SymbolNode)
		return s.Symbol == sym
	})
	if node == nil {
		return nil
	}
	return node.(*SymbolNode)
}

// find an RHS-node for (position, rule-no, signature).
func (t searchTree) findRHS(start uint64, rule int, signature int32) *rhsNode {
	node := t.find(start, uint64(rule), func(el interface{}) bool {
		rhs := el.(*rhsNode)
		return rhs.sigma == signature
	})
	if node == nil {
		return nil
	}
	return node.(*rhsNode)
}

// Add adds an item as a leaf of the searchTree-path (p1, p2).
// Semantics of (p1, p2) differ between symbol-nodes and RHS-nodes:
// For symbols, (p1, p2) = (start, end) input positions,
// for RHS, (p1, p2) = (start, rule).
func (t searchTree) Add(p1, p2 uint64, item interface{}) {
	if t1, ok := t[p1]; !ok {
		t[p1] = make(map[uint64]*iteratable.Set)
		t[p1][p2] = iteratable.NewSet(0)
	} else if _, ok := t1[p2]; !ok {
		t[p1][p2] = iteratable.NewSet(0)
	}
	t[p1][p2].Add(item)
}

func (t searchTree) All() *iteratable.Set {
	values := iteratable.NewSet(0)
	for _, t1 := range t {
		for _, set := range t1 {
			values = values.Union(set)
		}
	}
	return values
}

// --- GraphViz --------------------------------------------------------------

// ToGraphViz exports an SPPF to an io.Writer in GrahpViz DOT format.
func ToGraphViz(forest *Forest, w io.Writer) {
	io.WriteString(w, `digraph G {
{ graph [fontname="Helvetica"];
  node [fontname="Helvetica",shape=box,fontsize=10];
  edge [fontname="Helvetica",fontsize=9];
`)
	nodes := forest.rhsNodes.All()
	nodes.Sort(func(x, y interface{}) bool {
		return x.(*rhsNode).rule < y.(*rhsNode).rule
	})
	nodes.IterateOnce()
	for nodes.Next() {
		node := nodes.Item().(*rhsNode)
		io.WriteString(w, fmt.Sprintf("\"rule %d (%d)\" [style=rounded,color=\"#404040\"]\n",
			node.rule, node.sigma))
	}
	nodes = forest.symbolNodes.All()
	nodes.Sort(func(x, y interface{}) bool {
		return x.(*SymbolNode).Extent.From() < y.(*SymbolNode).Extent.From()
	})
	nodes.IterateOnce()
	for nodes.Next() {
		node := nodes.Item().(*SymbolNode)
		if node.Symbol.IsTerminal() {
			io.WriteString(w, fmt.Sprintf("\"%s\" [fillcolor=grey90,style=filled]\n", node.String()))
		} else {
			io.WriteString(w, fmt.Sprintf("\"%s\" []\n", node.String()))
		}
	}
	io.WriteString(w, "}\n")
	for _, set := range forest.orEdges {
		oredges := set.Values()
		for _, e := range oredges {
			edge := e.(orEdge)
			io.WriteString(w, fmt.Sprintf("\"%s\" -> \"rule %d (%d)\" [style=dashed]\n",
				edge.fromSym, edge.toRHS.rule, edge.toRHS.sigma))
		}
	}
	for _, set := range forest.andEdges {
		set.Sort(func(x, y interface{}) bool {
			return x.(andEdge).sequence < y.(andEdge).sequence
		})
		set.IterateOnce()
		for set.Next() {
			edge := set.Item().(andEdge)
			io.WriteString(w, fmt.Sprintf("\"rule %d (%d)\" -> \"%s\" [label=%d]\n", edge.fromRHS.rule,
				edge.fromRHS.sigma, edge.toSym, edge.sequence))
		}
	}
	io.WriteString(w, "{ rank=max;\n")
	// { rank=max; T1; T2; T3 } => all terminals at bottom row
	nodes.IterateOnce()
	for nodes.Next() {
		node := nodes.Item().(*SymbolNode)
		if node.Symbol.IsTerminal() {
			io.WriteString(w, fmt.Sprintf("\"%s\";", node.String()))
		}
	}
	io.WriteString(w, "\n}\n}\n")
}

// ---------------------------------------------------------------------------

func abs(n int) int64 {
	if n < 0 {
		n = -n
	}
	return int64(n)
}
