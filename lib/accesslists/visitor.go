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
	"fmt"
	"iter"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/clientutils"
)

const (
	filteredOutReasonExpired           = "expired"
	filteredOutReasonUnmetRequirements = "user did not meet list requirements"
)

// accessFilterFunc takes an access leg and returns true if it should be considered
// when traversing the accesslist graph. When returning false, a reason can be
// optionally specified. This helps troubleshooting access issues.
type accessFilterFunc func(leg accessLeg) (bool, string)

// validForUserFilter returns an accessFilterFunc that filters out every invalid
// accessLeg. Invalid access legs are:
// - expired legs
// - legs granting access to a different user
// - legs granting access to a list whose requirements are not met by the user
func validForUserFilter(user types.User, now time.Time) accessFilterFunc {
	return func(leg accessLeg) (bool, string) {
		if leg.member != nil {
			// If the membership is expired, it is invalid.
			if !leg.member.Spec.Expires.IsZero() && !now.Before(leg.member.Spec.Expires) {
				return false, filteredOutReasonExpired
			}
			// If the membership is for a user but the not the one we are looking for, we filter it out.
			if leg.member.Spec.MembershipKind == accesslist.MembershipKindUser && leg.member.Spec.Name != user.GetName() {
				return false, ""
			}
		}

		// If the member is a list but user doesn't meet the list's membership requirements, the leg is invalid.
		if leg.list != nil && !UserMeetsRequirements(user, leg.list.Spec.MembershipRequires) {
			return false, filteredOutReasonUnmetRequirements
		}

		return true, ""
	}
}

// visitor visits all members of an AccessList graph by doing a depth-first traversal.
// The visitor is cycle-proof.
type visitor struct {
	getter       AccessListAndMembersGetter
	seen         map[string]struct{}
	stack        []accessPath
	filter       accessFilterFunc
	ctx          context.Context
	skippedPaths []skippedAccessPath
}

// accessPath represents a path in the access list graph from the start list to
// a member.
type accessPath []accessLeg

// String implements stringer and provides a text representation of an accessPath.
// This is used to explain access decisions in user-facing error messages.
func (path accessPath) String() string {
	var sb strings.Builder
	for _, leg := range path {
		if leg.member != nil {
			sb.WriteString(" --> ")
		}
		if leg.list != nil {
			sb.WriteString(leg.list.GetName())
		} else {
			sb.WriteString("user")
		}
	}
	return sb.String()
}

// accessLeg represents one leg of an access path.
// The first leg of the path has a nil member.
// If the accessLeg target is an access list (as opposed to a user), list is non-nil.
type accessLeg struct {
	member *accesslist.AccessListMember
	list   *accesslist.AccessList
}

// skippedAccessPath is an accessPath that got filtered out for a reason worth
// surfacing to the end user/
type skippedAccessPath struct {
	accessPath
	reason string
}

// newAccessPathVisitor returns a single-use iterator traversing the
// nested access lists and returning access paths.
// In case of non-nil error, the caller should stop processing as there's no
// guarantee anymore that the graph will be completely traversed.
// The caller can optionally pass a listFilterFunc to prevent the iterator from
// visiting specific lists (e.g. restrict the graph traversal to lists a
// specific user cam be member of).
func newAccessPathVisitor(
	ctx context.Context,
	getter AccessListAndMembersGetter,
	accessList *accesslist.AccessList,
	filter accessFilterFunc) (*visitor, error) {

	if filter == nil {
		return nil, trace.BadParameter("filter is required (this is a bug)")
	}

	if accessList == nil {
		return nil, trace.BadParameter("accessList is required (this is a bug)")
	}

	stack := make([]accessPath, 0)
	skipped := make([]skippedAccessPath, 0)
	start := accessLeg{
		list: accessList,
	}

	ok, reason := filter(start)
	if ok {
		stack = append(stack, accessPath{start})
	} else if reason != "" {
		skipped = append(skipped, skippedAccessPath{accessPath{start}, reason})
	}

	seen := make(map[string]struct{})
	seen[accessList.GetName()] = struct{}{}

	return &visitor{
		getter:       getter,
		seen:         seen,
		stack:        stack,
		ctx:          ctx,
		filter:       filter,
		skippedPaths: skipped,
	}, nil
}

