package runtime

import (
	"testing"
)

func TestNewSymTab(t *testing.T) {
	symtab := NewSymbolTable()
	if symtab == nil {
		t.Error("no symbol table created")
	}
}

func TestNewSymbol(t *testing.T) {
	symtab := NewSymbolTable()
	sym, _ := symtab.DefineTag("new-sym")
	if sym == nil {
		t.Error("no symbol created for table")
	}
	sym.UData = 5
	if sym.UData != 5 {
		t.Errorf("UData does not work")
	}
}

func TestTwoSymbolsDistinctId(t *testing.T) {
	symtab := NewSymbolTable()
	sym1, _ := symtab.DefineTag("new-sym1")
	sym2, _ := symtab.DefineTag("new-sym2")
	if sym1 == sym2 {
		t.Error("2 symbols with equal name")
	}
}

func TestResolveTag(t *testing.T) {
	symtab := NewSymbolTable()
	sym, _ := symtab.DefineTag("new-sym")
	if s := symtab.ResolveTag(sym.Name); s == nil {
		t.Error("cannot find stored symbol in table")
	}
}

func TestResolveOrDefineTag(t *testing.T) {
	symtab := NewSymbolTable()
	sym, _ := symtab.DefineTag("new-sym")
	if _, found := symtab.ResolveOrDefineTag(sym.Name); !found {
		t.Error("cannot find stored symbol in table")
	}
}

func TestDefineTag(t *testing.T) {
	symtab := NewSymbolTable()
	sym, _ := symtab.DefineTag("new-sym")
	if _, old := symtab.DefineTag("new-sym"); old != sym {
		t.Error("symbol should have been replaced")
	}
}

func TestScopeUpsearch(t *testing.T) {
	scopep := NewScope("parent", nil)
	scope := NewScope("current", scopep)
	scopep.DefineTag("new-sym")
	if sym, _ := scope.ResolveTag("new-sym"); sym != nil {
		t.Logf("found symbol '%s' in parent scope, ok\n", sym.Name)
	} else {
		t.Fail()
	}
}

func TestAddChild(t *testing.T) {
	sym := NewTag("new-sym")
	ch1 := NewTag("child-sym1")
	ch2 := NewTag("child-sym2")
	sym.AppendChild(ch1)
	sym.AppendChild(ch2)
	if sym.Children.Name != "child-sym1" {
		t.Fail()
	}
}
