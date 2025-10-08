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
	stack  []string
	ctx    context.Context
}

// newVisitor returns a visitor that can be used to visit an accesslist graph.
func newVisitor(ctx context.Context, getter AccessListAndMembersGetter, startNode string) *visitor {
	return &visitor{
		getter: getter,
		seen:   make(map[string]struct{}),
		stack:  []string{startNode},
		ctx:    ctx,
	}
}

// Make sure members can be used as an iter.Seq2
var _ iter.Seq2[membershipRelation, error] = visitor{}.memberships

// a membershipRelation is an edge in the acceslist graph. It describes a
// membership relation between a list and an entity called the member.
// The member can be an accesslist or a user.
type membershipRelation struct {
	list   *accesslist.AccessList
	member *accesslist.AccessListMember
}

// memberships is a single-use iterator yielding every access list membership, direct or nested.
// This function is an iter.Seq2[membershipRelation, error].
// In case of non-nil error, the caller should stop processing as there's no
// guarantee anymore that the graph will be completely traversed.
func (v visitor) memberships(yield func(membershipRelation, error) bool) {
	var alName string

	// Walk the accesslist tree until we no longer have new nested access lists to visit
	for {
		if len(v.stack) == 0 {
			return
		}

		// Stop if the context is done.
		select {
		case <-v.ctx.Done():
			yield(membershipRelation{}, v.ctx.Err())
			return
		default:
		}

		// We take the accesslist on top of the stack
		v.stack, alName = v.stack[:len(v.stack)-1], v.stack[len(v.stack)-1]
		list, err := v.getter.GetAccessList(v.ctx, alName)
		if err != nil {
			yield(membershipRelation{}, err)
			return
		}

		// Get all its members, page by page. We don't wait to get all pages before yielding members
		// as the member we are looking for might be on the page we are consuming.
		// This is some optimization to not have to keep all pages in memory and do an early return.
		// This risk is that the accesslist might get deleted while we are looking at it.
		pageToken := ""
		var page []*accesslist.AccessListMember
		for {
			page, pageToken, err = v.getter.ListAccessListMembers(v.ctx, alName, 0, pageToken)
			if err != nil {
				// If the AccessList doesn't exist yet or has been deleted,
				// we gracefully handle the case and consider it has no more members.
				if trace.IsNotFound(err) {
					break
				}
				yield(membershipRelation{}, err)
				return
			}

			for _, member := range page {
				// If the member is a nested list, we might need to look up its members recursively.
				if member.Spec.MembershipKind == accesslist.MembershipKindList {
					name := member.GetName()
					if _, seen := v.seen[name]; !seen {
						// First time seeing this accesslist, marking it as seen and enqueueing it.
						v.seen[name] = struct{}{}
						v.stack = append(v.stack, name)
					}
				}
				if ok := yield(membershipRelation{
					list:   list,
					member: member,
				}, nil); !ok {
					return
				}
			}

			if pageToken == "" {
				break
			}

		}
	}

}
