package lr

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"text/scanner"

	"github.com/emirpasic/gods/lists/arraylist"
	"github.com/emirpasic/gods/sets/treeset"
	"github.com/emirpasic/gods/utils"
	"github.com/npillmayer/gorgo"
	"github.com/npillmayer/gorgo/lr/iteratable"
	"github.com/npillmayer/gorgo/lr/sparse"
)

// TODO: Improve documentation...
// https://stackoverflow.com/questions/12968048/what-is-the-closure-of-a-left-recursive-lr0-item-with-epsilon-transitions
// = optimization
// https://www.cs.bgu.ac.il/~comp151/wiki.files/ps6.html#sec-2-7-3

// Actions for parser action tables.
const (
	ShiftAction  = -1
	AcceptAction = -2
)

// === Closure and Goto-Set Operations =======================================

// Refer to "Crafting A Compiler" by Charles N. Fisher & Richard J. LeBlanc, Jr.
// Section 6.2.1 LR(0) Parsing

// Compute the closure of an Earley item.
func (ga *LRAnalysis) closure(i Item, A *Symbol) *iteratable.Set {
	S := newItemSet()
	S.Add(i)
	return ga.closureSet(S)
}

// Compute the closure of an Earley item.
// https://www.cs.bgu.ac.il/~comp151/wiki.files/ps6.html#sec-2-7-3
func (ga *LRAnalysis) closureSet(S *iteratable.Set) *iteratable.Set {
	C := S.Copy() // add start items to closure
	C.IterateOnce()
	for C.Next() {
		item := asItem(C.Item())
		A := item.PeekSymbol()           // get symbol A after dot
		if A != nil && !A.IsTerminal() { // A is non-terminal
			R := ga.g.FindNonTermRules(A, true)
			if New := R.Difference(C); !New.Empty() {
				C.Union(New)
			}
		}
	}
	return C
}

func (ga *LRAnalysis) gotoSet(closure *iteratable.Set, A *Symbol) (*iteratable.Set, *Symbol) {
	// for every item in closure C
	// if item in C:  N -> ... *A ...
	//     advance N -> ... A * ...
	gotoset := newItemSet()
	for _, x := range closure.Values() {
		i := asItem(x)
		if i.PeekSymbol() == A {
			ii := i.Advance()
			tracer().Debugf("goto(%s) -%s-> %s", i, A, ii)
			gotoset.Add(ii)
		}
	}
	//gotoset.Dump()
	return gotoset, A
}

func (ga *LRAnalysis) gotoSetClosure(i *iteratable.Set, A *Symbol) (*iteratable.Set, *Symbol) {
	gotoset, _ := ga.gotoSet(i, A)
	//T().Infof("gotoset  = %v", gotoset)
	gclosure := ga.closureSet(gotoset)
	//T().Infof("gclosure = %v", gclosure)
	tracer().Debugf("goto(%s) --%s--> %s", itemSetString(i), A, itemSetString(gclosure))
	return gclosure, A
}

// === CFSM Construction =====================================================

// CFSMState is a state within the CFSM for a grammar.
type CFSMState struct {
	ID     uint            // serial ID of this state
	items  *iteratable.Set // configuration items within this state
	Accept bool            // is this an accepting state?
}

// CFSM edge between 2 states, directed and with a terminal
type cfsmEdge struct {
	from  *CFSMState
	to    *CFSMState
	label *Symbol
}

// Dump is a debugging helper
func (s *CFSMState) Dump() {
	tracer().Debugf("--- state %03d -----------", s.ID)
	Dump(s.items)
	tracer().Debugf("-------------------------")
}

func (s *CFSMState) isErrorState() bool {
	return s.items.Size() == 0
}

// Create a state from an item set
func state(id uint, iset *iteratable.Set) *CFSMState {
	s := &CFSMState{ID: id}
	if iset == nil {
		s.items = newItemSet()
	} else {
		s.items = iset
	}
	return s
}

/* no longer used
func (s *CFSMState) allItems() []interface{} {
	vals := s.items.Values()
	return vals
}
*/

func (s *CFSMState) String() string {
	return fmt.Sprintf("(state %d | [%d])", s.ID, s.items.Size())
}

func (s *CFSMState) containsCompletedStartRule() bool {
	for _, x := range s.items.Values() {
		i := asItem(x)
		if i.rule.Serial == 0 && i.PeekSymbol() == nil {
			return true
		}
	}
	return false
}

