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

package delay

import (
	"container/heap"
	"fmt"
	"time"
)

type entry[T any] struct {
	tick time.Time
	key  T
}

func (e *entry[T]) String() string {
	return fmt.Sprintf("entry{tick: %v, key: %v}", e.tick.Format(time.RFC3339Nano), e.key)
}

type tickHeap[T comparable] struct {
	slice []*entry[T]
}

func (b *tickHeap[T]) iface() *tickHeapIface[T] {
	return (*tickHeapIface[T])(b)
}

func (b *tickHeap[T]) Push(v *entry[T]) {
	heap.Push(b.iface(), v)
}

func (b *tickHeap[T]) Pop() *entry[T] {
	return heap.Pop(b.iface()).(*entry[T])
}

func (b *tickHeap[T]) Remove(key T) {
	for i, v := range b.slice {
		if v.key == key {
			heap.Remove(b.iface(), i)
			return
		}
	}
}

func (b *tickHeap[T]) Len() int {
	return len(b.slice)
}

type tickHeapIface[T comparable] tickHeap[T]

var _ heap.Interface = (*tickHeapIface[int])(nil)

// Len implements [heap.Interface].
func (b *tickHeapIface[T]) Len() int {
	return len(b.slice)
}

// Less implements [heap.Interface].
func (b tickHeapIface[T]) Less(i int, j int) bool {
	return b.slice[i].tick.Before(b.slice[j].tick)
}

// Swap implements [heap.Interface].
func (b tickHeapIface[T]) Swap(i int, j int) {
	b.slice[i], b.slice[j] = b.slice[j], b.slice[i]
}

// Push implements [heap.Interface].
func (b *tickHeapIface[T]) Push(x any) {
	b.slice = append(b.slice, x.(*entry[T]))
}

// Pop implements [heap.Interface].
func (b *tickHeapIface[T]) Pop() any {
	last := len(b.slice) - 1
	v := b.slice[last]
	b.slice = b.slice[:last]
	clear(b.slice[last:])
	return v
}
