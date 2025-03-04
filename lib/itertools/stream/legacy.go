/*
Copyright 2025 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
