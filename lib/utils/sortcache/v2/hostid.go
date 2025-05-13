package sortcache

import "iter"

type hostIDKey struct {
	hostID string
	name   string
}

type HostIDTree[T any] struct {
	t *MultiTreeMap[hostIDKey, T]

	getKey func(T) (hostID, name string)
}

func NewHostIDTree[T any](getKey func(T) (hostID, name string)) *HostIDTree[T] {
	return &HostIDTree[T]{
		t: NewMultiTreeMap[hostIDKey, T](
			func(a, b hostIDKey) bool {
				if a.hostID < b.hostID {
					return true
				}
				if a.hostID > b.hostID {
					return false
				}
				return a.name < b.name
			},
			func(a, b hostIDKey) bool {
				return a.name < b.name
			},
		),
		getKey: getKey,
	}
}

func (t *HostIDTree[T]) Get(hostID, name string) (T, bool) {
	return t.t.Get(hostIDKey{
		hostID: hostID,
		name:   name,
	})
}

func (t *HostIDTree[T]) ReplaceOrInsert(value T) (T, bool) {
	hostID, name := t.getKey(value)
	return t.t.ReplaceOrInsert(hostIDKey{
		hostID: hostID,
		name:   name,
	}, value)
}

func (t *HostIDTree[T]) Delete(hostID, name string) (T, bool) {
	return t.t.Delete(hostIDKey{
		hostID: hostID,
		name:   name,
	})
}

func (t *HostIDTree[T]) RangeHostID(descend Direction, hostID string) iter.Seq2[string, T] {
	return func(yield func(string, T) bool) {
		for k, v := range t.t.RangeIncludedExcluded(
			descend,
			hostIDKey{hostID: hostID},
			hostIDKey{hostID: hostID + "\x00"},
		) {
			if !yield(k.name, v) {
				return
			}
		}
	}
}

func (t *HostIDTree[T]) RangeName(descend Direction, name string) iter.Seq2[string, T] {
	return func(yield func(string, T) bool) {
		for k, v := range t.t.IndexRangeIncludedExcluded(
			0,
			descend,
			hostIDKey{name: name},
			hostIDKey{name: name + "\x00"},
		) {
			if !yield(k.hostID, v) {
				return
			}
		}
	}
}
