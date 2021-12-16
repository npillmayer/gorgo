/*
Package sparse implements a simple type for sparse integer matrices.
It is mainly used for parser tables (GOTO-table and ACTION-table).
Every entry in the table is either a single int32 or a pair (int32,int32).

This implementation uses the COO algorithm (a.k.a. triplet-encoding).

   https://medium.com/@jmaxg3/101-ways-to-store-a-sparse-matrix-c7f2bf15a229
   https://www.coin-or.org/Ipopt/documentation/node38.html


License

Governed by a 3-Clause BSD license. License file may be found in the root
folder of this module.

Copyright © 2017–2021 Norbert Pillmayer <norbert@pillmayer.com>

*/
package sparse

import (
	"fmt"
)

// IntMatrix is a type for a spare matrix of integer values. Construct with
//
//     M := NewIntMatrix(10, 10, -1)  // last parameter is M's null-value
//
// Now
//
//     M.Set(2, 3, 4711)              // set a value
//     v := M.Value(2, 3)             // returns 4711
//     M.Add(2, 3, 123)               // add a second value
//     cnt := M.ValueCount()          // still returns 1 (one position set)
//     v = M.Value(10, 10)            // returns -1, i.e. the null-value
//
// Values cannot be deleted, but may be overwritten with the null-value. Space for
// null-values is not re-claimed.
type IntMatrix struct {
	values  []triplet
	rowcnt  int
	colcnt  int
	nullval int32
}

// Triplet values to store
type triplet struct {
	row, col int
	value    intPair
}

// NewIntMatrix creates a new matrix for int, size m x n. The 3rd argument is a null-value,
// indicating empty entries (use DefaultNullValue if you haven't any specific
// requirements).
func NewIntMatrix(m, n int, nullValue int32) *IntMatrix {
	return &IntMatrix{
		values:  []triplet{},
		rowcnt:  m,
		colcnt:  n,
		nullval: nullValue,
	}
}

// DefaultNullValue is the default empty-value for matrices (min int32).
const DefaultNullValue = -2147483648

// M returns the row count.
func (m *IntMatrix) M() int {
	return m.rowcnt
}

// N returns the column count.
func (m *IntMatrix) N() int {
	return m.colcnt
}

// NullValue returns this matrix' null value
func (m *IntMatrix) NullValue() int32 {
	return m.nullval
}

// ValueCount returns the number of values in the matrix.
func (m *IntMatrix) ValueCount() int {
	return len(m.values)
}

// Value returns the primary value at position (i,j), or NullValue
func (m *IntMatrix) Value(i, j int) int32 {
	for _, t := range m.values {
		if !t.storedLeftOf(i, j) { // have skipped all lesser indices
			if t.storedAt(i, j) {
				return t.value.a
			}
			break
		}
	}
	return m.nullval
}

// Values returns the pair of values at position (i,j), or (NullValue, NullValue)
func (m *IntMatrix) Values(i, j int) (int32, int32) {
	for _, t := range m.values {
		if !t.storedLeftOf(i, j) { // have skipped all lesser indices
			if t.storedAt(i, j) {
				return t.value.a, t.value.b
			}
			break
		}
	}
	return m.nullval, m.nullval
}

// Set a value in the matrix at position (i,j).
func (m *IntMatrix) Set(i, j int, value int32) *IntMatrix {
	return m.setOrAdd(i, j, value, false)
}

// Add a value in the matrix at position (i,j).
func (m *IntMatrix) Add(i, j int, value int32) *IntMatrix {
	return m.setOrAdd(i, j, value, true)
}

func (m *IntMatrix) setOrAdd(i, j int, value int32, doAdd bool) *IntMatrix {
	at := 0 // will be position of new value
	for k, t := range m.values {
		if !t.storedLeftOf(i, j) { // have skipped all lesser indices
			if t.storedAt(i, j) { // value already present
				if doAdd {
					v := m.values[k].value
					m.values[k].value = addIntValue(v, value, m.nullval) // add new value
				} else {
					m.values[k].value = newIntPair(value, m.nullval) // set new value
				}
				return m // and done
			}
			break // no old value present
		}
		at++
	}
	tnew := triplet{row: i, col: j, value: newIntPair(value, m.nullval)}
	// the following 3 lines have to work for k being the right edge of v or not
	m.values = append(m.values, tnew)    // make room
	copy(m.values[at+1:], m.values[at:]) // copy remainder values one index to right
	m.values[at] = tnew                  // if not append-case: insert new triplet
	return m
}

func addIntValue(v intPair, n int32, nullval int32) intPair {
	if v.a == nullval {
		v.a = n
	} else if v.b == nullval {
		v.b = n
	} else {
		// entry is full. what to do?
		v.b = n // overwrite second
	}
	return v
}

func (t *triplet) storedLeftOf(i, j int) bool {
	return t.row < i || t.row == i && t.col < j
}

func (t *triplet) storedAt(i, j int) bool {
	return (t.row == i && t.col == j)
}

// we will store 2 int32 in one position
type intPair struct {
	a int32
	b int32
}

func (pr intPair) String() string {
	return fmt.Sprintf("[%d,%d]", pr.a, pr.b)
}

func newIntPair(a, b int32) intPair {
	return intPair{a, b}
}
