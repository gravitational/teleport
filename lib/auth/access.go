/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package auth

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	apievents "github.com/gravitational/teleport/api/types/events"
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
	}); err != nil {
		log.WithError(err).Warnf("Failed to emit role create event.")
	}
	return created, nil
}

// UpdateRole updates a role and emits a related audit event.
func (a *Server) UpdateRole(ctx context.Context, role types.Role) (types.Role, error) {
	created, err := a.Services.UpdateRole(ctx, role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(tross): add a RoleUpdate type, RoleUpdatedEvent/Code for metadata
	// and convert this to use them instead of a create event. As is this matches
	// existing behavior since all updates to a role were done vie upsert which
	// only ever emits a create event.
	if err := a.emitter.EmitAuditEvent(a.closeCtx, &apievents.RoleCreate{
		Metadata: apievents.Metadata{
			Type: events.RoleCreatedEvent,
			Code: events.RoleCreatedCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: role.GetName(),
		},
	}); err != nil {
		log.WithError(err).Warnf("Failed to emit role create event.")
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
	}); err != nil {
		log.WithError(err).Warnf("Failed to emit role create event.")
	}
	return upserted, nil
}

var (
	errDeleteRoleUser       = trace.BadParameter("failed to delete a role that is still in use by a user, check the system server logs for more details")
	errDeleteRoleCA         = trace.BadParameter("failed to delete a role that is still in use by a certificate authority, check the system server logs for more details")
	errDeleteRoleAccessList = trace.BadParameter("failed to delete a role that is still in use by an access list, check the system server logs for more details")
)

// DeleteRole deletes a role and emits a related audit event.
func (a *Server) DeleteRole(ctx context.Context, name string) error {
	// check if this role is used by CA or Users
	users, err := a.Services.GetUsers(ctx, false)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, u := range users {
		for _, r := range u.GetRoles() {
			if r == name {
				// Mask the actual error here as it could be used to enumerate users
				// within the system.
				log.Warnf("Failed to delete role: role %v is used by user %v.", name, u.GetName())
				return trace.Wrap(errDeleteRoleUser)
			}
		}
	}
	// check if it's used by some external cert authorities, e.g.
	// cert authorities related to external cluster
	cas, err := a.Services.GetCertAuthorities(ctx, types.UserCA, false)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, a := range cas {
		for _, r := range a.GetRoles() {
			if r == name {
				// Mask the actual error here as it could be used to enumerate users
				// within the system.
				log.Warnf("Failed to delete role: role %v is used by user cert authority %v", name, a.GetClusterName())
				return trace.Wrap(errDeleteRoleCA)
			}
		}
	}

	var nextToken string
	for {
		var accessLists []*accesslist.AccessList
		var err error
		accessLists, nextToken, err = a.Services.AccessListClient().ListAccessLists(ctx, 0 /* default page size */, nextToken)
		if err != nil {
			return trace.Wrap(err)
		}

		for _, accessList := range accessLists {
			for _, r := range accessList.Spec.Grants.Roles {
				if r == name {
					log.Warnf("Failed to delete role: role %v is granted by access list %s", name, accessList.GetName())
					return trace.Wrap(errDeleteRoleAccessList)
				}
			}

			for _, r := range accessList.Spec.MembershipRequires.Roles {
				if r == name {
					log.Warnf("Failed to delete role: role %v is required by members of access list %s", name, accessList.GetName())
					return trace.Wrap(errDeleteRoleAccessList)
				}
			}

			for _, r := range accessList.Spec.OwnershipRequires.Roles {
				if r == name {
					log.Warnf("Failed to delete role: role %v is required by owners of access list %s", name, accessList.GetName())
					return trace.Wrap(errDeleteRoleAccessList)
				}
			}
		}

		if nextToken == "" {
			break
		}
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
	}); err != nil {
		log.WithError(err).Warnf("Failed to emit role delete event.")
	}
	return nil
}

// UpsertLock upserts a lock and emits a related audit event.
func (a *Server) UpsertLock(ctx context.Context, lock types.Lock) error {
	if err := a.Services.UpsertLock(ctx, lock); err != nil {
		return trace.Wrap(err)
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
			UpdatedBy: um.User,
		},
		Target: lock.Target(),
	}); err != nil {
		log.WithError(err).Warning("Failed to emit lock create event.")
	}
	return nil
}

// DeleteLock deletes a lock and emits a related audit event.
func (a *Server) DeleteLock(ctx context.Context, lockName string) error {
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
	}); err != nil {
		log.WithError(err).Warning("Failed to emit lock delete event.")
	}
	return nil
}