// Create an edge
func edge(from, to *CFSMState, label *Symbol) *cfsmEdge {
	return &cfsmEdge{
		from:  from,
		to:    to,
		label: label,
	}
}

// We need this for the set of states. It sorts states by serial ID.
func stateComparator(s1, s2 interface{}) int {
	c1 := s1.(*CFSMState)
	c2 := s2.(*CFSMState)
	return utils.IntComparator(int(c1.ID), int(c2.ID))
}

// Add a state to the CFSM. Checks first if state is present.
func (c *CFSM) addState(iset *iteratable.Set) *CFSMState {
	s := c.findStateByItems(iset)
	if s == nil {
		s = state(c.cfsmIds, iset)
		c.cfsmIds++
	}
	c.states.Add(s)
	return s
}

// Find a CFSM state by the contained item set.
func (c *CFSM) findStateByItems(iset *iteratable.Set) *CFSMState {
	it := c.states.Iterator()
	for it.Next() {
		s := it.Value().(*CFSMState)
		if s.items.Equals(iset) {
			return s
		}
	}
	return nil
}

func (c *CFSM) addEdge(s0, s1 *CFSMState, sym *Symbol) *cfsmEdge {
	e := edge(s0, s1, sym)
	c.edges.Add(e)
	return e
}

func (c *CFSM) allEdges(s *CFSMState) []*cfsmEdge {
	it := c.edges.Iterator()
	r := make([]*cfsmEdge, 0, 2)
	for it.Next() {
		e := it.Value().(*cfsmEdge)
		if e.from == s {
			r = append(r, e)
		}
	}
	return r
}

// CFSM is the characteristic finite state machine for a LR grammar, i.e. the
// LR(0) state diagram. Will be constructed by a TableGenerator.
// Clients normally do not use it directly. Nevertheless, there are some methods
// defined on it, e.g, for debugging purposes, or even to
// compute your own tables from it.
type CFSM struct {
	g       *Grammar        // this CFSM is for Grammar g
	states  *treeset.Set    // all the states
	edges   *arraylist.List // all the edges between states
	S0      *CFSMState      // start state
	cfsmIds uint            // serial IDs for CFSM states
}

// create an empty (initial) CFSM automata.
func emptyCFSM(g *Grammar) *CFSM {
	c := &CFSM{g: g}
	c.states = treeset.NewWith(stateComparator)
	c.edges = arraylist.New()
	return c
}

// TableGenerator is a generator object to construct LR parser tables.
// Clients usually create a Grammar G, then a LRAnalysis-object for G,
// and then a table generator. TableGenerator.CreateTables() constructs
// the CFSM and parser tables for an LR-parser recognizing grammar G.
type TableGenerator struct {
	g            *Grammar
	ga           *LRAnalysis
	dfa          *CFSM
	gototable    *Table
	actiontable  *Table
	HasConflicts bool
}

// NewTableGenerator creates a new TableGenerator for a (previously analysed) grammar.
func NewTableGenerator(ga *LRAnalysis) *TableGenerator {
	lrgen := &TableGenerator{}
	lrgen.g = ga.Grammar()
	lrgen.ga = ga
	return lrgen
}

// CFSM returns the characteristic finite state machine (CFSM) for a grammar.
// Usually clients call lrgen.CreateTables() beforehand, but it is possible
// to call lrgen.CFSM() directly. The CFSM will be created, if it has not
// been constructed previously.
func (lrgen *TableGenerator) CFSM() *CFSM {
	if lrgen.dfa == nil {
		lrgen.dfa = lrgen.buildCFSM()
	}
	return lrgen.dfa
}

// GotoTable returns the GOTO table for LR-parsing a grammar. The tables have to be
// built by calling CreateTables() previously (or a separate call to
// BuildGotoTable(...).)
func (lrgen *TableGenerator) GotoTable() *Table {
	if lrgen.gototable == nil {
		tracer().P("lr", "gen").Errorf("tables not yet initialized")
	}
	return lrgen.gototable
}

// ActionTable returns the ACTION table for LR-parsing a grammar. The tables have to be
// built by calling CreateTables() previously (or a separate call to
// BuildSLR1ActionTable(...).)
func (lrgen *TableGenerator) ActionTable() *Table {
	if lrgen.actiontable == nil {
		tracer().P("lr", "gen").Errorf("tables not yet initialized")
	}
	return lrgen.actiontable
}

