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

package desktop

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/tlsca"
)

// desktopSessionAuditor is used to build session-related events
// which are emitted to Teleport's audit log
type desktopSessionAuditor struct {
	clock clockwork.Clock

	sessionID   string
	identity    *tlsca.Identity
	windowsUser string
	desktop     types.WindowsDesktop

	startTime          time.Time
	clusterName        string
	desktopServiceUUID string

	auditCache sharedDirectoryAuditCache
}

func (d *desktopSessionAuditor) getSessionMetadata() events.SessionMetadata {
	return events.SessionMetadata{
		SessionID:        d.sessionID,
		WithMFA:          d.identity.MFAVerified,
		PrivateKeyPolicy: string(d.identity.PrivateKeyPolicy),
	}
}

func (d *desktopSessionAuditor) getConnectionMetadata() events.ConnectionMetadata {
	return events.ConnectionMetadata{
		LocalAddr:  d.identity.LoginIP,
		RemoteAddr: d.desktop.GetAddr(),
		Protocol:   libevents.EventProtocolTDP,
	}
}

func (s *WindowsService) newSessionAuditor(
	sessionID string,
	identity *tlsca.Identity,
	windowsUser string,
	desktop types.WindowsDesktop,
) *desktopSessionAuditor {
	return &desktopSessionAuditor{
		clock: s.cfg.Clock,

		sessionID:   sessionID,
		identity:    identity,
		windowsUser: windowsUser,
		desktop:     desktop,

		startTime:          s.cfg.Clock.Now().UTC().Round(time.Millisecond),
		clusterName:        s.clusterName,
		desktopServiceUUID: s.cfg.Heartbeat.HostUUID,

		auditCache: newSharedDirectoryAuditCache(),
	}
}

func (d *desktopSessionAuditor) makeSessionStart(err error) *events.WindowsDesktopSessionStart {
	userMetadata := d.identity.GetUserMetadata()
	userMetadata.Login = d.windowsUser

	event := &events.WindowsDesktopSessionStart{
		Metadata: events.Metadata{
			Type:        libevents.WindowsDesktopSessionStartEvent,
			Code:        libevents.DesktopSessionStartCode,
			ClusterName: d.clusterName,
			Time:        d.startTime,
		},
		UserMetadata:          userMetadata,
		SessionMetadata:       d.getSessionMetadata(),
		ConnectionMetadata:    d.getConnectionMetadata(),
		Status:                events.Status{Success: err == nil},
		WindowsDesktopService: d.desktopServiceUUID,
		DesktopName:           d.desktop.GetName(),
		DesktopAddr:           d.desktop.GetAddr(),
		Domain:                d.desktop.GetDomain(),
		WindowsUser:           d.windowsUser,
		DesktopLabels:         d.desktop.GetAllLabels(),
	}

	if err != nil {
		event.Code = libevents.DesktopSessionStartFailureCode
		event.Error = trace.Unwrap(err).Error()
		event.UserMessage = err.Error()
	}

	return event
}

func (d *desktopSessionAuditor) makeSessionEnd(recorded bool) *events.WindowsDesktopSessionEnd {
	userMetadata := d.identity.GetUserMetadata()
	userMetadata.Login = d.windowsUser

	return &events.WindowsDesktopSessionEnd{
		Metadata: events.Metadata{
			Type:        libevents.WindowsDesktopSessionEndEvent,
			Code:        libevents.DesktopSessionEndCode,
			ClusterName: d.clusterName,
		},
		UserMetadata:          userMetadata,
		SessionMetadata:       d.getSessionMetadata(),
		WindowsDesktopService: d.desktopServiceUUID,
		DesktopAddr:           d.desktop.GetAddr(),
		Domain:                d.desktop.GetDomain(),
		WindowsUser:           d.windowsUser,
		DesktopLabels:         d.desktop.GetAllLabels(),
		StartTime:             d.startTime,
		EndTime:               d.clock.Now().UTC(),
		DesktopName:           d.desktop.GetName(),
		Recorded:              recorded,

		// There can only be 1 participant, desktop sessions are not join-able.
		Participants: []string{userMetadata.User},
	}
}

