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
	"bytes"

	"github.com/gravitational/teleport/lib/backend"

	"github.com/google/btree"
)

// btreeItem is a copy of a backend item
// stored in the B-Tree and containing additional informatoin
// about B-Tree
type btreeItem struct {
	backend.Item
	index int
}

// Less is used for Btree operations,
// returns true if item is less than the other one
func (i *btreeItem) Less(iother btree.Item) bool {
	switch other := iother.(type) {
	case *btreeItem:
		return bytes.Compare(i.Key, other.Key) < 0
	case *prefixItem:
		return !iother.Less(i)
	default:
		return false
	}
}

// prefixItem is used for prefix matches on a B-Tree
type prefixItem struct {
	// prefix is a prefix to match
	prefix []byte
}

// Less is used for Btree operations
func (p *prefixItem) Less(iother btree.Item) bool {
	other := iother.(*btreeItem)
	if bytes.HasPrefix(other.Key, p.prefix) {
		return false
	}
	return true
}
