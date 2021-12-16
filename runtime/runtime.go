/*
Package runtime implements an interpreter runtime, consisting of
scopes, memory frames and symbols (variable declarations and references).

For a thorough discussion of an interpreter's runtime environment, refer to
"Language Implementation Patterns" by Terence Parr.

Symbol Table and Scope Tree

This module implements data structures for scope trees and symbol tables
attached to them.

Memory Frames

This module implements a stack of memory frames.
Memory frames are used by an interpreter to allocate local storage
for active scopes.


----------------------------------------------------------------------

License

Governed by a 3-Clause BSD license. License file may be found in the root
folder of this module.

Copyright © 2017–2021 Norbert Pillmayer <norbert@pillmayer.com>

*/
package runtime

import (
	"github.com/npillmayer/schuko/tracing"
)

// tracer traces with key 'gorgo.runtime'.
func tracer() tracing.Trace {
	return tracing.Select("gorgo.runtime")
}

// Runtime is a type implementing a runtime environment for an interpreter
type Runtime struct {
	ScopeTree     *ScopeTree        // collect scopes
	MemFrameStack *MemoryFrameStack // runtime stack of memory frames
	UData         interface{}       // extension point
}

// NewRuntimeEnvironment constructs
// a new runtime environment, initialized. Accepts a symbol creator for
// variable declarations to be used within this runtime environment.
//
func NewRuntimeEnvironment(withDeclarations func(string) *Tag) *Runtime {
	rt := &Runtime{}
	rt.ScopeTree = new(ScopeTree) // scopes for groups and functions
	//rt.ScopeTree.PushNewScope("globals", withDeclarations)   // push global scope first
	rt.ScopeTree.PushNewScope("globals")                     // push global scope first
	rt.MemFrameStack = new(MemoryFrameStack)                 // initialize memory frame stack
	mf := rt.MemFrameStack.PushNewMemoryFrame("global", nil) // global memory
	mf.Scope = rt.ScopeTree.Globals()                        // connect the global frame with the global scope
	rt.MemFrameStack.Globals().SymbolTable = NewSymbolTable()
	//rt.MemFrameStack.Globals().SymbolTable = NewSymbolTable(withDeclarations)
	return rt
}
