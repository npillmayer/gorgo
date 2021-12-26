package sppf

/*
License

Governed by a 3-Clause BSD license. License file may be found in the root
folder of this module.

Copyright © 2017–2022 Norbert Pillmayer <norbert@pillmayer.com>
*/

import (
	"github.com/npillmayer/gorgo"
	"github.com/npillmayer/gorgo/lr"
)

/*
Traversing a parse forest resulting from an ambiguous grammar in practice
mainly comes in two variants:

- The client has additional knowledge of how to prune the parse forest and
  select one tree. Using arithmetic expression 1+2+3 as an example, selecting
  the tree representing left-associativity would result in the pruning of 1+(2+3),
  leaving (1+2)+3 as the unambiguous parse-tree.

- The choice of parse tree is irrelevant. An example for this is determining
  the result of an arithmetic expression, ignoring associativity: 1+2+3=5,
  independently of associativity.

Many heavily ambiguous grammars can be invented (and often are for tests and for
research), but real-world clients most of the time have a clear understanding
of how strings for a given grammar are supposed to be structured. It is more
a challenge of communicating this easily to a parser.

The focus of this package is therefore to enable the user to prune ambiguous
parse trees, without making silent descicions which cannot be influenced by
the user. That means: having sensible defaults, but provide options for the
advanced user.

After pruning we are left with an unambiguous parse tree. The usual strategy is
to create an AST (abstract syntax tree) from it and go from there. We provide
tree pattern matching and term-rewriting for making these tasks easier.
*/

// RuleNode represents a node occuring during a parse tree/forest walk.
type RuleNode struct {
	symbol *SymbolNode
	Value  interface{} // user-defined value of a node
}

// Symbol returns the grammar symbol a RuleNode refers to.
// It is either a terminal or the LHS of a reduced rule.
func (rnode *RuleNode) Symbol() *lr.Symbol {
	return rnode.symbol.Symbol
}

// RHS collects the children symbols of a node as a slice.
// It uses a pruner to decide between ambiguous RHS variants.
func (c *Cursor) RHS(sym *SymbolNode) (int, []*RuleNode) {
	//rhs := c.forest.disambiguate(c.current.symbol, c.pruner)
	rhs := c.forest.disambiguate(sym, c.pruner)
	if rhs == nil {
		return -1, nil
	}
	if iter, ok := c.forest.children(rhs); ok {
		edges := c.forest.andEdges[rhs]
		tracer().Debugf("RHS(%s) has length %d", sym, edges.Size())
		rhsnodes := make([]*RuleNode, edges.Size())
		var rhschild *SymbolNode
		rhschild, iter = iter()
		i := 0
		for ; rhschild != nil; rhschild, iter = iter() {
			tracer().Debugf("   RHS #%d = %s", i, rhschild)
			rhsnodes[i] = &RuleNode{symbol: rhschild}
			i++
		}
		return rhs.rule, rhsnodes
	}
	return rhs.rule, nil
}

// Span returns the span of input symbols this rule covers.
func (rnode *RuleNode) Span() gorgo.Span {
	return rnode.symbol.Extent
}

// HasConflict returns true if this node is ambiguous.
func (rnode *RuleNode) HasConflict() bool {
	return false // TODO
}

// Root returns the root node of a parse forest.
func (f *Forest) Root() *RuleNode {
	if f == nil || len(f.symbolNodes) == 0 {
		return nil
	}
	return &RuleNode{
		symbol: f.root,
	}
}

// A Cursor is a movable mark within a parse forest, intended for navigating over
// rule nodes. It abstracts away the notion of the and-or-tree. Clients therefore
// are able to view the parse forest as a tree of SymbolNodes.
type Cursor struct {
	forest    *Forest
	current   *RuleNode
	pruner    Pruner
	startNode *RuleNode
	stack     []childIterator
}

// SetCursor sets up a cursor at a given rule node in a given forest.
// If rnode is nil, the cursor will be set up at the root node of the forest.
//
// A pruner may be given for solving disambiguities. If it is nil, parse tree variants
// will be selected at random.
func (f *Forest) SetCursor(rnode *RuleNode, pruner Pruner) *Cursor {
	if rnode == nil {
		if rnode = f.Root(); rnode == nil {
			return nil
		}
	}
	if pruner == nil {
		pruner = DontCarePruner
	}
	return &Cursor{
		forest:    f,
		current:   rnode,
		pruner:    pruner,
		startNode: rnode,
		stack:     make([]childIterator, 0, 256),
	}
}

type childIterator func() (*SymbolNode, childIterator)

func nullChildIterator() (*SymbolNode, childIterator) {
	return nil, nullChildIterator
}

func (f *Forest) children(rhs *rhsNode) (childIterator, bool) {
	if children, ok := f.andEdges[rhs]; ok {
		children.IterateOnce()
		var iterator childIterator
		iterator = func() (*SymbolNode, childIterator) {
			if children.Next() { // TODO iterate by child.selector
				// maybe sort the rhs before iterating
				// TODO implement backwards iteration, too => Direction
				childEdge := children.Item().(andEdge)
				return childEdge.toSym, iterator
			}
			return nil, nullChildIterator
		}
		return iterator, true
	}
	return nullChildIterator, false
}

// Pruner is an interface type for an entity to help prune ambiguous children
// edges.
type Pruner interface {
	prune(sym *SymbolNode, rhs *rhsNode) bool
}

type dcp struct{}

func (p dcp) prune(sym *SymbolNode, rhs *rhsNode) bool {
	tracer().Infof("Ambiguous symbol node %v detected", sym)
	return false // do not prune anything
}

// DontCarePruner never prunes an ambiguity alternative, thus resulting
// in always selecting the first alternative considered.
// It is the default Pruner if none is given by the caller of a Cursor.
var DontCarePruner dcp = dcp{}

func (f *Forest) disambiguate(sym *SymbolNode, pruner Pruner) *rhsNode {
	if choices, ok := f.orEdges[sym]; ok {
		if choices.Size() == 1 {
			return choices.First().(orEdge).toRHS
		}
		match := choices.FirstMatch(func(el interface{}) bool {
			rhs := el.(orEdge).toRHS
			return !pruner.prune(sym, rhs)
		})
		if match != nil {
			return match.(orEdge).toRHS
		}
	}
	return nil
}

// Up moves the cursor up to the parent node of the current node, if any.
func (c *Cursor) Up() (*RuleNode, bool) {
	if parent, ok := c.forest.parent[c.current.symbol]; ok {
		c.current.symbol = parent
		tracer().Debugf("UP Cursor @ %v", c.current.Symbol())
		c.stack = c.stack[:len(c.stack)-1]
		return c.current, true
	}
	return c.current, false
}

// Down moves the cursor down to the first child of the curent node, if any.
// dir lets clients start at either the leftmost child (default) or the rightmost
// child.
func (c *Cursor) Down(dir Direction) (*RuleNode, bool) {
	rhs := c.forest.disambiguate(c.current.symbol, c.pruner)
	if rhs == nil {
		return c.current, false
	}
	if iter, ok := c.forest.children(rhs); ok {
		c.stack = append(c.stack, iter)
		var child *SymbolNode
		if child, iter = iter(); child != nil {
			c.current.symbol = child
			tracer().Debugf("DOWN Cursor @ %v", c.current.Symbol())
			return c.current, true
		}
	}
	return c.current, false
}

// Sibling moves the cursor to the next sibling of the current node, if any.
func (c *Cursor) Sibling() (*RuleNode, bool) {
	iter := c.stack[len(c.stack)-1]
	var sym *SymbolNode
	if sym, iter = iter(); sym != nil {
		c.current.symbol = sym
		tracer().Debugf("SIBLING Cursor @ %v", c.current.Symbol())
		return c.current, true
	}
	return c.current, false
}

// TopDown traverses a sub-tree top-down, applying Listener-methods for all nodes
// encountered. It returns a user-defined value, calculated by the listener.
func (c *Cursor) TopDown(listener Listener, dir Direction, breakmode Breakmode) interface{} {
	c.startNode = c.current
	tracer().Debugf("TopDown starting at node %v", c.current.Symbol())
	value := c.traverseTopDown(listener, dir, breakmode, 0)
	return value
}

func (c *Cursor) traverseTopDown(listener Listener, dir Direction, breakmode Breakmode, level int) interface{} {
	var value interface{}
	if sym := c.current.Symbol(); sym.IsTerminal() {
		// TODO find position of token
		ctxt := makeCtxt(c.current.symbol.Extent, level+1, -1, nil)
		return listener.Terminal(sym.Value, sym.TokenType(), ctxt)
	}
	tracer().Debugf(">>> %s", c.current.symbol)
	ruleno, rhsNodes := c.RHS(c.current.symbol)
	tracer().Debugf("%s.rhsNodes=%v", c.current.Symbol(), rhsNodes)
	localAttributes := listener.MakeAttrs(c.current.Symbol())
	ctxt := makeCtxt(c.current.Span(), level, ruleno, localAttributes)
	doContinue := listener.EnterRule(c.current.Symbol(), rhsNodes, ctxt)
	if doContinue || breakmode == Continue { // listener signalled us to traverse children nodes
		i := 0
		if dir == RtoL {
			i = len(rhsNodes) - 1
		}
		if _, ok := c.Down(dir); ok {
			for ; ok; _, ok = c.Sibling() {
				chvalue := c.traverseTopDown(listener, dir, breakmode, level+1)
				tracer().Debugf("child value[%d] = %v", i, chvalue)
				rhsNodes[i].Value = chvalue
				i += int(dir)
			}
			c.Up()
		}
	}
	value = listener.ExitRule(c.current.Symbol(), rhsNodes, ctxt)
	tracer().Debugf("<<< %s", c.current.symbol)
	return value
}

// BottomUp TODO
func (c *Cursor) BottomUp(Listener, Direction, Breakmode) interface{} {
	panic("sppf.Cursor.BottomUp() not yet implemented")
}

// Direction lets clients decide wether children nodes should be traversed left-to-right
// (default) or right-to-left.
type Direction int

// Children nodes may be traversed left-to-right (default) or right-to-left.
const (
	LtoR Direction = 1
	RtoL Direction = -1
)

// Breakmode is a client hint wether to stop traversing on break-signals or not.
type Breakmode int

// Setting Continue will always traverse a complete (sub-)tree. Break will skip
// traversing sub-tree as soon as an Enter-function signals a break.
const (
	Continue Breakmode = iota
	Break
)

// --- Listener --------------------------------------------------------------

// Listener is a type for walking a parse tree/forest.
//
// Arguments are:
//
//     - *lr.Symbol:  the grammar symbol at the current node
//     - []*RuleNode: the right-hand side of the grammar production at this node
//     - RuleCtxt:    contextual information for the node
//
// enterRule returns a boolean value indicating if the traversal should continue to
// the children of this node. ExitRule and Terminal may return user-defined values
// to be propagated upwards of the tree.
//
// Conflict is called whenever the traversal encounters an ambiguous node, i.e. one
// where the parse symbol has more than one right-hand side, resulting from different
// applications of grammar productions. The return value indicates the positional number
// of the RHS-variant to be selected. If it returns an error, traversal will be aborted.
// (If in doubt what to do, simply return (0, nil).)
type Listener interface {
	EnterRule(*lr.Symbol, []*RuleNode, RuleCtxt) bool
	ExitRule(*lr.Symbol, []*RuleNode, RuleCtxt) interface{}
	Terminal(int, interface{}, RuleCtxt) interface{}
	Conflict(*lr.Symbol, RuleCtxt) (int, error)
	MakeAttrs(*lr.Symbol) interface{}
}

// RuleCtxt is a context structure for Listeners.
type RuleCtxt struct {
	Span      gorgo.Span  // span of input symbols covered by this rule
	Level     int         // nesting level
	RuleIndex int         // -1 for terminals
	Attrs     interface{} // client-defined attributes local to node
}

func makeCtxt(span gorgo.Span, level int, rule int, attrs interface{}) RuleCtxt {
	return RuleCtxt{
		Span:      span,
		Level:     level,
		RuleIndex: rule,
		Attrs:     attrs,
	}
}

// ---------------------------------------------------------------------------
