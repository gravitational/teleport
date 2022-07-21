/*
Copyright 2015 Gravitational, Inc.

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

// Package auth implements certificate signing authority and access control server
// Authority server is composed of several parts:
//
// * Authority server itself that implements signing and acl logic
// * HTTP server wrapper for authority server
// * HTTP client wrapper
//
package auth

import (
	"context"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// CreateUser inserts a new user entry in a backend.
func (s *Server) CreateUser(ctx context.Context, user types.User) error {
	if user.GetCreatedBy().IsEmpty() {
		user.SetCreatedBy(types.CreatedBy{
			User: types.UserRef{Name: ClientUsername(ctx)},
			Time: s.GetClock().Now().UTC(),
		})
	}

	// TODO: ctx is being swallowed here because the current implementation of
	// s.Identity.CreateUser is an older implementation that does not curently
	// accept a context.
	if err := s.Identity.CreateUser(user); err != nil {
		return trace.Wrap(err)
	}

	var connectorName string
	if user.GetCreatedBy().Connector == nil {
		connectorName = constants.LocalConnector
	} else {
		connectorName = user.GetCreatedBy().Connector.ID
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.UserCreate{
		Metadata: apievents.Metadata{
			Type: events.UserCreateEvent,
			Code: events.UserCreateCode,
		},
		UserMetadata: ClientUserMetadataWithUser(ctx, user.GetCreatedBy().User.Name),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    user.GetName(),
			Expires: user.Expiry(),
		},
		Connector: connectorName,
		Roles:     user.GetRoles(),
	}); err != nil {
		log.WithError(err).Warn("Failed to emit user create event.")
	}

	return nil
}

// UpdateUser updates an existing user in a backend.
func (s *Server) UpdateUser(ctx context.Context, user types.User) error {
	if err := s.checkUserRoleConstraint(ctx, user, "update"); err != nil {
		return trace.Wrap(err)
	}

	if err := s.Identity.UpdateUser(ctx, user); err != nil {
		return trace.Wrap(err)
	}

	var connectorName string
	if user.GetCreatedBy().Connector == nil {
		connectorName = constants.LocalConnector
	} else {
		connectorName = user.GetCreatedBy().Connector.ID
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.UserCreate{
		Metadata: apievents.Metadata{
			Type: events.UserUpdatedEvent,
			Code: events.UserUpdateCode,
		},
		UserMetadata: ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    user.GetName(),
			Expires: user.Expiry(),
		},
		Connector: connectorName,
		Roles:     user.GetRoles(),
	}); err != nil {
		log.WithError(err).Warn("Failed to emit user update event.")
	}

	return nil
}

// UpsertUser updates a user.
func (s *Server) UpsertUser(user types.User) error {
	if err := s.checkUserRoleConstraint(s.CloseContext(), user, "upsert"); err != nil {
		return trace.Wrap(err)
	}

	if err := s.Identity.UpsertUser(user); err != nil {
		return trace.Wrap(err)
	}

	var connectorName string
	if user.GetCreatedBy().Connector == nil {
		connectorName = constants.LocalConnector
	} else {
		connectorName = user.GetCreatedBy().Connector.ID
	}

	if err := s.emitter.EmitAuditEvent(s.closeCtx, &apievents.UserCreate{
		Metadata: apievents.Metadata{
			Type: events.UserCreateEvent,
			Code: events.UserCreateCode,
		},
		UserMetadata: apievents.UserMetadata{
			User: user.GetName(),
		},
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    user.GetName(),
			Expires: user.Expiry(),
		},
		Connector: connectorName,
		Roles:     user.GetRoles(),
	}); err != nil {
		log.WithError(err).Warn("Failed to emit user upsert event.")
	}

	return nil
}

// CompareAndSwapUser updates a user but fails if the value on the backend does
// not match the expected value.
func (s *Server) CompareAndSwapUser(ctx context.Context, new, existing types.User) error {
	err := s.Identity.CompareAndSwapUser(ctx, new, existing)
	if err != nil {
		return trace.Wrap(err)
	}

	var connectorName string
	if new.GetCreatedBy().Connector == nil {
		connectorName = constants.LocalConnector
	} else {
		connectorName = new.GetCreatedBy().Connector.ID
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.UserCreate{
		Metadata: apievents.Metadata{
			Type: events.UserUpdatedEvent,
			Code: events.UserUpdateCode,
		},
		UserMetadata: apievents.UserMetadata{
			User:         ClientUsername(ctx),
			Impersonator: ClientImpersonator(ctx),
		},
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    new.GetName(),
			Expires: new.Expiry(),
		},
		Connector: connectorName,
		Roles:     new.GetRoles(),
	}); err != nil {
		log.WithError(err).Warn("Failed to emit user update event.")
	}

	return nil
}

// DeleteUser deletes an existng user in a backend by username.
func (s *Server) DeleteUser(ctx context.Context, username string) error {
	user, err := types.NewUser(username)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := s.checkUserRoleConstraint(ctx, user, "delete"); err != nil {
		return trace.Wrap(err)
	}

	role, err := s.Access.GetRole(ctx, services.RoleNameForUser(username))
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	} else {
		if err := s.Access.DeleteRole(ctx, role.GetName()); err != nil {
			if !trace.IsNotFound(err) {
				return trace.Wrap(err)
			}
		}
	}

	if err = s.Identity.DeleteUser(ctx, username); err != nil {
		return trace.Wrap(err)
	}

	// If the user was successfully deleted, emit an event.
	if err := s.emitter.EmitAuditEvent(s.closeCtx, &apievents.UserDelete{
		Metadata: apievents.Metadata{
			Type: events.UserDeleteEvent,
			Code: events.UserDeleteCode,
		},
		UserMetadata: ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: username,
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit user delete event.")
	}

	return nil
}

// checkUserRoleConstraint checks if the request will result in having
// no users with access to upsert roles.
func (s *Server) checkUserRoleConstraint(ctx context.Context, user types.User, request string) error {
	rolesWithUpsertRolesRule, err := s.getRolesWithUpsertRolesRule(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	allUsers, err := s.Identity.GetUsers(false)
	if err != nil {
		return trace.Wrap(err)
	}

	matchedUsers := make(map[string]struct{})
	for _, u := range allUsers {
		for _, r := range u.GetRoles() {
			if _, ok := rolesWithUpsertRolesRule[r]; ok {
				matchedUsers[u.GetName()] = struct{}{}
				break
			}
		}
	}

	if _, ok := matchedUsers[user.GetName()]; ok && len(matchedUsers) == 1 {
		for _, r := range user.GetRoles() {
			if _, ok := rolesWithUpsertRolesRule[r]; ok {
				return nil
			}
		}
		log.Warnf("Failed to %s last user with with permissions to upsert roles", request)
		return trace.BadParameter("failed to %s last user with permissions to upsert roles", request)
	}

	return nil
}
