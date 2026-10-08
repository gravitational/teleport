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
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

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
		a.logger.WarnContext(ctx, "Failed to emit user update event", "error", err)
	}

	usagereporter.EmitEditorChangeEvent(new.GetName(), existing.GetRoles(), new.GetRoles(), a.AnonymizeAndSubmit)

	return nil
}

// SSOUserExpiry returns the expiry of an SSO user that logs in again at now.
// The result is existingExpiry, but at least now plus ttl and at most now
// plus maxTTL. maxTTL must be at least ttl or zero. If maxTTL is zero, the
// result is now plus ttl.
func SSOUserExpiry(existingExpiry, now time.Time, ttl, maxTTL time.Duration) time.Time {
	sessionExpiry := now.Add(ttl)
	maxExpiry := now.Add(max(ttl, maxTTL))
	if maxExpiry.Before(existingExpiry) {
		return maxExpiry
	}
	if sessionExpiry.After(existingExpiry) {
		return sessionExpiry
	}
	return existingExpiry
}
