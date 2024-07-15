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
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

// CreateUser inserts a new user entry in a backend.
// TODO(tross): DELETE IN 17.0.0
// Deprecated: use [usersv1.Service.CreateUser] instead.
func (a *Server) CreateUser(ctx context.Context, user types.User) (types.User, error) {
	if user.GetCreatedBy().IsEmpty() {
		user.SetCreatedBy(types.CreatedBy{
			User: types.UserRef{Name: authz.ClientUsername(ctx)},
			Time: a.GetClock().Now().UTC(),
		})
	}

	created, err := a.Services.CreateUser(ctx, user)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var connectorName string
	if created.GetCreatedBy().Connector == nil {
		connectorName = constants.LocalConnector
	} else {
		connectorName = created.GetCreatedBy().Connector.ID
	}

	if err := a.emitter.EmitAuditEvent(ctx, &apievents.UserCreate{
		Metadata: apievents.Metadata{
			Type: events.UserCreateEvent,
			Code: events.UserCreateCode,
		},
		UserMetadata: authz.ClientUserMetadataWithUser(ctx, created.GetCreatedBy().User.Name),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    created.GetName(),
			Expires: created.Expiry(),
		},
		Connector:          connectorName,
		Roles:              created.GetRoles(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}); err != nil {
		log.WithError(err).Warn("Failed to emit user create event.")
	}

	usagereporter.EmitEditorChangeEvent(created.GetName(), nil, created.GetRoles(), a.AnonymizeAndSubmit)

	return created, nil
}

// UpdateUser updates an existing user in a backend.
// TODO(tross): DELETE IN 17.0.0
// Deprecated: use [usersv1.Service.UpdateUser] instead.
func (a *Server) UpdateUser(ctx context.Context, user types.User) (types.User, error) {
	prevUser, err := a.GetUser(ctx, user.GetName(), false)
	var omitEditorEvent bool
	if err != nil {
		// don't return error here since this call is for event emitting purposes only
		log.WithError(err).Warn("Failed getting user during update")
		omitEditorEvent = true
	}

	// The use of legacyUserUpdater allows the legacy update method to be used without
	// exposing it in auth.ClientI. It really only needs to exist in the services.User interface
	// but if it's added there then it needs to be added everywhere. To reduce confusion and
	// prevent adding a throw away method to the interface the type assertion is leveraged here instead.
	type legacyUserUpdater interface {
		LegacyUpdateUser(ctx context.Context, user types.User) (types.User, error)
	}

	updater, ok := a.Services.Identity.(legacyUserUpdater)
	if !ok {
		log.Warn("Failed to update user via legacy update method. This is a bug!")
		return nil, trace.NotImplemented("legacy user updating is not implemented")
	}

	updated, err := updater.LegacyUpdateUser(ctx, user)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var connectorName string
	if updated.GetCreatedBy().Connector == nil {
		connectorName = constants.LocalConnector
	} else {
		connectorName = updated.GetCreatedBy().Connector.ID
	}

	if err := a.emitter.EmitAuditEvent(ctx, &apievents.UserUpdate{
		Metadata: apievents.Metadata{
			Type: events.UserUpdatedEvent,
			Code: events.UserUpdateCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    updated.GetName(),
			Expires: updated.Expiry(),
		},
		Connector:          connectorName,
		Roles:              updated.GetRoles(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}); err != nil {
		log.WithError(err).Warn("Failed to emit user update event.")
	}

	if !omitEditorEvent {
		usagereporter.EmitEditorChangeEvent(updated.GetName(), prevUser.GetRoles(), updated.GetRoles(), a.AnonymizeAndSubmit)
	}

	return updated, nil
}

// UpsertUser updates a user.
// TODO(tross): DELETE IN 17.0.0
// Deprecated: use [usersv1.Service.UpsertUser] instead.
func (a *Server) UpsertUser(ctx context.Context, user types.User) (types.User, error) {
	prevUser, err := a.GetUser(ctx, user.GetName(), false)
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

	upserted, err := a.Services.UpsertUser(ctx, user)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var connectorName string
	if upserted.GetCreatedBy().Connector == nil {
		connectorName = constants.LocalConnector
	} else {
		connectorName = upserted.GetCreatedBy().Connector.ID
	}

	if err := a.emitter.EmitAuditEvent(a.closeCtx, &apievents.UserCreate{
		Metadata: apievents.Metadata{
			Type: events.UserCreateEvent,
			Code: events.UserCreateCode,
		},
		UserMetadata: apievents.UserMetadata{
			User: upserted.GetName(),
		},
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    upserted.GetName(),
			Expires: upserted.Expiry(),
		},
		Connector:          connectorName,
		Roles:              upserted.GetRoles(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}); err != nil {
		log.WithError(err).Warn("Failed to emit user upsert event.")
	}

	var prevRoles []string
	if prevUser != nil {
		prevRoles = prevUser.GetRoles()
	}
	if !omitEditorEvent {
		usagereporter.EmitEditorChangeEvent(upserted.GetName(), prevRoles, upserted.GetRoles(), a.AnonymizeAndSubmit)
	}

	return upserted, nil
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

	if err := a.emitter.EmitAuditEvent(ctx, &apievents.UserUpdate{
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

// DeleteUser deletes an existing user in a backend by username.
// TODO(tross): DELETE IN 17.0.0
// Deprecated: use [usersv1.Service.DeleteUser] instead.
func (a *Server) DeleteUser(ctx context.Context, user string) error {
	prevUser, err := a.GetUser(ctx, user, false)
	var omitEditorEvent bool
	if err != nil && !trace.IsNotFound(err) {
		// don't return error here, delete may still succeed
		log.WithError(err).Warn("Failed getting user during delete operation")
		prevUser = nil
		omitEditorEvent = true
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
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}); err != nil {
		log.WithError(err).Warn("Failed to emit user delete event.")
	}

	if !omitEditorEvent {
		usagereporter.EmitEditorChangeEvent(user, prevUser.GetRoles(), nil, a.AnonymizeAndSubmit)
	}

	return nil
}
