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

package auth

import (
	"context"
	"slices"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
)

// CreateRole creates a role and emits a related audit event.
func (a *Server) CreateRole(ctx context.Context, role types.Role) (types.Role, error) {
	created, err := a.Services.CreateRole(ctx, role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.emitter.EmitAuditEvent(a.closeCtx, &apievents.RoleCreate{
		Metadata: apievents.Metadata{
			Type: events.RoleCreatedEvent,
			Code: events.RoleCreatedCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: role.GetName(),
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}); err != nil {
		a.logger.WarnContext(ctx, "Failed to emit role create event.", "error", err)
	}
	return created, nil
}

// UpdateRole updates a role and emits a related audit event.
func (a *Server) UpdateRole(ctx context.Context, role types.Role) (types.Role, error) {
	created, err := a.Services.UpdateRole(ctx, role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.emitter.EmitAuditEvent(a.closeCtx, &apievents.RoleUpdate{
		Metadata: apievents.Metadata{
			Type: events.RoleUpdatedEvent,
			Code: events.RoleUpdatedCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: role.GetName(),
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}); err != nil {
		a.logger.WarnContext(ctx, "Failed to emit role update event.", "error", err)
	}
	return created, nil
}

// UpsertRole creates or updates a role and emits a related audit event.
func (a *Server) UpsertRole(ctx context.Context, role types.Role) (types.Role, error) {
	upserted, err := a.Services.UpsertRole(ctx, role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.emitter.EmitAuditEvent(a.closeCtx, &apievents.RoleCreate{
		Metadata: apievents.Metadata{
			Type: events.RoleCreatedEvent,
			Code: events.RoleCreatedCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: role.GetName(),
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}); err != nil {
		a.logger.WarnContext(ctx, "Failed to emit role create event.", "error", err)
	}
	return upserted, nil
}

// checkRoleInUse checks if a role is currently in use by users, certificate authorities,
// or access lists. Returns an error if the role is in use.
func (a *Server) checkRoleInUse(ctx context.Context, name string) error {
	// check if this role is used by CA or Users
	users, err := a.Services.GetUsers(ctx, false)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, u := range users {
		if slices.Contains(u.GetRoles(), name) {
			// Mask the actual error here as it could be used to enumerate users
			// within the system.
			a.logger.WarnContext(
				ctx, "Failed to delete role: role is still in use by a user",
				"role", name, "user", u.GetName(),
			)
			return trace.Wrap(errDeleteRoleUser)
		}
	}
	// check if it's used by some external cert authorities, e.g.
	// cert authorities related to external cluster
	cas, err := a.Services.GetCertAuthorities(ctx, types.UserCA, false)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, ca := range cas {
		if slices.Contains(ca.GetRoles(), name) {
			// Mask the actual error here as it could be used to enumerate users
			// within the system.
			a.logger.WarnContext(
				ctx, "Failed to delete role: role is still in use by a user cert authority",
				"role", name, "ca", ca.GetClusterName(),
			)
			return trace.Wrap(errDeleteRoleCA)
		}
	}

	for accessList, err := range clientutils.Resources(ctx, a.Services.AccessListsInternal.ListAccessLists) {
		if err != nil {
			return trace.Wrap(err)
		}
		var usedIn []string
		if slices.Contains(accessList.Spec.Grants.Roles, name) {
			usedIn = append(usedIn, "spec.grants.roles")
		}
		if slices.Contains(accessList.Spec.MembershipRequires.Roles, name) {
			usedIn = append(usedIn, "spec.membership_requires.roles")
		}
		if slices.Contains(accessList.Spec.OwnershipRequires.Roles, name) {
			usedIn = append(usedIn, "spec.ownership_requires.roles")
		}
		if len(usedIn) > 0 {
			a.logger.WarnContext(
				ctx,
				"Failed to delete role: role is referenced by access list",
				"role", name,
				"access_list_name", accessList.GetName(),
				"access_list_title", accessList.Spec.Title,
				"used_in", usedIn,
			)
			return newRoleInUseError(
				"cannot delete role %q: role is referenced by access list %q (%s) in fields: %v "+
					"Remove the role from the access list before deleting it",
				name, accessList.GetName(), accessList.Spec.Title, usedIn)
		}
	}

	return nil
}

// DeleteRole deletes a role and emits a related audit event.
func (a *Server) DeleteRole(ctx context.Context, name string) error {
	if err := a.checkRoleInUse(ctx, name); err != nil {
		return trace.Wrap(err)
	}

	if err := a.Services.DeleteRole(ctx, name); err != nil {
		return trace.Wrap(err)
	}

	if err := a.emitter.EmitAuditEvent(a.closeCtx, &apievents.RoleDelete{
		Metadata: apievents.Metadata{
			Type: events.RoleDeletedEvent,
			Code: events.RoleDeletedCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: name,
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}); err != nil {
		a.logger.WarnContext(ctx, "Failed to emit role delete event", "error", err)
	}
	return nil
}

// UpsertLock upserts a lock and emits a related audit event.
func (a *Server) UpsertLock(ctx context.Context, lock types.Lock) error {
	if err := a.Services.UpsertLock(ctx, lock); err != nil {
		return trace.Wrap(err)
	}

	var expiresTime time.Time
	// leave as 0 if no lock expiration was set
	if le := lock.LockExpiry(); le != nil {
		expiresTime = le.UTC()
	}
	um := authz.ClientUserMetadata(ctx)
	if err := a.emitter.EmitAuditEvent(a.closeCtx, &apievents.LockCreate{
		Metadata: apievents.Metadata{
			Type: events.LockCreatedEvent,
			Code: events.LockCreatedCode,
		},
		UserMetadata: um,
		ResourceMetadata: apievents.ResourceMetadata{
			Name:      lock.GetName(),
			Expires:   expiresTime,
			UpdatedBy: um.User,
		},
		Target: lock.Target(),
		Lock: apievents.LockMetadata{
			Target: lock.Target(),
		},
	}); err != nil {
		a.logger.WarnContext(ctx, "Failed to emit lock create event.", "error", err)
	}
	return nil
}

// DeleteLock deletes a lock and emits a related audit event.
func (a *Server) DeleteLock(ctx context.Context, lockName string) error {
	lock, err := a.Services.GetLock(ctx, lockName)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := a.Services.DeleteLock(ctx, lockName); err != nil {
		return trace.Wrap(err)
	}

	if err := a.emitter.EmitAuditEvent(a.closeCtx, &apievents.LockDelete{
		Metadata: apievents.Metadata{
			Type: events.LockDeletedEvent,
			Code: events.LockDeletedCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: lockName,
		},
		Lock: apievents.LockMetadata{
			Target: lock.Target(),
		},
	}); err != nil {
		a.logger.WarnContext(ctx, "Failed to emit lock delete event.", "error", err)
	}
	return nil
}
