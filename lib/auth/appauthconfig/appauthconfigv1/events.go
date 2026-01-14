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

package appauthconfigv1

import (
	"context"

	appauthconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/appauthconfig/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
)

func newCreateAuditEvent(ctx context.Context, created *appauthconfigv1.AppAuthConfig) apievents.AuditEvent {
	return &apievents.AppAuthConfigCreate{
		Metadata: apievents.Metadata{
			Code: events.AppAuthConfigCreateCode,
			Type: events.AppAuthConfigCreateEvent,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: created.GetMetadata().GetName(),
		},
	}
}

func newUpdateAuditEvent(ctx context.Context, updated *appauthconfigv1.AppAuthConfig) apievents.AuditEvent {
	return &apievents.AppAuthConfigUpdate{
		Metadata: apievents.Metadata{
			Code: events.AppAuthConfigUpdateCode,
			Type: events.AppAuthConfigUpdateEvent,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: updated.GetMetadata().GetName(),
		},
	}
}

func newDeleteAuditEvent(ctx context.Context, deletedName string) apievents.AuditEvent {
	return &apievents.AppAuthConfigDelete{
		Metadata: apievents.Metadata{
			Code: events.AppAuthConfigDeleteCode,
			Type: events.AppAuthConfigDeleteEvent,
		},
		UserMetadata:       authz.ClientUserMetadata(ctx),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: deletedName,
		},
	}
}

func newVerifyJWTAuditEvent(ctx context.Context, req *appauthconfigv1.CreateAppSessionWithJWTRequest, sid string, err error) apievents.AuditEvent {
	evt := &apievents.AppAuthConfigVerify{
		Metadata: apievents.Metadata{
			Code: events.AppAuthConfigVerifySuccessCode,
			Type: events.AppAuthConfigVerifySuccessEvent,
		},
		AppAuthConfig:   req.ConfigName,
		UserMetadata:    authz.ClientUserMetadata(ctx),
		SessionMetadata: apievents.SessionMetadata{SessionID: sid},
		AppMetadata: apievents.AppMetadata{
			AppName: req.App.AppName,
			AppURI:  req.App.Uri,
		},
		Status: apievents.Status{
			Success: true,
		},
	}

	if err != nil {
		evt.Metadata.Code = events.AppAuthConfigVerifyFailureCode
		evt.Metadata.Type = events.AppAuthConfigVerifyFailureEvent
		evt.Status.Success = false
		evt.Status.Error = err.Error()
	}

	return evt
}
