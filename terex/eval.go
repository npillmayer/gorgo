package terex

import (
	"fmt"

	"github.com/npillmayer/schuko/tracing"
)

/*
License

Governed by a 3-Clause BSD license. License file may be found in the root
folder of this module.

Copyright © 2017–2021 Norbert Pillmayer <norbert@pillmayer.com>
*/

func (env *Environment) Eval(list *GCons) *GCons {
	r := Eval(Elem(list), env)
	//T().Errorf("############# Eval => %s", r.String())
	return r.AsList()
}

func Eval(el Element, env *Environment) Element {
	//T().Errorf("############# Eval => %s", el.String())
	if el.IsAtom() {
		// if el.AsAtom().Type() == ConsType {
		// 	T().Errorf("eval of sublist %v", el.AsList().ListString())
		// 	sublist := el.AsAtom().Data.(*GCons)
		// 	T().Errorf("eval of sublist %v", sublist.ListString())
		// 	mapped := evalList(sublist, env)
		// 	T().Errorf("        sublist %v", mapped.String())
		// 	//return mapped
		// 	tee := Cons(Atomize(mapped.AsList()), nil)
		// 	T().Errorf("   reconstucted %v", tee.String())
		// 	return Elem(tee)
		// }
		tracer().Debugf("eval of atom %v", el.AsAtom())
		return evalAtom(el.AsAtom(), env)
	}
	list := el.AsList()
	tracer().Debugf("eval of list %v", list.ListString())
	l := evalList(list, env)
	return l
}

func evalList(list *GCons, env *Environment) Element {
	if list == nil || list.Car == NilAtom {
		return Elem(list)
	}
	car, err := resolve(list.Car, env, true)
	if err != nil {
		return Elem(ErrorAtom(err.Error()))
	}
	if car.Type() != OperatorType { // Resolver gave us this
		verblist := list.Map(EvalAtom, env) // ⇒ accept it
		return Elem(verblist)               // and return non-operated list
	}
	tracer().Debugf("-------- op=%s -----------", car.String())
	operator := car.AsAtom().Data.(Operator)
	//T().Debugf("OP = %s", operator)
	//args := list.Cdr.Map(Eval, env)
	args := Elem(list.Cdr)
	tracer().Debugf("--- %s.call%v", operator.String(), args.String())
	ev := operator.Call(args, env)
	tracer().Debugf("list eval result:")
	ev.Dump(tracing.LevelDebug)
	tracer().Debugf("--------------------------")
	return ev
}

func EvalAtom(atom Element, env *Environment) Element {
	return evalAtom(atom.AsAtom(), env)
}

func evalAtom(atom Atom, env *Environment) Element {
	resolved, _ := resolve(atom, env, false)
	tracer().Debugf("%s resolved -> %v", atom, resolved)
	if resolved.IsNil() || atom.typ == VarType || resolved.Type() != ConsType {
		return resolved
	}
	// sublist
	//T().Errorf("--------> sublist --------")
	sublist := evalList(resolved.Sublist().AsList(), env)
	//T().Errorf("--------< sublist --------")
	//return Elem(Cons(sublist.AsAtom(), nil)) // wrap again in cons
	return Elem(sublist) //, nil // wrap again in cons?
}

func resolve(atom Atom, env *Environment, asOp bool) (Element, error) {
	if env.Resolver == nil {
		return DefaultSymbolResolver{}.Resolve(atom, env, asOp)
	}
	return env.Resolver.Resolve(atom, env, asOp)
}

type DefaultSymbolResolver struct {
	// TODO options
}

