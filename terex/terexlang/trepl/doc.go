/*
Package trepl/main provides an interactive command line tool (T.REPL)
for s-expressions of the TeREx language. TeREx is a Lisp-like language
with a focus on term rewriting.  T.REPL serves as a sandbox for
experiments with parse tree rewriting, useful for early stages of
parser/interpreter development.


License

Governed by a 3-Clause BSD license. License file may be found in the root
folder of this module.

Copyright © 2017–2021 Norbert Pillmayer <norbert@pillmayer.com>

*/

package main

import (
	"github.com/npillmayer/schuko/tracing"
)

// tracer traces with key 'gorgo.terex'
func tracer() tracing.Trace {
	return tracing.Select("gorgo.terex")
}
