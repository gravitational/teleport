// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package healthcheckconfigv1

import (
	"context"

	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
)

func newCreateAuditEvent(ctx context.Context, created *healthcheckconfigv1.HealthCheckConfig) apievents.AuditEvent {
	return &apievents.HealthCheckConfigCreate{
		Metadata: apievents.Metadata{
			Code: events.HealthCheckConfigCreateCode,
			Type: events.HealthCheckConfigCreateEvent,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: created.GetMetadata().GetName(),
		},
	}
}

func newUpdateAuditEvent(ctx context.Context, updated *healthcheckconfigv1.HealthCheckConfig) apievents.AuditEvent {
	return &apievents.HealthCheckConfigUpdate{
		Metadata: apievents.Metadata{
			Code: events.HealthCheckConfigUpdateCode,
			Type: events.HealthCheckConfigUpdateEvent,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: updated.GetMetadata().GetName(),
		},
	}
}

func newDeleteAuditEvent(ctx context.Context, deletedName string) apievents.AuditEvent {
	return &apievents.HealthCheckConfigDelete{
		Metadata: apievents.Metadata{
			Code: events.HealthCheckConfigDeleteCode,
			Type: events.HealthCheckConfigDeleteEvent,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: deletedName,
		},
	}
}