func (d *desktopSessionAuditor) makeClipboardSend(length int32) *events.DesktopClipboardSend {
	return &events.DesktopClipboardSend{
		Metadata: events.Metadata{
			Type:        libevents.DesktopClipboardSendEvent,
			Code:        libevents.DesktopClipboardSendCode,
			ClusterName: d.clusterName,
			Time:        d.clock.Now().UTC(),
		},
		UserMetadata:       d.identity.GetUserMetadata(),
		SessionMetadata:    d.getSessionMetadata(),
		ConnectionMetadata: d.getConnectionMetadata(),
		DesktopAddr:        d.desktop.GetAddr(),
		Length:             length,
	}
}

func (d *desktopSessionAuditor) makeClipboardReceive(length int32) *events.DesktopClipboardReceive {
	return &events.DesktopClipboardReceive{
		Metadata: events.Metadata{
			Type:        libevents.DesktopClipboardReceiveEvent,
			Code:        libevents.DesktopClipboardReceiveCode,
			ClusterName: d.clusterName,
			Time:        d.clock.Now().UTC(),
		},
		UserMetadata:       d.identity.GetUserMetadata(),
		SessionMetadata:    d.getSessionMetadata(),
		ConnectionMetadata: d.getConnectionMetadata(),
		DesktopAddr:        d.desktop.GetAddr(),
		Length:             length,
	}
}

// onSharedDirectoryAnnounce handles a shared directory announcement.
// In the happy path, no event is emitted here, but details from the announcement
// are cached for future audit events. An event is returned only if there was
// an error.
func (d *desktopSessionAuditor) onSharedDirectoryAnnounce(m tdp.SharedDirectoryAnnounce) *events.DesktopSharedDirectoryStart {
	err := d.auditCache.SetName(directoryID(m.DirectoryID), directoryName(m.Name))
	if err == nil {
		// no work to do yet, but data is cached for future events
		return nil
	}

	// An error means the audit cache exceeded its maximum allowable size.
	errMsg := err.Error()

	return &events.DesktopSharedDirectoryStart{
		Metadata: events.Metadata{
			Type:        libevents.DesktopSharedDirectoryStartEvent,
			Code:        libevents.DesktopSharedDirectoryStartFailureCode,
			ClusterName: d.clusterName,
			Time:        d.clock.Now().UTC(),
		},
		UserMetadata:       d.identity.GetUserMetadata(),
		SessionMetadata:    d.getSessionMetadata(),
		ConnectionMetadata: d.getConnectionMetadata(),
		Status: events.Status{
			Success:     false,
			Error:       errMsg,
			UserMessage: "Teleport failed the request and terminated the session as a security precaution",
		},
		DesktopAddr:   d.desktop.GetAddr(),
		DirectoryName: m.Name,
		DirectoryID:   m.DirectoryID,
	}
}

// makeSharedDirectoryStart creates a DesktopSharedDirectoryStart event.
func (d *desktopSessionAuditor) makeSharedDirectoryStart(m tdp.SharedDirectoryAcknowledge) *events.DesktopSharedDirectoryStart {
	code := libevents.DesktopSharedDirectoryStartCode
	name, ok := d.auditCache.GetName(directoryID(m.DirectoryID))
	if !ok {
		code = libevents.DesktopSharedDirectoryStartFailureCode
		name = "unknown"
	}

	if m.ErrCode != tdp.ErrCodeNil {
		code = libevents.DesktopSharedDirectoryStartFailureCode
	}

	return &events.DesktopSharedDirectoryStart{
		Metadata: events.Metadata{
			Type:        libevents.DesktopSharedDirectoryStartEvent,
			Code:        code,
			ClusterName: d.clusterName,
			Time:        d.clock.Now().UTC(),
		},
		UserMetadata:       d.identity.GetUserMetadata(),
		SessionMetadata:    d.getSessionMetadata(),
		ConnectionMetadata: d.getConnectionMetadata(),
		Status:             statusFromErrCode(m.ErrCode),
		DesktopAddr:        d.desktop.GetAddr(),
		DirectoryName:      string(name),
		DirectoryID:        m.DirectoryID,
	}
}

