// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package assignments

import (
	"iter"
	"strings"

	"github.com/google/btree"

	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	apiscopes "github.com/gravitational/teleport/api/scopes"
)

// IndexedAssignment contains information about a scope assignment contained in
// the scope assignment index. This struct is not safe for concurrent use.
type IndexedAssignment struct {
	// Scope is the scope of effect, as indicated by the subassignment.
	Scope string
	// Role is the role assigned within a given scope.
	Role string
	// RoleAssignmentScope is the scope of origin, i.e. the scope where the
	// assignment resource has been declared.
	RoleAssignmentScope string
}

// assignmentIndexTreeItem is the actual structure contained in the index.
type assignmentIndexTreeItem struct {
	IndexedAssignment
	// refCount is the number of separate assignments that defined the given
	// indexed assignment. Used to make sure that deleting an role assignment
	// resource doesn't delete the indexed assignment that is also defined by
	// another role assignment resource.
	refCount int
}

func assignmentIndexTreeItemLess(a assignmentIndexTreeItem, b assignmentIndexTreeItem) bool {
	diff := apiscopes.Sort(a.Scope, b.Scope)
	if diff != 0 {
		return diff < 0
	}

	if a.Role < b.Role {
		return true
	}
	if a.Role > b.Role {
		return false
	}

	return apiscopes.Sort(a.RoleAssignmentScope, b.RoleAssignmentScope) < 0
}

// assignmentIndex keeps track of assignments for all users and keeps them
// sorted by scope name in the context of a given user.
type assignmentIndex struct {
	// assignmentsByUser maps the user name to a sorted view of the scopes
	// assigned to this user.
	assignmentsByUser map[string]*btree.BTreeG[assignmentIndexTreeItem]
}

func newAssignmentIndex() *assignmentIndex {
	return &assignmentIndex{
		assignmentsByUser: map[string]*btree.BTreeG[assignmentIndexTreeItem]{},
	}
}

func (idx *assignmentIndex) Put(assignment *scopedaccessv1.ScopedRoleAssignment) {
	user := assignment.GetSpec().GetUser()
	if user == "" {
		return
	}

	userAssignments, ok := idx.assignmentsByUser[user]
	if !ok {
		const (
			// bTreeDegree of 8 is standard across most of the teleport codebase
			bTreeDegree = 8
		)
		userAssignments = btree.NewG(bTreeDegree, assignmentIndexTreeItemLess)
		idx.assignmentsByUser[user] = userAssignments
	}
	for _, a := range assignment.GetSpec().GetAssignments() {
		key := assignmentIndexTreeItem{
			IndexedAssignment: IndexedAssignment{
				Scope:               a.GetScope(),
				Role:                a.GetRole(),
				RoleAssignmentScope: assignment.GetScope(),
			},
		}
		existing, ok := userAssignments.Get(key)
		if ok {
			existing.refCount++
			userAssignments.ReplaceOrInsert(existing)
		} else {
			key.refCount = 1
			userAssignments.ReplaceOrInsert(key)
		}
	}
}

func (idx *assignmentIndex) Del(sa *scopedaccessv1.ScopedRoleAssignment) {
	user := sa.GetSpec().GetUser()
	userAssignments, ok := idx.assignmentsByUser[user]
	if !ok {
		return
	}
	for _, a := range sa.GetSpec().GetAssignments() {
		key := assignmentIndexTreeItem{
			IndexedAssignment: IndexedAssignment{
				Scope:               a.GetScope(),
				Role:                a.GetRole(),
				RoleAssignmentScope: sa.GetScope(),
			},
		}
		existing, ok := userAssignments.Get(key)
		if !ok {
			continue
		}

		if existing.refCount <= 1 {
			userAssignments.Delete(key)
		} else {
			existing.refCount--
			userAssignments.ReplaceOrInsert(existing)
		}
	}

	if userAssignments.Len() <= 0 {
		delete(idx.assignmentsByUser, user)
	}
}

// AssignmentsForUser returns an iterator over a given user's assigned scopes
// in alphabetical order. Only scopes that start from a certain prefix are
// returned. The `start` argument, if not empty, indicates that only scopes
// that are greater or equal to the start scope (in alphabetical order) are
// returned.
//
// Since [assignmentIndex] is not safe for concurrent use, concurrent callers
// must hold a read lock through the entire time of iteration over the returned
// iterator.
func (idx *assignmentIndex) AssignmentsForUser(
	username string, prefix string, start string,
) iter.Seq[IndexedAssignment] {
	return func(yield func(IndexedAssignment) bool) {
		userAssignments, ok := idx.assignmentsByUser[username]
		if !ok {
			return
		}

		startKey := assignmentIndexTreeItem{}
		if apiscopes.Sort(prefix, start) > 0 {
			startKey.Scope = prefix
		} else {
			startKey.Scope = start
		}

		trimmedPrefix := strings.TrimSuffix(prefix, "/")
		userAssignments.AscendGreaterOrEqual(startKey, func(item assignmentIndexTreeItem) bool {
			if strings.HasPrefix(item.Scope, prefix) {
				return yield(item.IndexedAssignment)
			}
			// Special case: continue iteration without yielding if the prefix ends
			// with a slash, but is otherwise equal to this scope. In this case,
			// don't terminate, but also don't yield this scope; descend into the
			// children instead.
			return item.Scope == trimmedPrefix
		})
	}
}
