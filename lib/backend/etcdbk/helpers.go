/*
Copyright 2023 Gravitational, Inc.

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

package etcdbk

import "sync/atomic"

// roundRobin is a helper for distributing load across multiple resources in a round-robin
// fashion (used to implement simple client pooling).
type roundRobin[T any] struct {
	ct    *atomic.Uint64
	items []T
}

func newRoundRobin[T any](items []T) roundRobin[T] {
	return roundRobin[T]{
		ct:    new(atomic.Uint64),
		items: items,
	}
}

func (r roundRobin[T]) Next() T {
	n := r.ct.Add(1) - 1
	l := uint64(len(r.items))
	return r.items[int(n%l)]
}
