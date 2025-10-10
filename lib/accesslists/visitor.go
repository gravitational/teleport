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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/accesslist"
)

type listFilterFunc func(accessList *accesslist.AccessList) bool

func allListsFilter(accessList *accesslist.AccessList) bool {
	return true
}

func userMeetsRequirementsListFilter(user types.User) listFilterFunc {
	return func(accessList *accesslist.AccessList) bool {
		return UserMeetsRequirements(user, accessList.Spec.MembershipRequires)
	}
}

// visitor visits all members of an AccessList graph by doing a depth-first traversal.
// The visitor is cycle-proof.
type visitor struct {
	getter AccessListAndMembersGetter
	seen   map[string]struct{}
	stack  []*accesslist.AccessList
	ctx    context.Context
	filter listFilterFunc
}

// newAccessListUserMemberIterator returns a single-use iterator traversing the
// nested access lists and returning user members.
// In case of non-nil error, the caller should stop processing as there's no
// guarantee anymore that the graph will be completely traversed.
// The caller can optionally pass a listFilterFunc to prevent the iterator from
// visiting specific lists (e.g. restrict the graph traversal to lists a
// specific user cam be member of).
func newAccessListUserMemberIterator(ctx context.Context, getter AccessListAndMembersGetter, accessList *accesslist.AccessList, filterFunc listFilterFunc) iter.Seq2[*accesslist.AccessListMember, error] {
	if filterFunc == nil {
		filterFunc = allListsFilter
	}
	return visitor{
		getter: getter,
		seen:   make(map[string]struct{}),
		stack:  []*accesslist.AccessList{accessList},
		filter: filterFunc,
		ctx:    ctx,
	}.iterateOverMembers
}

func (t visitor) iterateOverMembers(yield func(*accesslist.AccessListMember, error) bool) {
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
		var list *accesslist.AccessList

		for {
			page, pageToken, err = t.getter.ListAccessListMembers(t.ctx, accessList.GetName(), 0, pageToken)
			if err != nil {
				yield(nil, err)
				return
			}

			for _, member := range page {

				// If the member is a nested list, we check if we should process its members.
				if member.Spec.MembershipKind == accesslist.MembershipKindList {
					name := member.GetName()

					// If we already processed this list, we can skip it.
					if _, seen := t.seen[name]; seen {
						continue
					}
					// First time seeing this list, marking it as seen.
					t.seen[name] = struct{}{}

					list, err = t.getter.GetAccessList(t.ctx, name)
					if err != nil {
						// Gracefully handle the missing access list case,
						// to avoid breaking everything in case of membership inconsistency.
						if trace.IsNotFound(err) {
							t.seen[name] = struct{}{}
							continue
						}
						yield(nil, trace.Wrap(err))
					}

					// Check if we should consider the list or skip it.
					if t.filter(list) {
						t.stack = append(t.stack, list)
					}
					continue
				}

				// This is not a nested list but an individual member.
				if ok := yield(member, nil); !ok {
					return
				}
			}

			if pageToken == "" {
				break
			}

		}
	}

}