// onSharedDirectoryReadRequest handles shared directory reads.
// In the happy path, no event is emitted here, but details from the operation
// are cached for future audit events. An event is returned only if there was
// an error.
func (d *desktopSessionAuditor) onSharedDirectoryReadRequest(m tdp.SharedDirectoryReadRequest) *events.DesktopSharedDirectoryRead {
	did := directoryID(m.DirectoryID)
	path := m.Path
	offset := m.Offset

	err := d.auditCache.SetReadRequestInfo(completionID(m.CompletionID), readRequestInfo{
		directoryID: did,
		path:        path,
		offset:      offset,
	})
	if err == nil {
		// no work to do yet, but data is cached for future events
		return nil
	}

	name, ok := d.auditCache.GetName(did)
	if !ok {
		name = "unknown"
	}

	return &events.DesktopSharedDirectoryRead{
		Metadata: events.Metadata{
			Type:        libevents.DesktopSharedDirectoryReadEvent,
			Code:        libevents.DesktopSharedDirectoryReadFailureCode,
			ClusterName: d.clusterName,
			Time:        d.clock.Now().UTC(),
		},
		UserMetadata:       d.identity.GetUserMetadata(),
		SessionMetadata:    d.getSessionMetadata(),
		ConnectionMetadata: d.getConnectionMetadata(),
		Status: events.Status{
			Success:     false,
			Error:       err.Error(),
			UserMessage: "Teleport failed the request and terminated the session as a security precaution",
		},
		DesktopAddr:   d.desktop.GetAddr(),
		DirectoryName: string(name),
		DirectoryID:   uint32(did),
		Path:          path,
		Length:        m.Length,
		Offset:        offset,
	}
}

// makeSharedDirectoryReadResponse creates a DesktopSharedDirectoryRead audit event.
func (d *desktopSessionAuditor) makeSharedDirectoryReadResponse(m tdp.SharedDirectoryReadResponse) *events.DesktopSharedDirectoryRead {
	var did directoryID
	var name directoryName

	var path string
	var offset uint64

	code := libevents.DesktopSharedDirectoryReadCode

	// Gather info from the audit cache
	info, ok := d.auditCache.TakeReadRequestInfo(completionID(m.CompletionID))
	if ok {
		did = info.directoryID
		// Only search for the directory name if we retrieved the directory ID from the audit cache.
		name, ok = d.auditCache.GetName(did)
		if !ok {
			code = libevents.DesktopSharedDirectoryReadFailureCode
			name = "unknown"
		}
		path = info.path
		offset = info.offset
	} else {
		code = libevents.DesktopSharedDirectoryReadFailureCode
		path = "unknown"
		name = "unknown"
	}

	if m.ErrCode != tdp.ErrCodeNil {
		code = libevents.DesktopSharedDirectoryWriteFailureCode
	}

	return &events.DesktopSharedDirectoryRead{
		Metadata: events.Metadata{
			Type:        libevents.DesktopSharedDirectoryReadEvent,
			Code:        code,
			ClusterName: d.clusterName,
			Time:        d.clock.Now().UTC(),
		},
		UserMetadata:       d.identity.GetUserMetadata(),
		SessionMetadata:    d.getSessionMetadata(),
		ConnectionMetadata: d.getConnectionMetadata(),
		Status:             statusFromErrCode(m.ErrCode),
		DesktopAddr:        d.desktop.GetAddr(),
		DirectoryName:      string(name),
		DirectoryID:        uint32(did),
		Path:               path,
		Length:             m.ReadDataLength,
		Offset:             offset,
	}
}