func (dsr DefaultSymbolResolver) Resolve(atom Atom, env *Environment, asOp bool) (Element, error) {
	//T().Errorf("### Default Symbol Resolver")
	if atom.Type() == OperatorType {
		return Elem(atom), nil // shortcut, not resolved in env
	}
	if atom.Type() == VarType {
		atomSym := atom.Data.(*Symbol)
		sym := env.FindSymbol(atomSym.Name, true)
		if sym == nil {
			tracer().Errorf("Unable to resolve symbol '%s' in environment", atomSym.Name)
			err := fmt.Errorf("unable to resolve symbol '%s' in environment", atomSym.Name)
			env.Error(err)
			return Elem(atom), err
		}
		value := sym.Value
		if asOp && sym.ValueType() != OperatorType {
			env.lastError = fmt.Errorf("Symbol '%s' cannot be resolved as operator", sym.Name)
			tracer().Errorf("Symbol '%s' cannot be resolved as operator", sym.Name)
			err := fmt.Errorf("Symbol '%s' cannot be resolved as operator", sym.Name)
			env.Error(err)
			return Elem(nil), err
		}
		return value, nil
	}
	if asOp { // atom is neither a symbol nor an operator, but operator expected
		env.lastError = fmt.Errorf("Atom '%s' cannot be cast to operator ", atom)
		tracer().Errorf("Atom '%s' cannot be cast to operator ", atom)
		err := fmt.Errorf("Atom '%s' cannot be cast to operator ", atom)
		env.Error(err)
		return Elem(nil), err
	}
	//T().Errorf("Returning plain atom %v", Elem(atom))
	return Elem(atom), nil
}

var _ SymbolResolver = &DefaultSymbolResolver{}

// --- Quote -----------------------------------------------------------------

// Quote traverses an s-expr and returns it as pure list/tree data.
// It gets rid of #list:op and #quote:op nodes.
//
// If the environment contains a symbol's value, quoting will replace the symbol
// by its value. For example, if the s-expr contains a symbol 'str' with a value
// of "this is a string", the resulting data structure will contain the string,
// not the name of the symbol. If you do not have use for this kind of substitution,
// simply call Quote(…) for the global environment.
//
/*
func (env *Environment) Quote(list *GCons) *GCons {
	r := env.quote(Elem(list))
	T().Debugf("Quote => %s", r)
	return r.AsList()
}

func (env *Environment) quote(el Element) Element {
	if el.IsAtom() {
		//T().Errorf("quote of atom %v, type=%s", el, el.AsAtom().Type().String())
		if el.AsAtom().Type() == ConsType {
			//T().Infof("---- atom/list --------------")
			//T().Errorf("QUOTE el.Cdr=%s", el.AsAtom().Data.(*GCons).ListString())
			sublist := el.AsAtom().Data.(*GCons)
			mapped := env.quoteList(sublist).AsList()
			//T().Infof("MAPPED = %s", mapped.ListString())
			//T().Infof("-----------------------------")
			return Elem(mapped)
			//return Elem(Cons(Atomize(mapped), nil))
		}
		return el
	}
	//T().Infof("==== as list ================")
	list := el.AsList()
	l := env.quoteList(list)
	//T().Infof("=============================")
	return l
	//return env.quoteList(list)
	// T().Errorf("quote of list %v", list.ListString())
	// if list == nil || list.Car == NilAtom {
	// 	return Elem(list)
	// }
	// op := list.Car
	// if op.typ != OperatorType {
	// 	verblist := list.Map(env.quote)
	// 	return Elem(verblist)
	// }
	// T().Errorf("quote-OP = %s", op)
	// operator := op.Data.(Operator)
	// args := list.Cdr.Map(env.quote)
	// return operator.Quote(Elem(args), env)
}

func (env *Environment) quoteAtom(atom Atom) Element {
	return Elem(atom) // TODO
}

func (env *Environment) quoteList(list *GCons) Element {
	//
	//T().Errorf("quote of list %v", list.ListString())
	if list == nil || list.Car == NilAtom {
		return Elem(list)
	}
	op := list.Car
	if op.typ != OperatorType {
		T().Infof("------- VerbList -----------------------------")
		T().Infof("   > Map(quote(...))")
		verblist := list.Map(env.quote)
		T().Infof("----------------------------------------------")
		return Elem(verblist)
	}
	//T().Errorf("quote-OP = %s", op)
	operator := op.Data.(Operator)
	args := list.Cdr.Map(env.quote)
	//args := list.Cdr
	T().Infof("-------- Op = %s -----------------------------", operator.String())
	T().Infof("     args=%s", args.ListString())
	T().Infof("   > quote(args...)")
	quoted := operator.Quote(Elem(args), env)
	T().Infof("     quoted=%s", quoted.String())
	T().Infof("----------------------------------------------")
	return quoted
}
*/
