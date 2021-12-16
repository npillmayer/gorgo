package termr

/*
License

Governed by a 3-Clause BSD license. License file may be found in the root
folder of this module.

Copyright © 2017–2021 Norbert Pillmayer <norbert@pillmayer.com>

*/

import (
	"github.com/npillmayer/gorgo/terex"
	"github.com/npillmayer/gorgo/terex/fp"
)

// Rewriter is a function
//
//     list × env ↦ list
//
// i.e., a term rewriting function.
type Rewriter func(l *terex.GCons, env *terex.Environment) terex.Element

// RewriteRule is a type representing a rule for term rewriting.
// It contains a pattern and a rewriting-function. The pattern will be applied
// to nodes in an AST, and if it matches the rewriter will be called on the redex.
type RewriteRule struct {
	Pattern *terex.GCons
	Rewrite Rewriter
}

// ---------------------------------------------------------------------------

// Anything is a pattern matching any s-expr.
func Anything() *terex.GCons {
	return terex.Cons(terex.Atomize(terex.ConsType), nil)
}

// AnySymbol is a pattern matching any single symbol or token.
func AnySymbol() *terex.GCons {
	return terex.Cons(terex.Atomize(terex.AnyType), nil)
}

// Match is a node mapper which checks incoming tree nodes for a match against
// a pattern. If the match succeeds, a new environment is created, containing
// symbols for the operator and for matches arguments. The newly created environment
// is packed onto the tree node for later stages of the call sequence.
func Match(pattern *terex.GCons, env *terex.Environment) fp.NodeMapper {
	return func(node fp.TreeNode) fp.TreeNode {
		if node.Node == nil {
			panic("nil node as mapper input")
		}
		if node.Node.Car.Type() == terex.OperatorType {
			op := node.Node.Car.Data.(terex.Operator)
			env = EnvironmentForOperator(op, env)
		}
		if pattern.Match(node.Node, env) {
			node.UData = env
		}
		return node
	}
}

// RewriteWith applies a term rewrite to an incoming tree node.
func RewriteWith(rewrite Rewriter) fp.NodeMapper {
	return func(node fp.TreeNode) fp.TreeNode {
		if node.Node == nil {
			panic("nil node as mapper input")
		}
		if node.UData == nil {
			return node
		}
		env, ok := node.UData.(*terex.Environment)
		if !ok {
			return node
		}
		newNode := rewrite(node.Node, env).AsList()
		return node.ReplaceWith(newNode)
	}
}

// ---------------------------------------------------------------------------

// EnvironmentForOperator creates an environment for an operator. The intention is for
// op to be the head of a TeREx list. The environment will have the operatore pre-stored
// as a symbol.
//
// If the parent environment is not given, it will be set to the global environment.
//
// Will return nil, if op is nil, a new environment otherwise.
func EnvironmentForOperator(op terex.Operator, parent *terex.Environment) *terex.Environment {
	if op == nil {
		return nil
	}
	if parent == nil {
		parent = terex.GlobalEnvironment
	}
	env := terex.NewEnvironment("#"+op.String(), parent)
	sym := env.Intern(op.String(), false)
	sym.Value = terex.Elem(op)
	return env
}
