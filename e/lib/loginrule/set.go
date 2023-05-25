package loginrule

import (
	"golang.org/x/exp/maps"
)

type set map[string]struct{}

func newSet(values ...string) set {
	s := make(set, len(values))
	for _, value := range values {
		s[value] = struct{}{}
	}
	return s
}

func (s set) add(values ...string) set {
	out := s.clone()
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

func (s set) contains(str string) bool {
	_, ok := s[str]
	return ok
}

func (s set) remove(values ...string) any {
	out := s.clone()
	for _, value := range values {
		delete(out, value)
	}
	return out
}

func (s set) transform(f func(string) string) set {
	out := make(set, len(s))
	for str := range s {
		out[f(str)] = struct{}{}
	}
	return out
}

func (s set) clone() set {
	return maps.Clone(s)
}

func (s set) items() []string {
	return maps.Keys(s)
}

func union(sets ...set) set {
	result := make(set)
	for _, s := range sets {
		for v := range s {
			result[v] = struct{}{}
		}
	}
	return result
}
