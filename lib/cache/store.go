// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cache

import (
	"iter"
	"reflect"
	"sync/atomic"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils/sortcache"
)

type singletonStore[T any] struct {
	t atomic.Pointer[T]
}

func (s *singletonStore[T]) clear() error {
	s.t.Store(nil)
	return nil
}

func (s *singletonStore[T]) put(t T) error {
	old := s.t.Load()
	if !s.t.CompareAndSwap(old, &t) {
		return trace.CompareFailed("concurrent update occurred")
	}

	return nil
}

func (s *singletonStore[T]) delete(T) error {
	return s.clear()
}

func (s *singletonStore[T]) get() (T, error) {
	item := s.t.Load()
	if item == nil {
		var t T
		return t, trace.NotFound("no value for singleton of type %v", reflect.TypeOf((*T)(nil)).Elem())
	}

	return *item, nil
}

type resourceStore[T any] struct {
	filter  func(T) bool
	cache   *sortcache.SortCache[T]
	indexes map[string]func(T) string
}

func newResourceStore[T any](indexes map[string]func(T) string) *resourceStore[T] {
	return newResourceStoreWithFilter(nil, indexes)
}

func newResourceStoreWithFilter[T any](filter func(T) bool, indexes map[string]func(T) string) *resourceStore[T] {
	return &resourceStore[T]{
		filter:  filter,
		indexes: indexes,
		cache: sortcache.New(sortcache.Config[T]{
			Indexes: indexes,
		}),
	}
}

func (s *resourceStore[T]) clear() error {
	s.cache.Clear()
	return nil
}

func (s *resourceStore[T]) put(t T) error {
	if s.filter != nil && !s.filter(t) {
		return nil
	}

	s.cache.Put(t)
	return nil
}

func (s *resourceStore[T]) delete(t T) error {
	for idx, transform := range s.indexes {
		s.cache.Delete(idx, transform(t))
	}

	return nil
}

func (s *resourceStore[T]) get(index, key string) (T, error) {
	t, ok := s.cache.Get(index, key)
	if !ok {
		return t, trace.NotFound("no value for resource of type %v", reflect.TypeOf((*T)(nil)).Elem())
	}

	return t, nil
}

func (s *resourceStore[T]) iterate(index string, start string, stop string) iter.Seq[T] {
	return s.cache.Ascend(index, start, stop)
}
