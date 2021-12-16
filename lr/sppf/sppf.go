/*
Package sppf implements a "Shared Packed Parse Forest".

A packed parse forest re-uses existing parse tree nodes between different
parse trees. For a conventional non-ambiguous parse, a parse forest degrades
to a single tree. Ambiguous grammars, on the other hand, may result in parse
runs where more than one parse tree is created. To save space these parse
trees will share common nodes.


License

Governed by a 3-Clause BSD license. License file may be found in the root
folder of this module.

Copyright © 2017–2021 Norbert Pillmayer <norbert@pillmayer.com>

*/
package sppf

import (
	"github.com/npillmayer/schuko/tracing"
)

// tracer traces with key 'gorgo.lr'.
func tracer() tracing.Trace {
	return tracing.Select("gorgo.lr")
}
