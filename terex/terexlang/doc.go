/*
Package terexlang provides a parser for TeREx (term rewriting).

License

Governed by a 3-Clause BSD license. License file may be found in the root
folder of this module.

Copyright © 2017–2021 Norbert Pillmayer <norbert@pillmayer.com>

*/
package terexlang

import (
	"github.com/npillmayer/schuko/tracing"
)

// tracer traces with key 'gorgo.terex'
func tracer() tracing.Trace {
	return tracing.Select("gorgo.terex")
}
