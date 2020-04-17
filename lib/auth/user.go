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
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// CreateUser inserts a user and emits a UserCreated event
func (s *AuthServer) CreateUser(user services.User) error {
	err := s.Identity.CreateUser(user)
	if err != nil {
		return trace.Wrap(err)
	}

	s.EmitAuditEvent(events.UserCreate, events.EventFields{
		events.EventUser:     user.GetName(),
		events.UserExpires:   user.Expiry(),
		events.UserRoles:     user.GetRoles(),
		events.UserConnector: getUserAuthenticationConnector(user),
	})

	return nil
}

// UpsertUser upserts user
func (s *AuthServer) UpsertUser(user services.User) error {
	err := s.Identity.UpsertUser(user)
	if err != nil {
		return trace.Wrap(err)
	}

	s.EmitAuditEvent(events.UserUpdate, events.EventFields{
		events.EventUser:     user.GetName(),
		events.UserExpires:   user.Expiry(),
		events.UserRoles:     user.GetRoles(),
		events.UserConnector: getUserAuthenticationConnector(user),
	})

	return nil
}

// DeleteUser deletes user
func (s *AuthServer) DeleteUser(user string) error {
	role, err := s.Access.GetRole(services.RoleNameForUser(user))
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	} else {
		if err := s.Access.DeleteRole(role.GetName()); err != nil {
			if !trace.IsNotFound(err) {
				return trace.Wrap(err)
			}
		}
	}

	err = s.Identity.DeleteUser(user)
	if err != nil {
		return trace.Wrap(err)
	}

	// If the user was successfully deleted, emit an event.
	s.EmitAuditEvent(events.UserDelete, events.EventFields{
		events.EventUser: user,
	})

	return nil
}

// getUserAuthenticationConnector returns the type of connector
// users authenticate by when logging in.
func getUserAuthenticationConnector(user services.User) string {
	if user == nil {
		return ""
	}

	var connectorName string
	if user.GetCreatedBy().Connector == nil {
		connectorName = teleport.Local
	} else {
		connectorName = user.GetCreatedBy().Connector.ID
	}

	return connectorName
}