// CreateTables creates the necessary data structures for an SLR parser.
func (lrgen *TableGenerator) CreateTables() {
	lrgen.dfa = lrgen.buildCFSM()
	lrgen.gototable = lrgen.BuildGotoTable()
	lrgen.actiontable, lrgen.HasConflicts = lrgen.BuildSLR1ActionTable()
}

// AcceptingStates returns all states of the CFSM which represent an accept action.
// Clients have to call CreateTables() first.
func (lrgen *TableGenerator) AcceptingStates() []uint {
	if lrgen.dfa == nil {
		tracer().Errorf("tables not yet generated; call CreateTables() first")
		return nil
	}
	acc := make([]uint, 0, 3)
	for _, x := range lrgen.dfa.states.Values() {
		state := x.(*CFSMState)
		if state.Accept {
			//acc = append(acc, state.ID)
			it := lrgen.dfa.edges.Iterator()
			for it.Next() {
				e := it.Value().(*cfsmEdge)
				if e.to.ID == state.ID {
					acc = append(acc, e.from.ID)
				}
			}
		}
	}
	unique(acc)
	return acc
}

// Construct the characteristic finite state machine CFSM for a grammar.
func (lrgen *TableGenerator) buildCFSM() *CFSM {
	tracer().Debugf("=== build CFSM ==================================================")
	G := lrgen.g
	cfsm := emptyCFSM(G)
	closure0 := lrgen.ga.closure(StartItem(G.rules[0]))
	item, sym := StartItem(G.rules[0])
	tracer().Debugf("Start item=%v/%v", item, sym)
	tracer().Debugf("----------")
	Dump(closure0)
	tracer().Debugf("----------")
	cfsm.S0 = cfsm.addState(closure0)
	cfsm.S0.Dump()
	S := treeset.NewWith(stateComparator)
	S.Add(cfsm.S0)
	for S.Size() > 0 {
		s := S.Values()[0].(*CFSMState)
		S.Remove(s)
		G.EachSymbol(func(A *Symbol) interface{} {
			tracer().Debugf("checking goto-set for symbol = %v", A)
			gotoset, _ := lrgen.ga.gotoSetClosure(s.items, A)
			snew := cfsm.findStateByItems(gotoset)
			if snew == nil {
				snew = cfsm.addState(gotoset)
				if !snew.isErrorState() {
					S.Add(snew)
					if snew.containsCompletedStartRule() {
						snew.Accept = true
					}
				}
			}
			if !snew.isErrorState() {
				cfsm.addEdge(s, snew, A)
			}
			snew.Dump()
			return nil
		})
		tracer().Debugf("-----------------------------------------------------------------")
	}
	return cfsm
}

// CFSM2GraphViz exports a CFSM to the Graphviz Dot format, given a filename.
func (c *CFSM) CFSM2GraphViz(filename string) {
	f, err := os.Create(filename)
	if err != nil {
		panic(fmt.Sprintf("file open error: %v", err.Error()))
	}
	defer f.Close()
	f.WriteString(`digraph {
graph [splines=true, fontname=Helvetica, fontsize=10];
node [shape=Mrecord, style=filled, fontname=Helvetica, fontsize=10];
edge [fontname=Helvetica, fontsize=10];

`)
	for _, x := range c.states.Values() {
		s := x.(*CFSMState)
		f.WriteString(fmt.Sprintf("s%03d [fillcolor=%s label=\"{%03d | %s}\"]\n",
			s.ID, nodecolor(s), s.ID, forGraphviz(s.items)))
	}
	it := c.edges.Iterator()
	for it.Next() {
		x := it.Value()
		edge := x.(*cfsmEdge)
		f.WriteString(fmt.Sprintf("s%03d -> s%03d [label=\"%s\"]\n", edge.from.ID, edge.to.ID, edge.label))
	}
	f.WriteString("}\n")
}

func nodecolor(state *CFSMState) string {
	if state.Accept {
		return "lightgray"
	}
	return "white"
}

// ===========================================================================

