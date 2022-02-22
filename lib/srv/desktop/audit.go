/*
Copyright 2021 Gravitational, Inc.

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

package desktop

import (
	"context"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/trace"
)

func (s *WindowsService) onSessionStart(ctx context.Context, id *tlsca.Identity, startTime time.Time, windowsUser, sessionID string, desktop types.WindowsDesktop, err error) {
	userMetadata := id.GetUserMetadata()
	userMetadata.Login = windowsUser

	event := &events.WindowsDesktopSessionStart{
		Metadata: events.Metadata{
			Type:        libevents.WindowsDesktopSessionStartEvent,
			Code:        libevents.DesktopSessionStartCode,
			ClusterName: s.clusterName,
			Time:        startTime,
		},
		UserMetadata: userMetadata,
		SessionMetadata: events.SessionMetadata{
			SessionID: sessionID,
			WithMFA:   id.MFAVerified,
		},
		ConnectionMetadata: events.ConnectionMetadata{
			LocalAddr:  id.ClientIP,
			RemoteAddr: desktop.GetAddr(),
			Protocol:   libevents.EventProtocolTDP,
		},
		Status: events.Status{
			Success: err == nil,
		},
		WindowsDesktopService: s.cfg.Heartbeat.HostUUID,
		DesktopAddr:           desktop.GetAddr(),
		Domain:                desktop.GetDomain(),
		WindowsUser:           windowsUser,
		DesktopLabels:         desktop.GetAllLabels(),
	}
	if err != nil {
		event.Code = libevents.DesktopSessionStartFailureCode
		event.Error = trace.Unwrap(err).Error()
		event.UserMessage = err.Error()
	}
	s.emit(ctx, event)
}

func (s *WindowsService) onSessionEnd(ctx context.Context, id *tlsca.Identity, startedAt time.Time, recorded bool, windowsUser, sessionID string, desktop types.WindowsDesktop) {
	userMetadata := id.GetUserMetadata()
	userMetadata.Login = windowsUser

	event := &events.WindowsDesktopSessionEnd{
		Metadata: events.Metadata{
			Type:        libevents.WindowsDesktopSessionEndEvent,
			Code:        libevents.DesktopSessionEndCode,
			ClusterName: s.clusterName,
		},
		UserMetadata: userMetadata,
		SessionMetadata: events.SessionMetadata{
			SessionID: sessionID,
			WithMFA:   id.MFAVerified,
		},
		WindowsDesktopService: s.cfg.Heartbeat.HostUUID,
		DesktopAddr:           desktop.GetAddr(),
		Domain:                desktop.GetDomain(),
		WindowsUser:           windowsUser,
		DesktopLabels:         desktop.GetAllLabels(),
		StartTime:             startedAt,
		EndTime:               s.cfg.Clock.Now().UTC().Round(time.Millisecond),
		DesktopName:           desktop.GetName(),
		Recorded:              recorded,

		// There can only be 1 participant, desktop sessions are not join-able.
		Participants: []string{userMetadata.User},
	}
	s.emit(ctx, event)
}

func (s *WindowsService) onClipboardSend(ctx context.Context, id *tlsca.Identity, sessionID string, desktopAddr string, length int32) {
	event := &events.DesktopClipboardSend{
		Metadata: events.Metadata{
			Type:        libevents.DesktopClipboardSendEvent,
			Code:        libevents.DesktopClipboardSendCode,
			ClusterName: s.clusterName,
			Time:        s.cfg.Clock.Now().UTC(),
		},
		UserMetadata: id.GetUserMetadata(),
		SessionMetadata: events.SessionMetadata{
			SessionID: sessionID,
			WithMFA:   id.MFAVerified,
		},
		ConnectionMetadata: events.ConnectionMetadata{
			LocalAddr:  id.ClientIP,
			RemoteAddr: desktopAddr,
			Protocol:   libevents.EventProtocolTDP,
		},
		DesktopAddr: desktopAddr,
		Length:      length,
	}
	s.emit(ctx, event)
}

func (s *WindowsService) onClipboardReceive(ctx context.Context, id *tlsca.Identity, sessionID string, desktopAddr string, length int32) {
	event := &events.DesktopClipboardReceive{
		Metadata: events.Metadata{
			Type:        libevents.DesktopClipboardReceiveEvent,
			Code:        libevents.DesktopClipboardReceiveCode,
			ClusterName: s.clusterName,
			Time:        s.cfg.Clock.Now().UTC(),
		},
		UserMetadata: id.GetUserMetadata(),
		SessionMetadata: events.SessionMetadata{
			SessionID: sessionID,
			WithMFA:   id.MFAVerified,
		},
		ConnectionMetadata: events.ConnectionMetadata{
			LocalAddr:  id.ClientIP,
			RemoteAddr: desktopAddr,
			Protocol:   libevents.EventProtocolTDP,
		},
		DesktopAddr: desktopAddr,
		Length:      length,
	}
	s.emit(ctx, event)
}

func (s *WindowsService) emit(ctx context.Context, event events.AuditEvent) {
	if err := s.cfg.Emitter.EmitAuditEvent(ctx, event); err != nil {
		s.cfg.Log.WithError(err).Errorf("Failed to emit audit event %v", event)
	}
}
