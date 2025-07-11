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

package services

import (
	"context"

	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
)

// ScopedAccess provides an API for managing scoped access-control resources.
type ScopedAccess interface {
	ScopedAccessReader
	ScopedAccessWriter
}

// ScopedAccessReader provides an interface for reading scoped access resources.
type ScopedAccessReader interface {
	ScopedRoleReader
	ScopedRoleAssignmentReader
}

// ScopedAccessWriter provides an interface for writing scoped access resources.
type ScopedAccessWriter interface {
	ScopedRoleWriter
	ScopedRoleAssignmentWriter
}

// ScopedRoleReader provides an interface for reading scoped roles.
type ScopedRoleReader interface {
	// GetScopedRole gets a scoped role by name.
	GetScopedRole(context.Context, *scopedaccessv1.GetScopedRoleRequest) (*scopedaccessv1.GetScopedRoleResponse, error)

	// ListScopedRoles returns a paginated list of scoped roles.
	ListScopedRoles(context.Context, *scopedaccessv1.ListScopedRolesRequest) (*scopedaccessv1.ListScopedRolesResponse, error)
}

// ScopedRoleWriter provides an interface for writing scoped roles.
type ScopedRoleWriter interface {
	// CreateScopedRole creates a new scoped role.
	CreateScopedRole(context.Context, *scopedaccessv1.CreateScopedRoleRequest) (*scopedaccessv1.CreateScopedRoleResponse, error)

	// UpdateScopedRole updates a scoped role.
	UpdateScopedRole(context.Context, *scopedaccessv1.UpdateScopedRoleRequest) (*scopedaccessv1.UpdateScopedRoleResponse, error)

	// DeleteScopedRole deletes a scoped role.
	DeleteScopedRole(context.Context, *scopedaccessv1.DeleteScopedRoleRequest) (*scopedaccessv1.DeleteScopedRoleResponse, error)
}

// ScopedRoleAssignmentReader provides an interface for reading scoped role assignments.
type ScopedRoleAssignmentReader interface {
	// GetScopedRoleAssignment gets a scoped role assignment by name.
	GetScopedRoleAssignment(context.Context, *scopedaccessv1.GetScopedRoleAssignmentRequest) (*scopedaccessv1.GetScopedRoleAssignmentResponse, error)

	// ListScopedRoleAssignments returns a paginated list of scoped role assignments.
	ListScopedRoleAssignments(context.Context, *scopedaccessv1.ListScopedRoleAssignmentsRequest) (*scopedaccessv1.ListScopedRoleAssignmentsResponse, error)
}

// ScopedRoleAssignmentWriter provides an interface for writing scoped role assignments.
type ScopedRoleAssignmentWriter interface {
	// CreateScopedRoleAssignment creates a new scoped role assignment.
	CreateScopedRoleAssignment(context.Context, *scopedaccessv1.CreateScopedRoleAssignmentRequest) (*scopedaccessv1.CreateScopedRoleAssignmentResponse, error)

	// DeleteScopedRoleAssignment deletes a scoped role assignment.
	DeleteScopedRoleAssignment(context.Context, *scopedaccessv1.DeleteScopedRoleAssignmentRequest) (*scopedaccessv1.DeleteScopedRoleAssignmentResponse, error)
}
