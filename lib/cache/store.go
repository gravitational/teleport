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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils/sortcache"
)

type resourceStore[T any] struct {
	cache   *sortcache.SortCache[T]
	indexes map[string]func(T) string
}

func newResourceStore[T any](indexes map[string]func(T) string) *resourceStore[T] {
	return &resourceStore[T]{
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
		return t, trace.NotFound("no value for key %q in index %q", key, index)
	}

	return t, nil
}

func (s *resourceStore[T]) resources(index string, start string, stop string) iter.Seq[T] {
	return s.cache.Ascend(index, start, stop)
}
