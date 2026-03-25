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

package utils

import (
	"context"

	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

// RangeScopedRoles is a helper function that streams scoped roles from the provided reader.
// NOTE: this function mutates the request's PageToken field during iteration.
func RangeScopedRoles(ctx context.Context, reader services.ScopedRoleReader, req *scopedaccessv1.ListScopedRolesRequest) stream.Stream[*scopedaccessv1.ScopedRole] {
	return clientutils.Resources(ctx, func(ctx context.Context, pageSize int, pageToken string) ([]*scopedaccessv1.ScopedRole, string, error) {
		req.PageSize = int32(pageSize)
		req.PageToken = pageToken
		rsp, err := reader.ListScopedRoles(ctx, req)
		if err != nil {
			return nil, "", err
		}
		return rsp.GetRoles(), rsp.GetNextPageToken(), nil
	})
}

// RangeScopedRoleAssignments is a helper function that streams scoped role assignments from the provided reader.
// NOTE: this function mutates the request's PageToken field during iteration.
func RangeScopedRoleAssignments(ctx context.Context, reader services.ScopedRoleAssignmentReader, req *scopedaccessv1.ListScopedRoleAssignmentsRequest) stream.Stream[*scopedaccessv1.ScopedRoleAssignment] {
	return clientutils.Resources(ctx, func(ctx context.Context, pageSize int, pageToken string) ([]*scopedaccessv1.ScopedRoleAssignment, string, error) {
		req.PageSize = int32(pageSize)
		req.PageToken = pageToken
		rsp, err := reader.ListScopedRoleAssignments(ctx, req)
		if err != nil {
			return nil, "", err
		}
		return rsp.GetAssignments(), rsp.GetNextPageToken(), nil
	})
}
