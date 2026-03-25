/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package memory

import (
	"container/heap"
)

// minHeap implements heap.Interface and holds backend items
type minHeap []*btreeItem

// newMinHeap returns a new min heap
func newMinHeap() *minHeap {
	mh := &minHeap{}
	heap.Init(mh)
	return mh
}

func (mh minHeap) Len() int { return len(mh) }

func (mh minHeap) Less(i, j int) bool {
	return mh[i].Expires.Unix() < mh[j].Expires.Unix()
}

func (mh minHeap) Swap(i, j int) {
	mh[i], mh[j] = mh[j], mh[i]
	mh[i].index = i
	mh[j].index = j
}

func (mh *minHeap) Push(x any) {
	n := len(*mh)
	item := x.(*btreeItem)
	item.index = n
	*mh = append(*mh, item)
}

func (mh *minHeap) Pop() any {
	old := *mh
	n := len(old)
	item := old[n-1]
	item.index = -1 // for safety
	*mh = old[0 : n-1]
	return item
}

func (mh *minHeap) PushEl(el *btreeItem) {
	heap.Push(mh, el)
}

func (mh *minHeap) PopEl() *btreeItem {
	el := heap.Pop(mh)
	return el.(*btreeItem)
}

func (mh *minHeap) PeekEl() *btreeItem {
	items := *mh
	return items[0]
}

func (mh *minHeap) RemoveEl(el *btreeItem) {
	heap.Remove(mh, el.index)
}
