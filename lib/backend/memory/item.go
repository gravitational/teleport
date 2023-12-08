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
	"bytes"

	"github.com/google/btree"

	"github.com/gravitational/teleport/lib/backend"
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
	return !bytes.HasPrefix(other.Key, p.prefix)
}
