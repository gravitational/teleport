/*
Copyright 2019 Gravitational, Inc.

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

package memory

import (
	"container/heap"
	"time"
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

func (mh *minHeap) Push(x interface{}) {
	n := len(*mh)
	item := x.(*btreeItem)
	item.index = n
	*mh = append(*mh, item)
}

func (mh *minHeap) Pop() interface{} {
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

// update modifies the priority and value of an Item in the queue.
func (mh *minHeap) UpdateEl(el *btreeItem, expires time.Time) {
	heap.Remove(mh, el.index)
	el.Expires = expires
	heap.Push(mh, el)
}

func (mh *minHeap) RemoveEl(el *btreeItem) {
	heap.Remove(mh, el.index)
}
