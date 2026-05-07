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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/trace"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/clientutils"
)

var (
	// Not using trace errors is intentional here as those errors are expected.
	// They are used to control the graph traversal. Using trace.Errorf() or
	// other trace errors would cause useless stacktrace captures each time an
	// error is returned.
	//nolint:staticcheck // we mimick the Walk err naming, which don't follow the errFoo pattern
	skipLeg = errors.New("skip this access leg")
	//nolint:staticcheck // we mimick the Walk err naming, which don't follow the errFoo pattern
	skipAll = errors.New("skip everything and stop the walk")
)

// walkFunc is the type of the function called by walk on each
// access list member and nested access list part of the access graph.
// The error result returned by the function controls how accessPath continues.
// If the function returns the special value skipLeg, walk skips the accessPath.
// If the function returns the special value skipAll, walk skips all remaining
// accessPath. Otherwise, if the function returns a non-nil error, walk stops
// entirely and returns that error.
type walkFunc func(path accessPath) error

// walkUntilUser returns a walkFunc that filters out every invalid
// accessLeg. Invalid access legs are:
// - expired legs
// - legs granting access to a different user
// - legs granting access to a list whose requirements are not met by the user
func isAccessListMember(ctx context.Context, user types.User, cfg walkConfig, now time.Time) (accesslistv1.AccessListUserAssignmentType, error) {
	var skipped []skippedAccessPath
	var result accesslistv1.AccessListUserAssignmentType

	walkFn := func(path accessPath) error {
		// First, skip the path if it  doesn't meet the requirements.
		// This also prevents the walker from continuing to consider paths
		// containing this leg, as they would be invalid as well.

		// Because the walkFunc was already called on every leg of the path,
		// we only have to consider the last one.
		leg := path[len(path)-1]
		if leg.member != nil {
			// If the membership is for a user but not the one we are looking for, we do nothing.
			if leg.member.Spec.MembershipKind == accesslist.MembershipKindUser && leg.member.Spec.Name != user.GetName() {
				return nil
			}

			// If the membership is expired, it is invalid and we skip it.
			// This check is common for list and user members.
			if !leg.member.Spec.Expires.IsZero() && !now.Before(leg.member.Spec.Expires) {
				skipped = append(skipped, skippedAccessPath{path, "expired"})
				// Sometimes we might return skipLeg on a user membership instead of a list member.
				// walk should handle this properly.
				return skipLeg
			}
		}

		// If the member is a list but user doesn't meet the list's membership requirements, the leg is invalid.
		if leg.list != nil && !UserMeetsRequirements(user, leg.list.Spec.MembershipRequires) {
			skipped = append(skipped, skippedAccessPath{path, "did not meet list requirements"})
			return skipLeg
		}

		// At this point, we have a valid path. If this is the path we are
		// looking for, we save the result and tell the walker to stop.

		// We are only looking for user memberships, we ignore the list memberships.
		if leg.member == nil || leg.member.Spec.MembershipKind != accesslist.MembershipKindUser {
			return nil
		}

		// If the path is composed of only 2 components: the start list and
		// the user membership, this is an explicit assignment.
		// For example: ["my-list", "my-user"].
		// We can do this check even when walk is doing depth-first traversal
		// because it processes every direct member before looking into nested
		// lists.
		if len(path) == 2 {
			result = accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_EXPLICIT
		} else {
			// Else the assignment is inherited through one or many levels of nested access lists.
			// For example: ["my-list", "my-nested-list", "my-user"]
			result = accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_INHERITED
		}
		// We found at least one valid access path from the root to our user.
		// No need to look further, we tell the walker to stop.
		return skipAll
	}

	if err := walk(ctx, cfg, walkFn); err != nil {
		return accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED, trace.Wrap(err, "walking the access list graph")
	}

	if result == accesslistv1.AccessListUserAssignmentType_ACCESS_LIST_USER_ASSIGNMENT_TYPE_UNSPECIFIED {
		// If we land here, no valid access paths were identified.
		// To make troubleshooting easier, we optionally return a string
		// explaining which access paths were filtered out.
		return result, trace.AccessDenied("User is not member of the access list, directly or via nested list: %s", explainSkipped(skipped))
	}
	return result, nil
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

// walkConfig contains configuration required for walking the access list graph with walk.
// All fields are requied.
type walkConfig struct {
	getter AccessListAndMembersGetter
	// root is from where we start to walk the graph.
	root *accesslist.AccessList
}

// check if the walkConfig is valid.
func (c walkConfig) check() error {
	if c.getter == nil {
		return trace.BadParameter("getter is required (this is a bug)")
	}

	if c.root == nil {
		return trace.BadParameter("root is required (this is a bug)")
	}

	return nil
}

// walk walks the AccessList graph rooted at root, calling config.walkFn for
// each user or access list in the graph, including root.
// This does not exhaustively go through every valid accessPath:
// If several valid paths go through the same list, only one of them is walked.
// walk is doing a depth-first traversal of nested lists, but will
// go through every direct member of an access list before looking into nested
// lists. This function supports cyclic graphs.
func walk(ctx context.Context, config walkConfig, walkFn walkFunc) error {
	if err := config.check(); err != nil {
		return trace.Wrap(err, "checking access list walk config")
	}

	stack := make([]accessPath, 0)
	firstLeg := accessLeg{
		list: config.root,
	}

	err := walkFn(accessPath{firstLeg})
	if err != nil {
		// if the first leg is skipped, we return early
		if err == skipLeg || err == skipAll { //nolint:errorlint // error can't be wrapped
			return nil
		}
		return trace.Wrap(err)
	}
	stack = append(stack, accessPath{firstLeg})
	seen := map[string]struct{}{config.root.GetName(): {}}

	var path accessPath
	var list *accesslist.AccessList

	// Walk the accesslist tree until we no longer have new nested access lists to visit
	for len(stack) != 0 {
		// We take the accesslist on top of the stack
		stack, path = stack[:len(stack)-1], stack[len(stack)-1]
		list = path[len(path)-1].list

		var err error
		var nestedList *accesslist.AccessList
		var leg accessLeg
		var member *accesslist.AccessListMember

		// We iterate over every member of the considered list
		listMembersFn := func(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
			r, token, err := config.getter.ListAccessListMembers(ctx, list.GetName(), pageSize, pageToken)
			return r, token, trace.Wrap(err)
		}

		for member, err = range clientutils.Resources(ctx, listMembersFn) {
			if err != nil {
				return trace.Wrap(err, "getting access list members for %q", list.GetName())
			}

			if member.Spec.MembershipKind == accesslist.MembershipKindList {
				// The member is a nested list.
				name := member.GetName()

				// If we already walked a valid path to this list, skip it.
				if _, seen := seen[name]; seen {
					continue
				}

				// Note: here we don't cache the accesslist response, so we might
				// get the same AL several times if the accessLeg is filtered out.
				// It's a bit inefficient but should not happen often, it's
				// more relevant for us to avoid keeping everything in-memory.
				nestedList, err = config.getter.GetAccessList(ctx, name)
				if err != nil {
					// Gracefully handle the missing access list case,
					// to avoid breaking everything in case of membership inconsistency.
					if trace.IsNotFound(err) {
						seen[name] = struct{}{}
						continue
					}
					return trace.Wrap(err, "getting access list %q", name)
				}

				// Try to walk the leg.
				leg = accessLeg{member: member, list: nestedList}
				if err := walkFn(append(path, leg)); err != nil {
					if err == skipLeg { //nolint:errorlint // error can't be wrapped
						continue
					} else if err == skipAll { //nolint:errorlint // error can't be wrapped
						return nil
					}
					return trace.Wrap(err, "calling walk function for list %q at %q", name, append(path, leg))
				}

				// We got a valid path, and it's the first time seeing this list: marking it as seen.
				seen[name] = struct{}{}

				stack = append(stack, append(path, leg))
				continue
			}

			leg = accessLeg{member: member}
			// This is not a nested list but an individual member.
			// Check if the member passes the walkFn.
			if err := walkFn(append(path, leg)); err != nil {
				if err == skipLeg { //nolint:errorlint // error can't be wrapped
					// Although skipLeg doesn't make sense for a user, some of the checks from
					// the walkFunc can be common for list and user members (e.g. expiry).
					// In this case we might receive a skipLeg error and should handle it gracefully.
					continue
				} else if err == skipAll { //nolint:errorlint // error can't be wrapped
					return nil
				}
				return trace.Wrap(err, "calling walk function for member %q at %q", member.GetName(), append(path, leg))
			}
		}
	}

	// If we landed here, we're done walking the access graph and can return
	return nil
}

func explainSkipped(skipped []skippedAccessPath) string {
	if len(skipped) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\nWhen resolving access, the following access paths were ignored:")
	for _, path := range skipped {
		fmt.Fprintf(&sb, "\n * %q because %s", path, path.reason)
	}
	return sb.String()
}
