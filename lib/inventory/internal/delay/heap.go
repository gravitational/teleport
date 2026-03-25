// Copyright 2025 Gravitational, Inc.
// Copyright 2009 The Go Authors
// SPDX-License-Identifier: BSD-3-Clause

package delay

type noUnkeyedLiterals struct{}

type heap[T any] struct {
	_ noUnkeyedLiterals

	Less  func(T, T) bool
	Slice []T
}

func (h *heap[T]) up(j int) {
	for {
		i := (j - 1) / 2 // parent
		if i == j || !h.Less(h.Slice[j], h.Slice[i]) {
			break
		}
		h.Slice[i], h.Slice[j] = h.Slice[j], h.Slice[i]
		j = i
	}
}

func (h *heap[T]) down(i int) bool {
	i0 := i
	for {
		j1 := 2*i + 1
		if j1 >= len(h.Slice) || j1 < 0 { // j1 < 0 after int overflow
			break
		}
		j := j1 // left child
		if j2 := j1 + 1; j2 < len(h.Slice) && h.Less(h.Slice[j2], h.Slice[j1]) {
			j = j2 // = 2*i + 2  // right child
		}
		if !h.Less(h.Slice[j], h.Slice[i]) {
			break
		}
		h.Slice[i], h.Slice[j] = h.Slice[j], h.Slice[i]
		i = j
	}
	return i > i0
}

func (h *heap[T]) Root() *T {
	if len(h.Slice) == 0 {
		return nil
	}

	return &h.Slice[0]
}

func (h *heap[T]) FixRoot() {
	h.Fix(0)
}

func (h *heap[T]) Init() {
	// heapify
	n := len(h.Slice)
	for i := n/2 - 1; i >= 0; i-- {
		h.down(i)
	}
}

func (h *heap[T]) Push(x T) {
	h.Slice = append(h.Slice, x)
	h.up(len(h.Slice) - 1)
}

func (h *heap[T]) Pop() T {
	n := len(h.Slice) - 1
	x := h.Slice[0]
	h.Slice[0] = h.Slice[n]
	h.Slice[n] = *new(T)
	h.Slice = h.Slice[:n]
	if n != 0 {
		h.down(0)
	}
	return x
}

func (h *heap[T]) Remove(i int) T {
	n := len(h.Slice) - 1
	x := h.Slice[i]
	h.Slice[i] = h.Slice[n]
	h.Slice[n] = *new(T)
	h.Slice = h.Slice[:n]
	if n != i {
		if !h.down(i) {
			h.up(i)
		}
	}
	return x
}

func (h *heap[T]) Fix(i int) {
	if !h.down(i) {
		h.up(i)
	}
}

func (h *heap[T]) Clear() {
	h.Slice = nil
}
