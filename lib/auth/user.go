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
package auth

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

// CreateUser inserts a new user entry in a backend.
func (a *Server) CreateUser(ctx context.Context, user types.User) error {
	if user.GetCreatedBy().IsEmpty() {
		user.SetCreatedBy(types.CreatedBy{
			User: types.UserRef{Name: authz.ClientUsername(ctx)},
			Time: a.GetClock().Now().UTC(),
		})
	}

	// TODO: ctx is being swallowed here because the current implementation of
	// s.Uncached.CreateUser is an older implementation that does not curently
	// accept a context.
	if err := a.Services.CreateUser(user); err != nil {
		return trace.Wrap(err)
	}

	var connectorName string
	if user.GetCreatedBy().Connector == nil {
		connectorName = constants.LocalConnector
	} else {
		connectorName = user.GetCreatedBy().Connector.ID
	}

	if err := a.emitter.EmitAuditEvent(ctx, &apievents.UserCreate{
		Metadata: apievents.Metadata{
			Type: events.UserCreateEvent,
			Code: events.UserCreateCode,
		},
		UserMetadata: authz.ClientUserMetadataWithUser(ctx, user.GetCreatedBy().User.Name),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    user.GetName(),
			Expires: user.Expiry(),
		},
		Connector: connectorName,
		Roles:     user.GetRoles(),
	}); err != nil {
		log.WithError(err).Warn("Failed to emit user create event.")
	}

	usagereporter.EmitEditorChangeEvent(user.GetName(), nil, user.GetRoles(), a.AnonymizeAndSubmit)

	return nil
}

// UpdateUser updates an existing user in a backend.
func (a *Server) UpdateUser(ctx context.Context, user types.User) error {
	prevUser, err := a.GetUser(user.GetName(), false)
	var omitEditorEvent bool
	if err != nil {
		// don't return error here since this call is for event emitting purposes only
		log.WithError(err).Warn("Failed getting user during update")
		omitEditorEvent = true
	}

	if err := a.Services.UpdateUser(ctx, user); err != nil {
		return trace.Wrap(err)
	}

	var connectorName string
	if user.GetCreatedBy().Connector == nil {
		connectorName = constants.LocalConnector
	} else {
		connectorName = user.GetCreatedBy().Connector.ID
	}

	if err := a.emitter.EmitAuditEvent(ctx, &apievents.UserCreate{
		Metadata: apievents.Metadata{
			Type: events.UserUpdatedEvent,
			Code: events.UserUpdateCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    user.GetName(),
			Expires: user.Expiry(),
		},
		Connector: connectorName,
		Roles:     user.GetRoles(),
	}); err != nil {
		log.WithError(err).Warn("Failed to emit user update event.")
	}

	if !omitEditorEvent {
		usagereporter.EmitEditorChangeEvent(user.GetName(), prevUser.GetRoles(), user.GetRoles(), a.AnonymizeAndSubmit)
	}

	return nil
}

// UpsertUser updates a user.
func (a *Server) UpsertUser(user types.User) error {
	prevUser, err := a.GetUser(user.GetName(), false)
	var omitEditorEvent bool
	if err != nil {
		if trace.IsNotFound(err) {
			prevUser = nil
		} else {
			// don't return error here since upsert may still succeed, just omit the event
			log.WithError(err).Warn("Failed getting user during upsert")
			omitEditorEvent = true
		}
	}

	err = a.Services.UpsertUser(user)
	if err != nil {
		return trace.Wrap(err)
	}

	var connectorName string
	if user.GetCreatedBy().Connector == nil {
		connectorName = constants.LocalConnector
	} else {
		connectorName = user.GetCreatedBy().Connector.ID
	}

	if err := a.emitter.EmitAuditEvent(a.closeCtx, &apievents.UserCreate{
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

	var prevRoles []string
	if prevUser != nil {
		prevRoles = prevUser.GetRoles()
	}
	if !omitEditorEvent {
		usagereporter.EmitEditorChangeEvent(user.GetName(), prevRoles, user.GetRoles(), a.AnonymizeAndSubmit)
	}

	return nil
}

// CompareAndSwapUser updates a user but fails if the value on the backend does
// not match the expected value.
func (a *Server) CompareAndSwapUser(ctx context.Context, new, existing types.User) error {
	err := a.Services.CompareAndSwapUser(ctx, new, existing)
	if err != nil {
		return trace.Wrap(err)
	}

	var connectorName string
	if new.GetCreatedBy().Connector == nil {
		connectorName = constants.LocalConnector
	} else {
		connectorName = new.GetCreatedBy().Connector.ID
	}

	if err := a.emitter.EmitAuditEvent(ctx, &apievents.UserCreate{
		Metadata: apievents.Metadata{
			Type: events.UserUpdatedEvent,
			Code: events.UserUpdateCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    new.GetName(),
			Expires: new.Expiry(),
		},
		Connector: connectorName,
		Roles:     new.GetRoles(),
	}); err != nil {
		log.WithError(err).Warn("Failed to emit user update event.")
	}

	usagereporter.EmitEditorChangeEvent(new.GetName(), existing.GetRoles(), new.GetRoles(), a.AnonymizeAndSubmit)

	return nil
}

// DeleteUser deletes an existng user in a backend by username.
func (a *Server) DeleteUser(ctx context.Context, user string) error {
	prevUser, err := a.GetUser(user, false)
	var omitEditorEvent bool
	if err != nil && !trace.IsNotFound(err) {
		// don't return error here, delete may still succeed
		log.WithError(err).Warn("Failed getting user during delete operation")
		prevUser = nil
		omitEditorEvent = true
	}

	role, err := a.Services.GetRole(ctx, services.RoleNameForUser(user))
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	} else {
		if err := a.DeleteRole(ctx, role.GetName()); err != nil {
			if !trace.IsNotFound(err) {
				return trace.Wrap(err)
			}
		}
	}

	err = a.Services.DeleteUser(ctx, user)
	if err != nil {
		return trace.Wrap(err)
	}

	// If the user was successfully deleted, emit an event.
	if err := a.emitter.EmitAuditEvent(a.closeCtx, &apievents.UserDelete{
		Metadata: apievents.Metadata{
			Type: events.UserDeleteEvent,
			Code: events.UserDeleteCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: user,
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit user delete event.")
	}

	if !omitEditorEvent {
		usagereporter.EmitEditorChangeEvent(user, prevUser.GetRoles(), nil, a.AnonymizeAndSubmit)
	}

	return nil
}
