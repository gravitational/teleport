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
	"sync"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

const (
	unknownName = "unknown"
)

// sharedDirectoryNameMap keeps a mapping of sessionId
// to a shared directory's name, for use by the audit log.
type sharedDirectoryNameMap struct {
	m   map[string]string
	log logrus.FieldLogger
	sync.Mutex
}

func newSharedDirectoryNameMap(log logrus.FieldLogger) *sharedDirectoryNameMap {
	return &sharedDirectoryNameMap{
		m:   make(map[string]string),
		log: log,
	}
}

func (n *sharedDirectoryNameMap) set(sessionID, name string) {
	n.Lock()
	defer n.Unlock()

	n.m[sessionID] = name
}

func (n *sharedDirectoryNameMap) get(sessionID string) string {
	n.Lock()
	defer n.Unlock()

	var directoryName string
	if name, ok := n.m[sessionID]; ok {
		directoryName = name

	} else {
		directoryName = unknownName
		n.log.Warnf("received a SharedDirectoryAcknowledge event for unknown directory in session: %v", sessionID)
	}

	return directoryName
}

func (s *WindowsService) onSessionStart(ctx context.Context, emitter events.Emitter, id *tlsca.Identity, startTime time.Time, windowsUser, sessionID string, desktop types.WindowsDesktop, err error) {
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
		DesktopName:           desktop.GetName(),
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
	s.emit(ctx, emitter, event)
}

func (s *WindowsService) onSessionEnd(ctx context.Context, emitter events.Emitter, id *tlsca.Identity, startedAt time.Time, recorded bool, windowsUser, sessionID string, desktop types.WindowsDesktop) {
	// Clean up the name map if applicable
	s.sdMap.Lock()
	delete(s.sdMap.m, sessionID)
	s.sdMap.Unlock()

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
	s.emit(ctx, emitter, event)
}

func (s *WindowsService) onClipboardSend(ctx context.Context, emitter events.Emitter, id *tlsca.Identity, sessionID string, desktopAddr string, length int32) {
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
	s.emit(ctx, emitter, event)
}

func (s *WindowsService) onClipboardReceive(ctx context.Context, emitter events.Emitter, id *tlsca.Identity, sessionID string, desktopAddr string, length int32) {
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
	s.emit(ctx, emitter, event)
}

// onSharedDirectoryAnnounce does not emit an audit event, but rather registers a directory's
// name in the sharedDirectoryNameMap, which will be used to add audit log entries corresponding
// to later Shared Directory TDP messages.
func (s *WindowsService) onSharedDirectoryAnnounce(sessionID string, m tdp.SharedDirectoryAnnounce) {
	s.sdMap.set(sessionID, m.Name)
}

func (s *WindowsService) onSharedDirectoryAcknowledge(ctx context.Context, emitter events.Emitter, id *tlsca.Identity, sessionID string, desktopAddr string, ack tdp.SharedDirectoryAcknowledge) {
	succeeded := true
	if ack.ErrCode != tdp.ErrCodeNil {
		succeeded = false
	}

	directoryName := s.sdMap.get(sessionID)

	event := &events.DesktopSharedDirectoryStart{
		Metadata: events.Metadata{
			Type:        libevents.DesktopSharedDirectoryStartEvent,
			Code:        libevents.DesktopSharedDirectoryStartCode,
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
		Succeeded:     succeeded,
		DesktopAddr:   desktopAddr,
		DirectoryName: directoryName,
		DirectoryID:   ack.DirectoryID,
	}

	s.emit(ctx, emitter, event)
}

func (s *WindowsService) emit(ctx context.Context, emitter events.Emitter, event events.AuditEvent) {
	if err := emitter.EmitAuditEvent(ctx, event); err != nil {
		s.cfg.Log.WithError(err).Errorf("Failed to emit audit event %v", event)
	}
}
