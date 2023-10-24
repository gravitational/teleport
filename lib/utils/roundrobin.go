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

package utils

import "sync/atomic"

// RoundRobin is a helper for distributing load across multiple resources in a round-robin
// fashion.
type RoundRobin[T any] struct {
	ct    atomic.Uint64
	items []T
}

// NewRoundRobin creates a new round-robin inst
func NewRoundRobin[T any](items []T) *RoundRobin[T] {
	return &RoundRobin[T]{
		items: items,
	}
}

// Next gets the next item that is up for use.
func (r *RoundRobin[T]) Next() T {
	n := r.ct.Add(1) - 1
	l := uint64(len(r.items))
	return r.items[int(n%l)]
}

// ForEach applies the supplied closure to each item.
func (r *RoundRobin[T]) ForEach(fn func(T)) {
	for _, item := range r.items {
		fn(item)
	}
}