// Ensure that visitor.accessPaths is a valid sequence iterator.
var _ iter.Seq2[accessPath, error] = (&visitor{}).accessPaths

// accessPaths returns an iterator yielding complete accessPaths meeting the
// filter requirements. This does not exhaustively list every valid accessPath.
// If several valid paths go through the same list, only one of them is yielded.
func (v *visitor) accessPaths(yield func(accessPath, error) bool) {
	var path accessPath
	var list *accesslist.AccessList

	// Walk the accesslist tree until we no longer have new nested access lists to visit
	for {
		if len(v.stack) == 0 {
			return
		}

		// We take the accesslist on top of the stack
		v.stack, path = v.stack[:len(v.stack)-1], v.stack[len(v.stack)-1]
		list = path[len(path)-1].list

		var err error
		var nestedList *accesslist.AccessList
		var leg accessLeg
		var member *accesslist.AccessListMember

		// We iterate over every member of the considered list
		listMembersFn := func(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
			r, token, err := v.getter.ListAccessListMembers(v.ctx, list.GetName(), pageSize, pageToken)
			return r, token, trace.Wrap(err)
		}

		for member, err = range clientutils.Resources(v.ctx, listMembersFn) {
			if err != nil {
				yield(nil, trace.Wrap(err))
			}

			if member.Spec.MembershipKind == accesslist.MembershipKindList {
				// The member is a nested list.
				name := member.GetName()

				// If we already have a valid path to this list, skip it.
				if _, seen := v.seen[name]; seen {
					continue
				}

				// Note: here we don't cache the accesslist response, so we might
				// get the same AL several times if the accessLeg is filtered out.
				// It's a bit inefficient but should not happen often, it's
				// more relevant for us to avoid keeping everything in-memory.
				nestedList, err = v.getter.GetAccessList(v.ctx, name)
				if err != nil {
					// Gracefully handle the missing access list case,
					// to avoid breaking everything in case of membership inconsistency.
					if trace.IsNotFound(err) {
						v.seen[name] = struct{}{}
						continue
					}
					yield(nil, trace.Wrap(err))
				}

				// Check if the leg is valid
				leg = accessLeg{member: member, list: nestedList}
				if ok, reason := v.filter(leg); !ok {
					// If the leg is not valid and user should be informed, we add it to skipped paths.
					if reason != "" {
						v.skippedPaths = append(v.skippedPaths, skippedAccessPath{append(path, leg), reason})
					}
					continue
				}

				// We got a valid path, and it's the first time seeing this list: marking it as seen.
				v.seen[name] = struct{}{}

				v.stack = append(v.stack, append(path, leg))
				continue
			}

			leg = accessLeg{member: member}
			// This is not a nested list but an individual member.
			// Check if the member passes the filter.
			if ok, reason := v.filter(leg); !ok {
				// If the leg is not valid and the caller should be informed, we add it to skipped paths.
				if reason != "" {
					v.skippedPaths = append(v.skippedPaths, skippedAccessPath{append(path, leg), reason})
				}
				continue
			}

			// If it does, return the access path.
			if ok := yield(append(path, leg), nil); !ok {
				return
			}

		}
	}
}

func (v *visitor) explainAccessDecision() string {
	var sb strings.Builder
	sb.WriteString("User is not member of the access list, directly or via nested list")
	if len(v.skippedPaths) == 0 {
		return sb.String()
	}
	sb.WriteString("\nWhen resolving access, the following access paths were ignored:")
	for _, path := range v.skippedPaths {
		sb.WriteString(fmt.Sprintf("\n * %q because %s", path, path.reason))
	}
	return sb.String()
}