// onSharedDirectoryWriteRequest handles shared directory writes.
// In the happy path, no event is emitted here, but details from the operation
// are cached for future audit events. An event is returned only if there was
// an error.
func (d *desktopSessionAuditor) onSharedDirectoryWriteRequest(m tdp.SharedDirectoryWriteRequest) *events.DesktopSharedDirectoryWrite {
	did := directoryID(m.DirectoryID)
	path := m.Path
	offset := m.Offset

	err := d.auditCache.SetWriteRequestInfo(
		completionID(m.CompletionID),
		writeRequestInfo{
			directoryID: did,
			path:        path,
			offset:      offset,
		})
	if err == nil {
		// no work to do yet, but data is cached for future events
		return nil
	}

	name, ok := d.auditCache.GetName(did)
	if !ok {
		name = "unknown"
	}

	return &events.DesktopSharedDirectoryWrite{
		Metadata: events.Metadata{
			Type:        libevents.DesktopSharedDirectoryWriteEvent,
			Code:        libevents.DesktopSharedDirectoryWriteFailureCode,
			ClusterName: d.clusterName,
			Time:        d.clock.Now().UTC(),
		},
		UserMetadata:       d.identity.GetUserMetadata(),
		SessionMetadata:    d.getSessionMetadata(),
		ConnectionMetadata: d.getConnectionMetadata(),
		Status: events.Status{
			Success:     false,
			Error:       err.Error(),
			UserMessage: "Teleport failed the request and terminated the session as a security precaution",
		},
		DesktopAddr:   d.desktop.GetAddr(),
		DirectoryName: string(name),
		DirectoryID:   uint32(did),
		Path:          path,
		Length:        m.WriteDataLength,
		Offset:        offset,
	}
}

// makeSharedDirectoryWriteResponse creates a DesktopSharedDirectoryWrite audit event.
func (d *desktopSessionAuditor) makeSharedDirectoryWriteResponse(m tdp.SharedDirectoryWriteResponse) *events.DesktopSharedDirectoryWrite {
	var did directoryID
	var name directoryName

	var path string
	var offset uint64

	code := libevents.DesktopSharedDirectoryWriteCode
	// Gather info from the audit cache
	info, ok := d.auditCache.TakeWriteRequestInfo(completionID(m.CompletionID))
	if ok {
		did = info.directoryID
		// Only search for the directory name if we retrieved the directoryID from the audit cache.
		name, ok = d.auditCache.GetName(did)
		if !ok {
			code = libevents.DesktopSharedDirectoryWriteFailureCode
			name = "unknown"
		}
		path = info.path
		offset = info.offset
	} else {
		code = libevents.DesktopSharedDirectoryWriteFailureCode
		path = "unknown"
		name = "unknown"
	}

	if m.ErrCode != tdp.ErrCodeNil {
		code = libevents.DesktopSharedDirectoryWriteFailureCode
	}

	return &events.DesktopSharedDirectoryWrite{
		Metadata: events.Metadata{
			Type:        libevents.DesktopSharedDirectoryWriteEvent,
			Code:        code,
			ClusterName: d.clusterName,
			Time:        d.clock.Now().UTC(),
		},
		UserMetadata:       d.identity.GetUserMetadata(),
		SessionMetadata:    d.getSessionMetadata(),
		ConnectionMetadata: d.getConnectionMetadata(),
		Status:             statusFromErrCode(m.ErrCode),
		DesktopAddr:        d.desktop.GetAddr(),
		DirectoryName:      string(name),
		DirectoryID:        uint32(did),
		Path:               path,
		Length:             m.BytesWritten,
		Offset:             offset,
	}
}

func (s *WindowsService) emit(ctx context.Context, event events.AuditEvent) {
	if err := s.cfg.Emitter.EmitAuditEvent(ctx, event); err != nil {
		s.cfg.Log.WithError(err).Errorf("Failed to emit audit event %v", event)
	}
}

func (s *WindowsService) record(ctx context.Context, recorder libevents.SessionPreparerRecorder, event events.AuditEvent) {
	if err := libevents.SetupAndRecordEvent(ctx, recorder, event); err != nil {
		s.cfg.Log.WithError(err).Errorf("Failed to record session event %v", event)
	}
}

func statusFromErrCode(errCode uint32) events.Status {
	success := errCode == tdp.ErrCodeNil

	// early return for most common case
	if success {
		return events.Status{
			Success: success,
		}
	}

	msg := unknownErrStatusMsg
	switch errCode {
	case tdp.ErrCodeFailed:
		msg = failedStatusMessage
	case tdp.ErrCodeDoesNotExist:
		msg = doesNotExistStatusMessage
	case tdp.ErrCodeAlreadyExists:
		msg = alreadyExistsStatusMessage
	}

	return events.Status{
		Success:     success,
		Error:       msg,
		UserMessage: msg,
	}
}

const (
	failedStatusMessage        = "operation failed"
	doesNotExistStatusMessage  = "item does not exist"
	alreadyExistsStatusMessage = "item already exists"
	unknownErrStatusMsg        = "unknown error"
)
