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
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// CreateUser inserts a new user entry in a backend.
func (s *AuthServer) CreateUser(ctx context.Context, user services.User) error {
	createdBy := user.GetCreatedBy()
	if createdBy.IsEmpty() {
		return trace.BadParameter("created by is not set for new user %q", user.GetName())
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

	if err := s.EmitAuditEvent(events.UserCreate, events.EventFields{
		events.EventUser:     createdBy.User.Name,
		events.UserExpires:   user.Expiry(),
		events.UserRoles:     user.GetRoles(),
		events.FieldName:     user.GetName(),
		events.UserConnector: connectorName,
	}); err != nil {
		log.Warnf("Failed to emit user create event: %v", err)
	}

	return nil
}

// UpdateUser updates an existing user in a backend.
func (s *AuthServer) UpdateUser(ctx context.Context, user services.User) error {
	if err := s.Identity.UpdateUser(ctx, user); err != nil {
		return trace.Wrap(err)
	}

	var connectorName string
	if user.GetCreatedBy().Connector == nil {
		connectorName = teleport.Local
	} else {
		connectorName = user.GetCreatedBy().Connector.ID
	}

	if err := s.EmitAuditEvent(events.UserUpdate, events.EventFields{
		events.EventUser:     clientUsername(ctx),
		events.FieldName:     user.GetName(),
		events.UserExpires:   user.Expiry(),
		events.UserRoles:     user.GetRoles(),
		events.UserConnector: connectorName,
	}); err != nil {
		log.Warnf("Failed to emit user update event: %v", err)
	}

	return nil
}

// UpsertUser updates a user.
func (s *AuthServer) UpsertUser(user services.User) error {
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

	if err := s.EmitAuditEvent(events.UserUpdate, events.EventFields{
		events.EventUser:     user.GetName(),
		events.UserExpires:   user.Expiry(),
		events.UserRoles:     user.GetRoles(),
		events.UserConnector: connectorName,
	}); err != nil {
		log.Warnf("Failed to emit user update event: %v", err)
	}

	return nil
}

// DeleteUser deletes an existng user in a backend by username.
func (s *AuthServer) DeleteUser(ctx context.Context, user string) error {
	role, err := s.Access.GetRole(services.RoleNameForUser(user))
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
	if err := s.EmitAuditEvent(events.UserDelete, events.EventFields{
		events.FieldName: user,
		events.EventUser: clientUsername(ctx),
	}); err != nil {
		log.Warnf("Failed to emit user delete event: %v", err)
	}

	return nil
}