// BuildGotoTable builds the GOTO table. This is normally not called directly, but rather
// via CreateTables().
func (lrgen *TableGenerator) BuildGotoTable() *Table {
	statescnt := lrgen.dfa.states.Size()
	var maxtok gorgo.TokType
	var mintok gorgo.TokType
	lrgen.g.EachSymbol(func(A *Symbol) interface{} {
		tracer().Infof("%q.token-type = %d", A.Name, A.TokenType())
		if A.TokenType() > maxtok { // find maximum token value
			maxtok = A.TokenType()
		} else if A.TokenType() < mintok { // find maximum token value
			mintok = A.TokenType()
		}
		return nil
	})
	extent := uint(maxtok - mintok + 1)
	tracer().Infof("GOTO table of size %d x (%d-%d=%d)", statescnt, maxtok, mintok, extent)
	matrix := sparse.NewIntMatrix(uint(statescnt), extent, sparse.DefaultNullValue)
	gototable := &Table{
		matrix: matrix,
		mincol: mintok,
	}
	states := lrgen.dfa.states.Iterator()
	for states.Next() {
		state := states.Value().(*CFSMState)
		edges := lrgen.dfa.allEdges(state)
		for _, e := range edges {
			//T().Debugf("edge %s --%v--> %v", state, e.label, e.to)
			//T().Debugf("GOTO (%d , %d ) = %d", state.ID, symvalue(e.label), e.to.ID)
			//
			gototable.set(state.ID, gorgo.TokType(e.label.Value), int32(e.to.ID))
		}
	}
	return gototable
}

// GotoTableAsHTML exports a GOTO-table in HTML-format.
func GotoTableAsHTML(lrgen *TableGenerator, w io.Writer) {
	if lrgen.gototable == nil {
		tracer().Errorf("GOTO table not yet created, cannot export to HTML")
		return
	}
	parserTableAsHTML(lrgen, "GOTO", lrgen.gototable, w)
}

// ActionTableAsHTML exports the SLR(1) ACTION-table in HTML-format.
func ActionTableAsHTML(lrgen *TableGenerator, w io.Writer) {
	if lrgen.actiontable == nil {
		tracer().Errorf("ACTION table not yet created, cannot export to HTML")
		return
	}
	parserTableAsHTML(lrgen, "ACTION", lrgen.actiontable, w)
}

func parserTableAsHTML(lrgen *TableGenerator, tname string, table *Table, w io.Writer) {
	var symvec = make([]*Symbol, len(lrgen.g.terminals)+len(lrgen.g.nonterminals))
	io.WriteString(w, "<html><body>\n")
	io.WriteString(w, "<img src=\"cfsm.png\"/><p>")
	io.WriteString(w, fmt.Sprintf("%s table of size = %d<p>", tname, table.matrix.ValueCount()))
	io.WriteString(w, "<table border=1 cellspacing=0 cellpadding=5>\n")
	io.WriteString(w, "<tr bgcolor=#cccccc><td></td>\n")
	j := 0
	lrgen.g.EachSymbol(func(A *Symbol) interface{} {
		io.WriteString(w, fmt.Sprintf("<td>%s</td>", A))
		symvec[j] = A
		j++
		return nil
	})
	io.WriteString(w, "</tr>\n")
	states := lrgen.dfa.states.Iterator()
	var td string // table cell
	for states.Next() {
		state := states.Value().(*CFSMState)
		io.WriteString(w, fmt.Sprintf("<tr><td>state %d</td>\n", state.ID))
		for _, A := range symvec {
			if A.Value < 0 {
				panic("cannot use parser table for symbol with symbol-value < 0")
			}
			v1, v2 := table.Values(state.ID, gorgo.TokType(A.Value))
			if v1 == table.NullValue() {
				td = "&nbsp;"
			} else if v2 == table.NullValue() {
				td = fmt.Sprintf("%d", v1)
			} else {
				td = fmt.Sprintf("%d/%d", v1, v2)
			}
			io.WriteString(w, "<td>")
			io.WriteString(w, td)
			io.WriteString(w, "</td>\n")
		}
		io.WriteString(w, "</tr>\n")
	}
	io.WriteString(w, "</table></body></html>\n")
}

// ===========================================================================

