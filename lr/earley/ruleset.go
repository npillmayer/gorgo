package earley

import (
	"github.com/npillmayer/gorgo/lr"
)

type ruleset map[*lr.Rule]struct{}

var exists = struct{}{}

func (set ruleset) add(r *lr.Rule) ruleset {
	if set == nil {
		set = ruleset{}
	}
	set[r] = exists
	return set
}

func (set ruleset) contains(r *lr.Rule) bool {
	if set == nil || r == nil {
		return false
	}
	_, ok := set[r]
	return ok
}

func (set ruleset) delete(r *lr.Rule) {
	if set != nil {
		delete(set, r)
	}
}
