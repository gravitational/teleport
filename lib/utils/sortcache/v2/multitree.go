package sortcache

import (
	"iter"
	"sync"

	"github.com/google/btree"
)

type Direction bool

const (
	Ascending  Direction = false
	Descending Direction = true
)

type item[K, V any] struct {
	key   K
	value V
}

type MultiTreeMap[K, V any] struct {
	less    LessFunc[K]
	indices []LessFunc[K]

	mu    sync.RWMutex
	tree  *btree.BTreeG[item[K, V]]
	trees []*btree.BTreeG[item[K, V]]
}

type LessFunc[T any] = func(a, b T) bool

func NewMultiTreeMap[K, V any](less LessFunc[K], indices ...LessFunc[K]) *MultiTreeMap[K, V] {
	const bTreeDegree = 8

	freeList := btree.NewFreeListG[item[K, V]](btree.DefaultFreeListSize * (1 + len(indices)))

	tree := btree.NewWithFreeListG(
		bTreeDegree,
		func(a, b item[K, V]) bool {
			return less(a.key, b.key)
		},
		freeList,
	)

	trees := make([]*btree.BTreeG[item[K, V]], 0, len(indices))
	for _, lessIndex := range indices {
		trees = append(trees, btree.NewWithFreeListG(
			bTreeDegree,
			func(a, b item[K, V]) bool {
				if lessIndex(a.key, b.key) {
					return true
				}
				if lessIndex(b.key, a.key) {
					return false
				}
				return less(a.key, b.key)
			},
			freeList,
		))
	}

	return &MultiTreeMap[K, V]{
		less:    less,
		indices: indices,

		tree:  tree,
		trees: trees,
	}
}

func (t *MultiTreeMap[K, V]) Get(key K) (V, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	item, ok := t.tree.Get(item[K, V]{key: key})
	if !ok {
		return *new(V), false
	}
	return item.value, true
}

func (t *MultiTreeMap[K, V]) ReplaceOrInsert(key K, value V) (V, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	newItem := item[K, V]{key: key, value: value}
	oldItem, ok := t.tree.ReplaceOrInsert(newItem)
	for _, tree := range t.trees {
		if ok {
			tree.Delete(oldItem)
		}
		tree.ReplaceOrInsert(newItem)
	}
	return oldItem.value, ok
}

func (t *MultiTreeMap[K, V]) Delete(key K) (V, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	oldItem, ok := t.tree.Delete(item[K, V]{key: key})
	if !ok {
		return *new(V), false
	}
	for _, tree := range t.trees {
		tree.Delete(oldItem)
	}
	return oldItem.value, true
}

func (t *MultiTreeMap[K, V]) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()

	const addNodesToFreelistTrue = true
	t.tree.Clear(addNodesToFreelistTrue)
	for _, tree := range t.trees {
		tree.Clear(addNodesToFreelistTrue)
	}
}

func (t *MultiTreeMap[K, V]) RangeIncludedExcluded(descend Direction, greaterOrEqual, lessThan K) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		t.mu.RLock()
		defer t.mu.RUnlock()
		if descend {
			t.tree.DescendLessOrEqual(
				item[K, V]{key: lessThan},
				func(item item[K, V]) bool {
					if !t.less(item.key, lessThan) {
						return true
					}
					if t.less(item.key, greaterOrEqual) {
						return false
					}
					return yield(item.key, item.value)
				},
			)
		} else {
			t.tree.AscendRange(
				item[K, V]{key: greaterOrEqual},
				item[K, V]{key: lessThan},
				func(item item[K, V]) bool {
					return yield(item.key, item.value)
				},
			)
		}
	}
}

func (t *MultiTreeMap[K, V]) IndexRangeIncludedExcluded(index int, descend Direction, greaterOrEqual, lessThan K) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		t.mu.RLock()
		defer t.mu.RUnlock()
		if descend {
			t.trees[index].DescendLessOrEqual(
				item[K, V]{key: lessThan},
				func(item item[K, V]) bool {
					if !t.less(item.key, lessThan) {
						return true
					}
					if t.less(item.key, greaterOrEqual) {
						return false
					}
					return yield(item.key, item.value)
				},
			)
		} else {
			t.trees[index].AscendRange(
				item[K, V]{key: greaterOrEqual},
				item[K, V]{key: lessThan},
				func(item item[K, V]) bool {
					return yield(item.key, item.value)
				},
			)
		}
	}
}

func (t *MultiTreeMap[K, V]) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.tree.Len()
}
