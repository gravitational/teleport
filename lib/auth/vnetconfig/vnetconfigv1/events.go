/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package vnetconfigv1

import (
	"context"

	"github.com/gravitational/trace"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	libevents "github.com/gravitational/teleport/lib/events"
)

func newCreateAuditEvent(ctx context.Context, status apievents.Status) apievents.AuditEvent {
	return &apievents.VnetConfigCreate{
		Metadata: apievents.Metadata{
			Code: libevents.VnetConfigCreateCode,
			Type: libevents.VnetConfigCreateEvent,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             status,
	}
}

func newUpdateAuditEvent(ctx context.Context, status apievents.Status) apievents.AuditEvent {
	return &apievents.VnetConfigUpdate{
		Metadata: apievents.Metadata{
			Code: libevents.VnetConfigUpdateCode,
			Type: libevents.VnetConfigUpdateEvent,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             status,
	}
}

func newDeleteAuditEvent(ctx context.Context, status apievents.Status) apievents.AuditEvent {
	return &apievents.VnetConfigDelete{
		Metadata: apievents.Metadata{
			Code: libevents.VnetConfigDeleteCode,
			Type: libevents.VnetConfigDeleteEvent,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             status,
	}
}

// emitAuditEvent wraps emitting audit events and logs failures.
func (s *Service) emitAuditEvent(ctx context.Context, evt apievents.AuditEvent) {
	if err := s.emitter.EmitAuditEvent(ctx, evt); err != nil {
		s.logger.ErrorContext(ctx, "Failed to emit audit event",
			"event_code", evt.GetCode(),
			"event_type", evt.GetType(),
			"error", err,
		)
	}
}

func eventStatus(err error) apievents.Status {
	var errorMessage, userMessage string
	if err != nil {
		errorMessage = err.Error()
		userMessage = trace.UserMessage(err)
	}

	return apievents.Status{
		Success:     err == nil,
		Error:       errorMessage,
		UserMessage: userMessage,
	}
}
