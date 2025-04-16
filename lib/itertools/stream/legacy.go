/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package stream

import (
	"iter"

	legacy "github.com/gravitational/teleport/api/internalutils/stream"
)

type legacyAdapter[T any] struct {
	next func() (T, error, bool)
	stop func()
	item T
	err  error
}

func (stream *legacyAdapter[T]) Next() bool {
	var ok bool
	stream.item, stream.err, ok = stream.next()
	return ok && stream.err == nil
}

func (stream *legacyAdapter[T]) Item() T {
	return stream.item
}

func (stream *legacyAdapter[T]) Done() error {
	stream.stop()
	return stream.err
}

// IntoLegacy converts a standard stream into a legacy pull-based stream.
func IntoLegacy[T any](stream Stream[T]) legacy.Stream[T] {
	next, stop := iter.Pull2(stream)
	return &legacyAdapter[T]{
		next: next,
		stop: stop,
	}
}

// FromLegacy converts a legacy pull-based stream into a standard stream.
func FromLegacy[T any](stream legacy.Stream[T]) Stream[T] {
	return func(yield func(T, error) bool) {
		for stream.Next() {
			if !yield(stream.Item(), nil) {
				stream.Done()
				return
			}
		}

		if err := stream.Done(); err != nil {
			var zero T
			yield(zero, err)
		}
	}
}