// BuildLR0ActionTable contructs the LR(0) Action table. This method is not called by
// CreateTables(), as we normally use an SLR(1) parser and therefore an action table with
// lookahead included. This method is provided as an add-on.
func (lrgen *TableGenerator) BuildLR0ActionTable() (*Table, bool) {
	statescnt := uint(lrgen.dfa.states.Size())
	tracer().Infof("ACTION.0 table of size %d x 1", statescnt)
	matrix := sparse.NewIntMatrix(statescnt, 1, sparse.DefaultNullValue)
	actions := &Table{
		matrix: matrix,
		mincol: 0,
	}
	return lrgen.buildActionTable(actions, false)
}

// BuildSLR1ActionTable constructs the SLR(1) Action table. This method is normally not called
// by clients, but rather via CreateTables(). It builds an action table including
// lookahead (using the FOLLOW-set created by the grammar analyzer).
func (lrgen *TableGenerator) BuildSLR1ActionTable() (*Table, bool) {
	statescnt := uint(lrgen.dfa.states.Size())
	var maxtok gorgo.TokType
	var mintok gorgo.TokType
	lrgen.g.EachSymbol(func(A *Symbol) interface{} {
		if A.TokenType() > maxtok { // find minimum and  maximum token value
			maxtok = A.TokenType()
		} else if A.TokenType() < mintok {
			mintok = A.TokenType()
		}
		return nil
	})
	extent := uint(maxtok - mintok + 1)
	tracer().Infof("ACTION.1 table of size %d x (%d-%d=%d)", statescnt, maxtok, mintok, extent)
	matrix := sparse.NewIntMatrix(statescnt, extent, sparse.DefaultNullValue)
	actions := &Table{
		matrix: matrix,
		mincol: mintok,
	}
	// TODO shilft all input token values by mintok
	return lrgen.buildActionTable(actions, true)
}

// For building an ACTION table we iterate over all the states of the CFSM.
// An inner loop iterates over alle the Earley items within a CFSM-state.
// If an item has a non-terminal immediately after the dot, we produce a shift
// entry. If an item's dot is behind the complete (non-epsilon) RHS of a rule,
// then
// - for the LR(0) case: we produce a reduce-entry for the rule
// - for the SLR case: we produce a reduce-entry for for the rule for each
//   terminal from FOLLOW(LHS).
//
// The table is returned as a sparse matrix, where every entry may consist of up
// to 2 entries, thus allowing for shift/reduce- or reduce/reduce-conflicts.
//
// Shift entries are represented as -1.  Reduce entries are encoded as the
// ordinal no. of the grammar rule to reduce. 0 means reducing the start rule,
// i.e., accept.
func (lrgen *TableGenerator) buildActionTable(actions *Table, slr1 bool) (*Table, bool) {
	//func (lrgen *TableGenerator) buildActionTable(actions *sparse.IntMatrix, slr1 bool) (
	//*sparse.IntMatrix, bool) {
	//
	hasConflicts := false
	states := lrgen.dfa.states.Iterator()
	for states.Next() {
		state := states.Value().(*CFSMState)
		tracer().Debugf("--- state %d --------------------------------", state.ID)
		for _, v := range state.items.Values() {
			tracer().Debugf("item in s%d = %v", state.ID, v)
			i := asItem(v)
			A := i.PeekSymbol()
			prefix := i.Prefix()
			tracer().Debugf("symbol at dot = %v, prefix = %v", A, prefix)
			if A != nil && A.IsTerminal() { // create a shift entry
				P := pT(state, A)
				tracer().Debugf("    creating action entry --%v--> %d", A, P)
				if slr1 {
					if a1 := actions.Value(state.ID, A.TokenType()); a1 != actions.NullValue() {
						tracer().Debugf("    %s is 2nd action", valstring(int32(P), actions))
						if a1 == ShiftAction {
							tracer().Debugf("    relax, double shift")
						} else {
							hasConflicts = true
							actions.add(state.ID, A.TokenType(), int32(P))
						}
					} else {
						actions.add(state.ID, A.TokenType(), int32(P))
					}
					tracer().Debugf(actionEntry(state.ID, A.TokenType(), actions))
				} else {
					actions.add(state.ID, 1, int32(P))
				}
			}
			if A == nil { // we are at the end of a rule
				rule, inx := lrgen.g.matchesRHS(i.rule.LHS, prefix) // find the rule
				if inx >= 0 {                                       // found => create a reduce entry
					if slr1 {
						lookaheads := lrgen.ga.Follow(rule.LHS)
						tracer().Debugf("    Follow(%v) = %v", rule.LHS, lookaheads)
						laslice := lookaheads.AppendTo(nil)
						//for _, la := range lookaheads {
						for _, la := range laslice {
							a1, a2 := actions.Values(state.ID, gorgo.TokType(la))
							if a1 != actions.NullValue() || a2 != actions.NullValue() {
								tracer().Debugf("    %s is 2nd action", valstring(int32(inx), actions))
								hasConflicts = true
							}
							actions.add(state.ID, gorgo.TokType(la), int32(inx)) // reduce rule[inx]
							tracer().Debugf("    creating reduce_%d action entry @ %v for %v", inx, la, rule)
							tracer().Debugf(actionEntry(state.ID, gorgo.TokType(la), actions))
						}
					} else {
						tracer().Debugf("    creating reduce_%d action entry for %v", inx, rule)
						actions.add(state.ID, 1, int32(inx)) // reduce rule[inx]
					}
				}
			}
		}
	}
	return actions, hasConflicts
}

