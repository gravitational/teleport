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
	"fmt"

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	prehogv1alpha "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

// CreateUser inserts a new user entry in a backend.
func (s *Server) CreateUser(ctx context.Context, user types.User) error {
	if user.GetCreatedBy().IsEmpty() {
		user.SetCreatedBy(types.CreatedBy{
			User: types.UserRef{Name: authz.ClientUsername(ctx)},
			Time: s.GetClock().Now().UTC(),
		})
	}

	// TODO: ctx is being swallowed here because the current implementation of
	// s.Uncached.CreateUser is an older implementation that does not curently
	// accept a context.
	if err := s.Services.CreateUser(user); err != nil {
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

	s.emitEditorChangeEvent(user.GetName(), []string{}, user.GetRoles())

	return nil
}

// UpdateUser updates an existing user in a backend.
func (s *Server) UpdateUser(ctx context.Context, user types.User) error {
	prevUser, err := s.GetUser(user.GetName(), false)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := s.Services.UpdateUser(ctx, user); err != nil {
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

	s.emitEditorChangeEvent(user.GetName(), prevUser.GetRoles(), user.GetRoles())

	return nil
}

// UpsertUser updates a user.
func (s *Server) UpsertUser(user types.User) error {
	prevUser, err := s.GetUser(user.GetName(), false)
	if err != nil && !trace.IsNotFound(err) {
		// don't return error here since upsert may still succeed
		log.WithError(err).Warn("Failed getting user during upsert")
	}

	err = s.Services.UpsertUser(user)
	if err != nil {
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

	prevRoles := []string{}
	if prevUser != nil {
		prevRoles = prevUser.GetRoles()
	}
	s.emitEditorChangeEvent(user.GetName(), prevRoles, user.GetRoles())

	return nil
}

// CompareAndSwapUser updates a user but fails if the value on the backend does
// not match the expected value.
func (s *Server) CompareAndSwapUser(ctx context.Context, new, existing types.User) error {
	err := s.Services.CompareAndSwapUser(ctx, new, existing)
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

	s.emitEditorChangeEvent(new.GetName(), existing.GetRoles(), new.GetRoles())

	return nil
}

// DeleteUser deletes an existng user in a backend by username.
func (s *Server) DeleteUser(ctx context.Context, user string) error {
	prevUser, err := s.GetUser(user, false)
	if err != nil && !trace.IsNotFound(err) {
		// don't return error here, upsert may still succeed
		log.WithError(err).Warn("Failed getting user during upsert")
	}

	role, err := s.Services.GetRole(ctx, services.RoleNameForUser(user))
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	} else {
		if err := s.DeleteRole(ctx, role.GetName()); err != nil {
			if !trace.IsNotFound(err) {
				return trace.Wrap(err)
			}
		}
	}

	err = s.Services.DeleteUser(ctx, user)
	if err != nil {
		return trace.Wrap(err)
	}

	// If the user was successfully deleted, emit an event.
	if err := s.emitter.EmitAuditEvent(s.closeCtx, &apievents.UserDelete{
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

	s.emitEditorChangeEvent(user, []string{}, prevUser.GetRoles())

	return nil
}

// emitEditorChangeEvent emits an editor change event if the editor role was added or removed
func (s *Server) emitEditorChangeEvent(username string, prevRoles, newRoles []string) {
	var prevEditor bool
	for _, r := range prevRoles {
		if r == teleport.PresetEditorRoleName {
			prevEditor = true
			break
		}
	}

	var newEditor bool
	for _, r := range newRoles {
		if r == teleport.PresetEditorRoleName {
			newEditor = true
			break
		}
	}

	// don't emit event if editor role wasn't added/removed
	if prevEditor == newEditor {
		return
	}

	eventType := prehogv1alpha.EditorChangeStatus_EDITOR_CHANGE_STATUS_ROLE_GRANTED
	if prevEditor {
		eventType = prehogv1alpha.EditorChangeStatus_EDITOR_CHANGE_STATUS_ROLE_REMOVED
	}

	fmt.Printf("event type: %s\n", eventType)

	s.AnonymizeAndSubmit(&usagereporter.EditorChangeEvent{
		UserName: username,
		Status:   eventType,
	})
}
