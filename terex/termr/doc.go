/*
Package termr implements tools for term rewriting and construction
of abstract syntax trees. It is based on TeREx, a framework for
homogenous trees.


License

Governed by a 3-Clause BSD license. License file may be found in the root
folder of this module.

Copyright © 2017–2021 Norbert Pillmayer <norbert@pillmayer.com>

*/
package termr

import (
	"github.com/npillmayer/schuko/tracing"
)

// tracer traces with key 'gorgo.terex'.
func tracer() tracing.Trace {
	return tracing.Select("gorgo.terex")
}
