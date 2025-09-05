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

package access

import (
	"context"

	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

// StreamRoles is a helper function that streams scoped roles from the provided reader.
func StreamRoles(ctx context.Context, reader services.ScopedRoleReader) stream.Stream[*scopedaccessv1.ScopedRole] {
	return func(yield func(*scopedaccessv1.ScopedRole, error) bool) {
		var cursor string
		for {
			rsp, err := reader.ListScopedRoles(ctx, &scopedaccessv1.ListScopedRolesRequest{
				PageToken: cursor,
			})
			if err != nil {
				yield(nil, err)
				return
			}

			for _, role := range rsp.GetRoles() {
				if !yield(role, nil) {
					return
				}
			}

			cursor = rsp.GetNextPageToken()

			if cursor == "" {
				break
			}
		}
	}
}

// StreamAssignments is a helper function that streams scoped role assignments from the provided reader.
func StreamAssignments(ctx context.Context, reader services.ScopedRoleAssignmentReader) stream.Stream[*scopedaccessv1.ScopedRoleAssignment] {
	return func(yield func(*scopedaccessv1.ScopedRoleAssignment, error) bool) {
		var cursor string
		for {
			rsp, err := reader.ListScopedRoleAssignments(ctx, &scopedaccessv1.ListScopedRoleAssignmentsRequest{
				PageToken: cursor,
			})
			if err != nil {
				yield(nil, err)
				return
			}

			for _, assignment := range rsp.GetAssignments() {
				if !yield(assignment, nil) {
					return
				}
			}

			cursor = rsp.GetNextPageToken()

			if cursor == "" {
				break
			}
		}
	}
}
