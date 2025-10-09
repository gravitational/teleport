/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package accesslists

import (
	"context"
	"iter"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/accesslist"
)

// visitor visits all members of an AccessList graph by doing a depth-first traversal.
// The visitor is cycle-proof.
type visitor struct {
	getter AccessListAndMembersGetter
	seen   map[string]struct{}
	stack  []*accesslist.AccessList
	ctx    context.Context
}

// newAccessListMemberIterator returns a single-use iterator traversing the accesslist
// graph membership edges and yielding every vertex.
// In case of non-nil error, the caller should stop processing as there's no
// guarantee anymore that the graph will be completely traversed.
// If the caller wants to prune a node and its subgraph it can call vertex.discard.
func newAccessListMemberIterator(ctx context.Context, getter AccessListAndMembersGetter, accessList *accesslist.AccessList) iter.Seq2[vertex, error] {
	return visitor{
		getter: getter,
		seen:   make(map[string]struct{}),
		stack:  []*accesslist.AccessList{accessList},
		ctx:    ctx,
	}.iterate
}

// Make sure members can be used as an iter.Seq2
var _ iter.Seq2[vertex, error] = visitor{}.iterate

// vertex is a node in the acceslist graph. This is stored in the backend as a member.
// The member can be an accesslist or a user. If the member kind is a list, the
// vertex list field is populated.
// The vertex contains a discard function that the caller can use to indicate the
// iterator should not attempt to traverse arcs from the vertex. This is used to
// filter out access lists whose requirements are not met. If the vertex is not a
// list, discard has no effect.
type vertex struct {
	member  *accesslist.AccessListMember
	list    *accesslist.AccessList
	discard func()
}

func (t visitor) iterate(yield func(vertex, error) bool) {
	var accessList *accesslist.AccessList

	// Walk the accesslist tree until we no longer have new nested access lists to visit
	for {
		if len(t.stack) == 0 {
			return
		}

		// We take the accesslist on top of the stack
		t.stack, accessList = t.stack[:len(t.stack)-1], t.stack[len(t.stack)-1]

		// Get all its members, page by page. We don't wait to get all pages before yielding members
		// as the member we are looking for might be on the page we are consuming.
		// This is some optimization to not have to keep all pages in memory and do an early return.
		// This risk is that the accesslist might get deleted while we are looking at it.
		pageToken := ""
		var page []*accesslist.AccessListMember
		var err error
		var discard bool
		var list *accesslist.AccessList

		for {
			page, pageToken, err = t.getter.ListAccessListMembers(t.ctx, accessList.GetName(), 0, pageToken)
			if err != nil {
				yield(vertex{}, err)
				return
			}

			for _, member := range page {
				discard = false
				v := vertex{
					member:  member,
					discard: func() { discard = true },
				}

				// If the member is a nested list, look it up, mark it as seen and populate vertex.list
				if member.Spec.MembershipKind == accesslist.MembershipKindList {
					name := member.GetName()

					// If we already processed this vertex, we can skip it.
					if _, seen := t.seen[name]; seen {
						continue
					}

					list, err = t.getter.GetAccessList(t.ctx, name)
					if err != nil {
						// Gracefully handle the missing access list case, to avoid breaking everything in case of
						// membership inconsistency.
						if trace.IsNotFound(err) {
							t.seen[name] = struct{}{}
							continue
						}
						yield(vertex{}, trace.Wrap(err))
					}

					// First time seeing this accesslist, marking it as seen and enqueueing it.
					t.seen[name] = struct{}{}
					t.stack = append(t.stack, list)
					v.list = list
				}

				if ok := yield(v, nil); !ok {
					return
				}

				if discard && member.Spec.MembershipKind == accesslist.MembershipKindList {
					t.stack = t.stack[:len(t.stack)-1]
				}
			}

			if pageToken == "" {
				break
			}

		}
	}

}
