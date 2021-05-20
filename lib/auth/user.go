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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// CreateUser inserts a new user entry in a backend.
func (s *Server) CreateUser(ctx context.Context, user services.User) error {
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
		connectorName = teleport.Local
	} else {
		connectorName = user.GetCreatedBy().Connector.ID
	}

	if err := s.emitter.EmitAuditEvent(ctx, &events.UserCreate{
		Metadata: events.Metadata{
			Type: events.UserCreateEvent,
			Code: events.UserCreateCode,
		},
		UserMetadata: events.UserMetadata{
			User:         user.GetCreatedBy().User.Name,
			Impersonator: ClientImpersonator(ctx),
		},
		ResourceMetadata: events.ResourceMetadata{
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
func (s *Server) UpdateUser(ctx context.Context, user services.User) error {
	if err := s.Identity.UpdateUser(ctx, user); err != nil {
		return trace.Wrap(err)
	}

	var connectorName string
	if user.GetCreatedBy().Connector == nil {
		connectorName = teleport.Local
	} else {
		connectorName = user.GetCreatedBy().Connector.ID
	}

	if err := s.emitter.EmitAuditEvent(ctx, &events.UserCreate{
		Metadata: events.Metadata{
			Type: events.UserUpdatedEvent,
			Code: events.UserUpdateCode,
		},
		UserMetadata: events.UserMetadata{
			User:         ClientUsername(ctx),
			Impersonator: ClientImpersonator(ctx),
		},
		ResourceMetadata: events.ResourceMetadata{
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
func (s *Server) UpsertUser(user services.User) error {
	err := s.Identity.UpsertUser(user)
	if err != nil {
		return trace.Wrap(err)
	}

	var connectorName string
	if user.GetCreatedBy().Connector == nil {
		connectorName = teleport.Local
	} else {
		connectorName = user.GetCreatedBy().Connector.ID
	}

	if err := s.emitter.EmitAuditEvent(s.closeCtx, &events.UserCreate{
		Metadata: events.Metadata{
			Type: events.UserCreateEvent,
			Code: events.UserCreateCode,
		},
		UserMetadata: events.UserMetadata{
			User: user.GetName(),
		},
		ResourceMetadata: events.ResourceMetadata{
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

// DeleteUser deletes an existng user in a backend by username.
func (s *Server) DeleteUser(ctx context.Context, user string) error {
	role, err := s.Access.GetRole(ctx, services.RoleNameForUser(user))
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

	err = s.Identity.DeleteUser(ctx, user)
	if err != nil {
		return trace.Wrap(err)
	}

	// If the user was successfully deleted, emit an event.
	if err := s.emitter.EmitAuditEvent(s.closeCtx, &events.UserDelete{
		Metadata: events.Metadata{
			Type: events.UserDeleteEvent,
			Code: events.UserDeleteCode,
		},
		UserMetadata: events.UserMetadata{
			User:         ClientUsername(ctx),
			Impersonator: ClientImpersonator(ctx),
		},
		ResourceMetadata: events.ResourceMetadata{
			Name: user,
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit user delete event.")
	}

	return nil
}
