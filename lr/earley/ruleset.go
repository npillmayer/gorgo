package earley

import (
	"github.com/npillmayer/gorgo/lr"
)

type starts []uint64

type ruleset map[*lr.Rule]starts

//var exists = struct{}{}

func (set ruleset) add(r *lr.Rule, n uint64) ruleset {
	if set == nil {
		set = ruleset{}
	}
	if set.has(r) {
		st := set[r]
		st = append(st, n)
		set[r] = st
	} else {
		set[r] = make([]uint64, 1, 5)
		set[r][0] = n
	}
	return set
}

func (set ruleset) has(r *lr.Rule) bool {
	if set == nil || r == nil {
		return false
	}
	_, ok := set[r]
	return ok
}

func (set ruleset) contains(r *lr.Rule, n uint64) bool {
	if !set.has(r) {
		return false
	}
	//st := set[r]
	for _, m := range set[r] {
		if m == n {
			return true
		}
	}
	return false
}

func (set ruleset) delete(r *lr.Rule) {
	if set != nil {
		delete(set, r)
	}
}
