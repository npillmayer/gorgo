package runtime

import (
	"fmt"
)

// This module implements a stack of memory frames.
// Memory frames are used by an interpreter to allocate local storage
// for active scopes.

// DynamicMemoryFrame is a memory frame, representing a piece of memory for a scope.
type DynamicMemoryFrame struct {
	Name        string
	Scope       *Scope
	SymbolTable *SymbolTable
	Parent      *DynamicMemoryFrame
}

// NewDynamicMemoryFrame creates a new memory frame.
func NewDynamicMemoryFrame(nm string, scope *Scope) *DynamicMemoryFrame {
	mf := &DynamicMemoryFrame{
		Name:  nm,
		Scope: scope,
	}
	return mf
}

func (mf *DynamicMemoryFrame) String() string {
	return fmt.Sprintf("<mem %s -> %v>", mf.Name, mf.Scope)
}

// IsRoot is a predicate: Is this a root frame?
func (mf *DynamicMemoryFrame) IsRoot() bool {
	return (mf.Parent == nil)
}

// ---------------------------------------------------------------------------

// MemoryFrameStack is a (call-)stack of memory frames.
type MemoryFrameStack struct {
	memoryFrameBase *DynamicMemoryFrame
	memoryFrameTOS  *DynamicMemoryFrame
}

// Current gets the current memory frame of a stack (TOS).
func (mfst *MemoryFrameStack) Current() *DynamicMemoryFrame {
	if mfst.memoryFrameTOS == nil {
		panic("attempt to access memory frame from empty stack")
	}
	return mfst.memoryFrameTOS
}

// Globals gets the outermost memory frame, containing global symbols.
func (mfst *MemoryFrameStack) Globals() *DynamicMemoryFrame {
	if mfst.memoryFrameBase == nil {
		panic("attempt to access global memory frame from empty stack")
	}
	return mfst.memoryFrameBase
}

// PushNewMemoryFrame pushes a new memory frame as TOS.
// A frame is constructed, having the recent TOS as its
// parent. If the new frame is not the bottommost frame, it will copy the
// symbol-creator from the parent frame. Otherwise callers will have to provide
// a scope (if needed) in a separate step.
//
func (mfst *MemoryFrameStack) PushNewMemoryFrame(nm string, scope *Scope) *DynamicMemoryFrame {
	mfp := mfst.memoryFrameTOS
	newmf := NewDynamicMemoryFrame(nm, scope)
	newmf.Parent = mfp
	if mfp == nil { // the new frame is the global frame
		mfst.memoryFrameBase = newmf // make new mf anchor
	} else {
		//symcreator := mfp.SymbolTable.GetSymbolCreator()
		symtab := NewSymbolTable()
		newmf.SymbolTable = symtab
	}
	mfst.memoryFrameTOS = newmf // new frame now TOS
	T().P("mem", newmf.Name).Debugf("pushing new memory frame")
	return newmf
}

// PopMemoryFrame pops the top-most memory frame. Returns the popped frame.
func (mfst *MemoryFrameStack) PopMemoryFrame() *DynamicMemoryFrame {
	if mfst.memoryFrameTOS == nil {
		panic("attempt to pop memory frame from empty call stack")
	}
	mf := mfst.memoryFrameTOS
	T().Debugf("popping memory frame [%s]", mf.Name)
	mfst.memoryFrameTOS = mfst.memoryFrameTOS.Parent
	return mf
}

// FindMemoryFrameForScope finds the top-most memory frame pointing to scope.
func (mfst *MemoryFrameStack) FindMemoryFrameForScope(scope *Scope) *DynamicMemoryFrame {
	mf := mfst.memoryFrameTOS
	for mf != nil {
		if mf.Scope == scope {
			return mf
		}
		mf = mf.Parent
	}
	return nil
}
