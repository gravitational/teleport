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
	"sync/atomic"

	"github.com/gravitational/trace"
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
		return t, trace.NotFound("no items present")
	}

	return *item, nil
}
