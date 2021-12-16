package earley

import (
	"bytes"

	"github.com/npillmayer/gorgo/lr"
	"github.com/npillmayer/gorgo/lr/iteratable"
)

func dumpState(states []*iteratable.Set, stateno uint64) {
	tracer().Debugf("--- State %04d ------------------------------------", stateno)
	S := states[stateno]
	n := 1
	S.IterateOnce()
	for S.Next() {
		item := S.Item().(lr.Item)
		tracer().Debugf("[%2d] %s", n, item)
		n++
	}
}

func itemSetString(S *iteratable.Set) string {
	var b bytes.Buffer
	b.WriteString("{")
	S.IterateOnce()
	first := true
	for S.Next() {
		item := S.Item().(lr.Item)
		if first {
			b.WriteString(" ")
			first = false
		} else {
			b.WriteString(", ")
		}
		b.WriteString(item.String())
	}
	b.WriteString(" }")
	return b.String()
}
