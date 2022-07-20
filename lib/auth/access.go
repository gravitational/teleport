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
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
)

// UpsertRole creates or updates a role and emits a related audit event.
func (a *Server) UpsertRole(ctx context.Context, role types.Role) error {
	// check if the update will result in having no roles
	// with access to create or edit roles
	rolesWithPermission, err := a.getRolesWithCreateOrUpdateRolesRule(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if _, ok := rolesWithPermission[role.GetName()]; ok &&
		len(rolesWithPermission) == 1 &&
		!roleHasCreateAndUpdateRolesRule(role) {
		log.Warnf("Failed to update role: role %q is the last role with permissions to create/update roles", role.GetName())
		return trace.BadParameter("failed to update last role with permissions to create/update roles")
	}

	if err := a.Access.UpsertRole(ctx, role); err != nil {
		return trace.Wrap(err)
	}

	if err := a.emitter.EmitAuditEvent(a.closeCtx, &apievents.RoleCreate{
		Metadata: apievents.Metadata{
			Type: events.RoleCreatedEvent,
			Code: events.RoleCreatedCode,
		},
		UserMetadata: ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: role.GetName(),
		},
	}); err != nil {
		log.WithError(err).Warnf("Failed to emit role create event.")
	}
	return nil
}

// DeleteRole deletes a role and emits a related audit event.
func (a *Server) DeleteRole(ctx context.Context, name string) error {
	// check if this role is used by CA or Users
	users, err := a.Services.GetUsers(false)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, u := range users {
		for _, r := range u.GetRoles() {
			if r == name {
				// Mask the actual error here as it could be used to enumerate users
				// within the system.
				log.Warnf("Failed to delete role: role %q is used by user %q.", name, u.GetName())
				return trace.BadParameter("failed to delete role that still in use by a user. Check system server logs for more details.")
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
				log.Warnf("Failed to delete role: role %q is used by user cert authority %q", name, a.GetClusterName())
				return trace.BadParameter("failed to delete role that still in use by a user. Check system server logs for more details.")
			}
		}
	}

	// check if it's the last role with access to create or edit roles
	rolesWithPermission, err := a.getRolesWithCreateOrUpdateRolesRule(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if _, ok := rolesWithPermission[name]; ok && len(rolesWithPermission) == 1 {
		log.Warnf("Failed to delete role: role %q is the last role with permissions to create/update roles", name)
		return trace.BadParameter("failed to delete last role with permissions to create/update roles")
	}

	if err := a.Access.DeleteRole(ctx, name); err != nil {
		return trace.Wrap(err)
	}

	if err := a.emitter.EmitAuditEvent(a.closeCtx, &apievents.RoleDelete{
		Metadata: apievents.Metadata{
			Type: events.RoleDeletedEvent,
			Code: events.RoleDeletedCode,
		},
		UserMetadata: ClientUserMetadata(ctx),
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

	um := ClientUserMetadata(ctx)
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
		UserMetadata: ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: lockName,
		},
	}); err != nil {
		log.WithError(err).Warning("Failed to emit lock delete event.")
	}
	return nil
}

func (a *Server) getRolesWithCreateOrUpdateRolesRule(ctx context.Context) (map[string]struct{}, error) {
	allRoles, err := a.Access.GetRoles(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	roles := make(map[string]struct{})
	for _, role := range allRoles {
		if roleHasCreateAndUpdateRolesRule(role) {
			roles[role.GetName()] = struct{}{}
		}
	}

	return roles, nil
}

func roleHasCreateAndUpdateRolesRule(role types.Role) bool {
	rules := role.GetRules(types.Allow)
	for _, rule := range rules {
		if !rule.HasResource(types.KindRole) {
			continue
		}

		if !rule.HasVerb(types.VerbCreate) || !rule.HasVerb(types.VerbUpdate) {
			continue
		}
		return true

	}
	return false
}