func pT(state *CFSMState, terminal *Symbol) int {
	if terminal.TokenType() == scanner.EOF {
		return AcceptAction
	}
	return ShiftAction
}

type Table struct {
	matrix *sparse.IntMatrix
	mincol gorgo.TokType // lowest value for index j => offset for access
}

func (t *Table) add(i uint, tt gorgo.TokType, val int32) {
	j := tt - t.mincol
	if j < 0 {
		panic(fmt.Sprintf("lr.Table.add() with index < 0: %d", j))
	}
	t.matrix.Add(i, uint(j), val)
}

func (t *Table) set(i uint, tt gorgo.TokType, val int32) {
	j := tt - t.mincol
	if j < 0 {
		panic(fmt.Sprintf("lr.Table.set() with index < 0: %d", j))
	}
	t.matrix.Set(i, uint(j), val)
}

func (t *Table) NullValue() int32 {
	return t.matrix.NullValue()
}

func (t *Table) Value(i uint, tt gorgo.TokType) int32 {
	j := tt - t.mincol
	if j < 0 {
		panic(fmt.Sprintf("lr.Table.Value() with index < 0: %d", j))
	}
	return t.matrix.Value(i, uint(j))
}

func (t *Table) Values(i uint, tt gorgo.TokType) (int32, int32) {
	j := tt - t.mincol
	if j < 0 {
		panic(fmt.Sprintf("lr.Table.Values() with index < 0: %d", j))
	}
	return t.matrix.Values(i, uint(j))
}

// ----------------------------------------------------------------------

func unique(in []uint) []uint { // from slice tricks
	sortUInts(in)
	j := 0
	for i := 1; i < len(in); i++ {
		if in[j] == in[i] {
			continue
		}
		j++
		// in[i], in[j] = in[j], in[i] // preserve the original data
		in[j] = in[i] // only set what is required
	}
	result := in[:j+1]
	return result
}

func actionEntry(stateID uint, la gorgo.TokType, aT *Table) string {
	a1, a2 := aT.Values(stateID, la)
	return fmt.Sprintf("Action(%s,%s)", valstring(a1, aT), valstring(a2, aT))
}

// valstring is a short helper to stringify an action table entry.
func valstring(v int32, m *Table) string {
	if v == m.NullValue() {
		return "<none>"
	} else if v == AcceptAction {
		return "<accept>"
	} else if v == ShiftAction {
		return "<shift>"
	}
	return fmt.Sprintf("<reduce %d>", v)
}

func itemSetString(S *iteratable.Set) string {
	var b bytes.Buffer
	b.WriteString("{")
	S.IterateOnce()
	first := true
	for S.Next() {
		item := S.Item().(Item)
		if first {
			b.WriteString(" ")
			first = false
		} else {
			b.WriteString(", ")
		}
		b.WriteString(item.String())
	}
	b.WriteString(" }")
	return b.String()
}

// sortUInts sorts a slice of ints in increasing order.
func sortUInts(x []uint) { sort.Sort(UIntSlice(x)) }

// UIntSlice attaches the methods of sort.Interface to []uint.
type UIntSlice []uint

func (x UIntSlice) Len() int           { return len(x) }
func (x UIntSlice) Less(i, j int) bool { return x[i] < x[j] }
func (x UIntSlice) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }
