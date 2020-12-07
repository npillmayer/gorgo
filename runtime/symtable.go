package runtime

import (
	"fmt"
)

// Symbol table for variables. Symbol tables are attached to scopes.
// Scopes are organized in a tree.
//

// --- Tags -------------------------------------------------------

// Every symbol has a serial ID.
var serialID int = 1 // must not start with 0 !

// Tag is the symbols type to be stored into symbol tables. It may be a
// little surprising this type is not called 'Symbol', but I prefer the
// name 'tag' because it is less confusing when dealing with parser
// generators and grammars: Grammars consist of symbols (within rules), too.
// Thus, symbols are used in the scope of the grammar, tags are used during
// runtime (of the client program).
//
type Tag struct {
	Name     string
	Id       int32
	typ      int8
	Sibling  *Tag // Some varibles form small trees
	Children *Tag
}

// Pre-defined tag types, if you want to use them.
const (
	Undefined int = iota
	IntegerType
	FloatType
	StringType
	ColorType
	PairType
	PathType
	PenType
)

// NewTag creates a new tag, with a new ID.
func NewTag(nm string, typ int) Symbol {
	serialID += 1
	var tag = &Tag{
		Name: nm,
		Id:   serialID,
		typ:  typ,
	}
	return tag
}

// String is a debug Stringer for symbols.
func (s *Tag) String() string {
	return fmt.Sprintf("<tag '%s'[%d]:%s>", s.Name, s.Id, s.typ)
}

// Type gets the tag's type.
func (s *Tag) Type() int {
	return s.typ
}

// AppendChild appends a rightmost child tag.
// Returns the tag (for chaining).
func (s *Tag) AppendChild(ch *Tag) *Tag {
	//T().Debug("---> append child %v to %v\n", ch, s)
	if s.Children == nil {
		T().Debugf("appending first child: %s", ch)
		s.Children = ch
	} else {
		next := s.Children
		for ; next.GetSibling() != nil; next = next.GetSibling() {
			// do nothing
		}
		next.Sibling = ch
		T().Debugf("appending child: %s\n", next.Sibling)
	}
	return s
}

// === Symbol Tables =========================================================

// Symbol tables to store tags (map-like semantics).
type SymbolTable struct {
	Table     map[string]*Tag
	createTag func(string) *Tag
}

// Create an empty symbol table.
//
func NewSymbolTable() *SymbolTable {
	var symtab = SymbolTable{
		Table:     make(map[string]*Tag),
		createTag: NewTag,
	}
	return &symtab
}

// ResolveTag checks for a tag in the symbol table.
// Returns a tag or nil.
//
func (t *SymbolTable) ResolveTag(tagname string) *Tag {
	//t.Lock()
	tag := t.Table[tagname] // get tag by name
	//t.Unlock()
	return tag
}

// ResolveOrDefineTag finds
// a tag in the table, inserts a new one if not found.
// Creates non-existent tags on the fly.
// Returns the tag and a flag, signalling wether the tag
// has already been present.
//
func (t *SymbolTable) ResolveOrDefineTag(tagname string) (Symbol, bool) {
	if len(tagname) == 0 {
		return nil, false
	}
	found := true
	tag := t.ResolveTag(tagname)
	if tag == nil { // if not already there, insert it
		tag, _ = t.DefineTag(tagname)
		found = false
	}
	return tag, found
}

// DefineTag creates a new tag to store into the symbol table.
// The tag's name may not be empty
// Overwrites existing tag with this name, if any.
// Returns the new tag and the previously stored tag (or nil).
//
func (t *SymbolTable) DefineTag(tagname string) (*Tag, *Tag) {
	if len(tagname) == 0 {
		return nil, nil
	}
	tag := t.createTag(tagname)
	old := t.InsertTag(sym)
	return tag, old
}

// Insert a pre-created symbol.
func (t *SymbolTable) InsertTag(tag *Tag) *Tag {
	old := t.ResolveTag(tag.Name)
	t.Table[tag.Name] = sym
	return old
}

// Count the tags in a symbol table.
func (t *SymbolTable) Size() int {
	return len(t.Table)
}

// Iterate over each tags in the table, executing a mapper function.
func (t *SymbolTable) Each(mapper func(string, *Tag)) {
	for k, v := range t.Table {
		mapper(k, v)
	}
}

// === Scopes ================================================================

// A named scope, which may contain symbol definitions. Scopes link back to a
// parent scope, forming a tree.
type Scope struct {
	Name   string
	Parent *Scope
	symtab *SymbolTable
}

// NewScope creates a new scope.
func NewScope(nm string, parent *Scope, symcreator func(string) Symbol) *Scope {
	sc := &Scope{
		Name:   nm,
		Parent: parent,
		symtab: NewSymbolTable(symcreator),
	}
	return sc
}

// Prettyfied Stringer.
func (s *Scope) String() string {
	return fmt.Sprintf("<scope %s>", s.Name)
}

/* Return the symbol table of a scope.
 */
func (s *Scope) Tags() *SymbolTable {
	return s.symtab
}

// Define a tag in the scope. Returns the new tag and the previously
// stored tag under this key, if any.
//
func (s *Scope) DefineTag(tagname string) (Symbol, Symbol) {
	return s.symtab.DefineTag(tagname)
}

// Find a tag. Returns the tag (or nil) and a scope. The scope is
// the scope (of a scope-tree-path) the tag was found in.
//
func (s *Scope) ResolveTag(tagname string) (Symbol, *Scope) {
	tag := s.symtab.ResolveTag(tagname)
	if tag != nil {
		return tag, s
	}
	for s.Parent != nil {
		s = s.Parent
		tag, _ = s.ResolveTag(tagname)
		if tag != nil {
			return tag, s
		}
	}
	return tag, nil
}

// ---------------------------------------------------------------------------

// ScopeTree can be treated as a stack during static analysis, thus
// building a tree from scopes which are pushed an popped to/from the stack.
//
type ScopeTree struct {
	ScopeBase *Scope
	ScopeTOS  *Scope
}

// Current gets the current scope of a stack (TOS).
func (scst *ScopeTree) Current() *Scope {
	if scst.ScopeTOS == nil {
		panic("attempt to access scope from empty stack")
	}
	return scst.ScopeTOS
}

// Globals gets the outermost scope, containing global symbols.
func (scst *ScopeTree) Globals() *Scope {
	if scst.ScopeBase == nil {
		panic("attempt to access global scope from empty stack")
	}
	return scst.ScopeBase
}

// PushNewScope pushes a scope onto the stack of scopes. A scope is constructed, including a symbol table
// for variable declarations.
func (scst *ScopeTree) PushNewScope(nm string, symcreator func(string) Symbol) *Scope {
	scp := scst.ScopeTOS
	newsc := NewScope(nm, scp, symcreator)
	if scp == nil { // the new scope is the global scope
		scst.ScopeBase = newsc // make new scope anchor
	}
	scst.ScopeTOS = newsc // new scope now TOS
	T().P("scope", newsc.Name).Debugf("pushing new scope")
	return newsc
}

// PopScope pops the top-most (recent) scope.
func (scst *ScopeTree) PopScope() *Scope {
	if scst.ScopeTOS == nil {
		panic("attempt to pop scope from empty stack")
	}
	sc := scst.ScopeTOS
	T().Debugf("popping scope [%s]", sc.Name)
	scst.ScopeTOS = scst.ScopeTOS.Parent
	return sc
}
