/*
Package fp provides utilities for kind-of functional programming on
TeREx lists. It introduces sequence types, which wrap lists and other
iteratable/enumeratable types, and Lisp-like operations on them.
Sequences may be infinite, i.e. be generators.

License

Governed by a 3-Clause BSD license. License file may be found in the root
folder of this module.

Copyright © 2017–2021 Norbert Pillmayer <norbert@pillmayer.com>

*/
package fp

import "github.com/npillmayer/schuko/tracing"

// tracer traces with key 'gorgo.terex'.
func tracer() tracing.Trace {
	return tracing.Select("gorgo.terex")
}

/*
Note:
=====
The current implementation always pre-fetches the first value.
This could be optimized. It would be a problem with long-running ops in the
atom-creation, in case the value is never fetched by an output call.
For now, we will leave it this way.
*/

// IntSeq is a sequence of integers.
type IntSeq struct {
	n   int64
	seq IntGenerator
}

// Break stops generating integers.
func (iseq *IntSeq) Break() {
	iseq.seq = nil
}

// Done returns true if the sequence will not produce any further values.
func (iseq *IntSeq) Done() bool {
	return iseq.seq == nil
}

// TODO for testing only
func (iseq *IntSeq) N() int64 {
	return iseq.n
}

// First returns the first integer of a sequence.
func (iseq IntSeq) First() (int64, IntSeq) {
	//n := iseq.n
	//seq := iseq.seq()
	//seq := iseq
	return iseq.n, iseq
}

// Next returns the next integer of a sequence.
func (iseq *IntSeq) Next() int64 {
	//n := iseq.n
	if iseq.Done() {
		return iseq.n // no possibility to return in-band error
	}
	next := iseq.seq()
	iseq.n = next.n
	iseq.seq = next.seq
	return iseq.n
}

// IntGenerator is a generator for integers, returning itself wrapped in a IntSeq.
type IntGenerator func() IntSeq

// N is the infinite sequence of natural numbers 0...
func N() IntSeq {
	var n int64
	var N IntGenerator
	N = func() IntSeq {
		n++
		return IntSeq{n, N}
	}
	return IntSeq{n, N}
}

// FloatSeq is a sequence of floating point numbers.
type FloatSeq struct {
	n   float64
	seq FloatGenerator
}

// First returns the first float of a sequence.
func (rseq FloatSeq) First() (float64, FloatSeq) {
	n := rseq.n
	seq := rseq.seq()
	return n, seq
}

// Next returns the next float of a sequence.
func (rseq *FloatSeq) Next() float64 {
	n := rseq.n
	next := rseq.seq()
	rseq.n = next.n
	rseq.seq = next.seq
	return n
}

// FloatGenerator is a generator for an infinite sequence of floats, wrapping itself in
// a FloatSeq.
type FloatGenerator func() FloatSeq

// R is a generator for an infinite sequence of real numbers, given a start value
// and an increment.
func R(start float64, step float64) FloatSeq {
	x := start
	var R FloatGenerator
	R = func() FloatSeq {
		x += step
		return FloatSeq{x, R}
	}
	return FloatSeq{x, R}
}

// IntFilter is a type for filtering integers in a sequence.
type IntFilter func(n int64) bool

// LessThanN is a filter which only allows integers less than a threshold.
func LessThanN(b int64) IntFilter {
	return func(n int64) bool {
		return n < b
	}
}

// EvenN is a filter which allows even integers only.
func EvenN() IntFilter {
	return func(n int64) bool {
		return n%2 == 0
	}
}

// Where applies a filter to a sequence of integers.
func (iseq IntSeq) Where(filt IntFilter) IntSeq {
	var F IntGenerator
	n, inner := iseq.n, iseq
	F = func() IntSeq {
		n = inner.Next()
		for !inner.Done() && !filt(n) {
			n = inner.Next()
		}
		if inner.Done() {
			return IntSeq{n, nil}
		}
		return IntSeq{n, F}
	}
	return IntSeq{n, F}
}

// IntMapper is a function returning an integer from an input integer.
type IntMapper func(n int64) int64

// SquareN returns a mapper to compute the square of every input integer.
func SquareN() IntMapper {
	return func(n int64) int64 {
		return n * n
	}
}

// Map applies a mapper to all elements of an integer sequence.
func (iseq IntSeq) Map(mapper IntMapper) IntSeq {
	var F IntGenerator
	//inner := seq
	n, inner := iseq.n, iseq
	//n, inner := seq.First()
	v := mapper(n)
	F = func() IntSeq {
		//fmt.Printf("F  called, n=%d\n", n)
		n = inner.Next()
		v = mapper(n)
		//fmt.Printf("F' n=%d, v=%d\n", n, v)
		return IntSeq{v, F}
	}
	return IntSeq{v, F}
}

// Vec returns all the input integers of a sequence as a intantiated vector.
func (iseq IntSeq) Vec() []int64 {
	return nil
}
