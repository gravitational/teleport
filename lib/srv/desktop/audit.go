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
	"log/slog"
	"time"

	"github.com/gravitational/teleport/api/constants"
	linuxdesktopv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/linuxdesktop/v1"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/legacy"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/tdpb"
	"github.com/gravitational/teleport/lib/tlsca"
)

// desktopSessionAuditor is used to build session-related events
// which are emitted to Teleport's audit log
type desktopSessionAuditor struct {
	clock clockwork.Clock

	sessionID      string
	identity       *tlsca.Identity
	targetUser     string
	windowsDesktop types.WindowsDesktop
	linuxDesktop   *linuxdesktopv1.LinuxDesktop
	enableNLA      bool

	startTime          time.Time
	clusterName        string
	desktopServiceUUID string

	compactor  auditCompactor
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
		RemoteAddr: d.getName(),
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

		sessionID:          sessionID,
		identity:           identity,
		targetUser:         windowsUser,
		windowsDesktop:     desktop,
		enableNLA:          s.enableNLA,
		startTime:          s.cfg.Clock.Now().UTC().Round(time.Millisecond),
		clusterName:        s.clusterName,
		desktopServiceUUID: s.cfg.Heartbeat.HostUUID,
		compactor:          newAuditCompactor(3*time.Second, 10*time.Second, s.emit),
		auditCache:         newSharedDirectoryAuditCache(),
	}
}

func (s *LinuxService) newSessionAuditor(
	sessionID string,
	identity *tlsca.Identity,
	linuxUser string,
	desktop *linuxdesktopv1.LinuxDesktop,
) *desktopSessionAuditor {
	return &desktopSessionAuditor{
		clock: s.cfg.Clock,

		sessionID:          sessionID,
		identity:           identity,
		targetUser:         linuxUser,
		linuxDesktop:       desktop,
		startTime:          s.cfg.Clock.Now().UTC().Round(time.Millisecond),
		clusterName:        s.clusterName,
		desktopServiceUUID: s.cfg.Heartbeat.HostUUID,
		compactor:          newAuditCompactor(3*time.Second, 10*time.Second, s.emit),
		auditCache:         newSharedDirectoryAuditCache(),
	}
}

func (d *desktopSessionAuditor) teardown(ctx context.Context) {
	d.compactor.flush(ctx)
}

func (d *desktopSessionAuditor) getAddr() string {
	if d.windowsDesktop != nil {
		return d.getName()
	} else if d.linuxDesktop != nil {
		return d.linuxDesktop.GetSpec().GetAddr()
	}
	return ""
}

func (d *desktopSessionAuditor) getName() string {
	if d.windowsDesktop != nil {
		return d.getName()
	} else if d.linuxDesktop != nil {
		return d.linuxDesktop.GetMetadata().GetName()
	}
	return ""
}

func (d *desktopSessionAuditor) makeWindowsSessionStart(err error) *events.WindowsDesktopSessionStart {
	userMetadata := d.identity.GetUserMetadata()
	userMetadata.Login = d.targetUser

	event := &events.WindowsDesktopSessionStart{
		Metadata: events.Metadata{
			Type:        libevents.WindowsDesktopSessionStartEvent,
			Code:        libevents.DesktopSessionStartCode,
			ClusterName: d.clusterName,
			Time:        d.startTime,
		},
		CertMetadata:          new(events.WindowsCertificateMetadata),
		UserMetadata:          userMetadata,
		SessionMetadata:       d.getSessionMetadata(),
		ConnectionMetadata:    d.getConnectionMetadata(),
		Status:                events.Status{Success: err == nil},
		WindowsDesktopService: d.desktopServiceUUID,
		DesktopName:           d.getName(),
		DesktopAddr:           d.getName(),
		Domain:                d.windowsDesktop.GetDomain(),
		WindowsUser:           d.targetUser,
		DesktopLabels:         d.windowsDesktop.GetAllLabels(),
		NLA:                   d.enableNLA && !d.windowsDesktop.NonAD(),
	}

	if err != nil {
		event.Code = libevents.DesktopSessionStartFailureCode
		event.Error = trace.Unwrap(err).Error()
		event.UserMessage = err.Error()
	}

	return event
}

func (d *desktopSessionAuditor) makeWindowsSessionEnd(recorded bool) *events.WindowsDesktopSessionEnd {
	userMetadata := d.identity.GetUserMetadata()
	userMetadata.Login = d.targetUser

	return &events.WindowsDesktopSessionEnd{
		Metadata: events.Metadata{
			Type:        libevents.WindowsDesktopSessionEndEvent,
			Code:        libevents.DesktopSessionEndCode,
			ClusterName: d.clusterName,
		},
		UserMetadata:          userMetadata,
		SessionMetadata:       d.getSessionMetadata(),
		ConnectionMetadata:    d.getConnectionMetadata(),
		WindowsDesktopService: d.desktopServiceUUID,
		DesktopAddr:           d.getName(),
		Domain:                d.windowsDesktop.GetDomain(),
		WindowsUser:           d.targetUser,
		DesktopLabels:         d.windowsDesktop.GetAllLabels(),
		StartTime:             d.startTime,
		EndTime:               d.clock.Now().UTC(),
		DesktopName:           d.getName(),
		Recorded:              recorded,

		// There can only be 1 participant, desktop sessions are not join-able.
		Participants: []string{
			services.UsernameForCluster(
				services.UsernameForClusterConfig{
					User:              d.identity.Username,
					OriginClusterName: d.identity.OriginClusterName,
					LocalClusterName:  d.clusterName,
				},
			)},
	}
}

func (d *desktopSessionAuditor) makeLinuxSessionStart(err error) *events.LinuxDesktopSessionStart {
	userMetadata := d.identity.GetUserMetadata()
	userMetadata.Login = d.targetUser

	event := &events.LinuxDesktopSessionStart{
		Metadata: events.Metadata{
			Type:        libevents.LinuxDesktopSessionStartEvent,
			Code:        libevents.LinuxDesktopSessionStartCode,
			ClusterName: d.clusterName,
			Time:        d.startTime,
		},
		UserMetadata:       userMetadata,
		SessionMetadata:    d.getSessionMetadata(),
		ConnectionMetadata: d.getConnectionMetadata(),
		Status:             events.Status{Success: err == nil},
		DesktopName:        d.getName(),
		DesktopAddr:        d.getName(),
		LinuxUser:          d.targetUser,
		DesktopLabels:      d.linuxDesktop.Metadata.Labels,
	}

	if err != nil {
		event.Code = libevents.LinuxDesktopSessionStartFailureCode
		event.Error = trace.Unwrap(err).Error()
		event.UserMessage = err.Error()
	}

	return event
}

func (d *desktopSessionAuditor) makeLinuxSessionEnd(recorded bool) *events.LinuxDesktopSessionEnd {
	userMetadata := d.identity.GetUserMetadata()
	userMetadata.Login = d.targetUser

	return &events.LinuxDesktopSessionEnd{
		Metadata: events.Metadata{
			Type:        libevents.LinuxDesktopSessionEndEvent,
			Code:        libevents.LinuxDesktopSessionEndCode,
			ClusterName: d.clusterName,
		},
		UserMetadata:       userMetadata,
		SessionMetadata:    d.getSessionMetadata(),
		ConnectionMetadata: d.getConnectionMetadata(),
		DesktopAddr:        d.getName(),
		LinuxUser:          d.targetUser,
		DesktopLabels:      d.linuxDesktop.Metadata.Labels,
		StartTime:          d.startTime,
		EndTime:            d.clock.Now().UTC(),
		DesktopName:        d.getName(),
		Recorded:           recorded,

		// There can only be 1 participant, desktop sessions are not join-able.
		Participants: []string{
			services.UsernameForCluster(
				services.UsernameForClusterConfig{
					User:              d.identity.Username,
					OriginClusterName: d.identity.OriginClusterName,
					LocalClusterName:  d.clusterName,
				},
			)},
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
		DesktopAddr:        d.getName(),
		Length:             length,
		DesktopName:        d.getName(),
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
		DesktopAddr:        d.getName(),
		Length:             length,
		DesktopName:        d.getName(),
	}
}

// onSharedDirectoryAnnounce handles a shared directory announcement.
// In the happy path, no event is emitted here, but details from the announcement
// are cached for future audit events. An event is returned only if there was
// an error.
func (d *desktopSessionAuditor) onSharedDirectoryAnnounce(m *tdpb.SharedDirectoryAnnounce) *events.DesktopSharedDirectoryStart {
	err := d.auditCache.SetName(directoryID(m.DirectoryId), directoryName(m.Name))
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
		DesktopAddr:   d.getName(),
		DirectoryName: m.Name,
		DirectoryID:   m.DirectoryId,
		DesktopName:   d.getName(),
	}
}

// makeSharedDirectoryStart creates a DesktopSharedDirectoryStart event.
func (d *desktopSessionAuditor) makeSharedDirectoryStart(m *tdpb.SharedDirectoryAcknowledge) *events.DesktopSharedDirectoryStart {
	code := libevents.DesktopSharedDirectoryStartCode
	name, ok := d.auditCache.GetName(directoryID(m.DirectoryId))
	if !ok {
		code = libevents.DesktopSharedDirectoryStartFailureCode
		name = "unknown"
	}

	if m.ErrorCode != legacy.ErrCodeNil {
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
		Status:             statusFromErrCode(m.ErrorCode),
		DesktopAddr:        d.getName(),
		DirectoryName:      string(name),
		DirectoryID:        m.DirectoryId,
		DesktopName:        d.getName(),
	}
}

// onSharedDirectoryReadRequest handles shared directory reads.
// In the happy path, no event is emitted here, but details from the operation
// are cached for future audit events. An event is returned only if there was
// an error.
func (d *desktopSessionAuditor) onSharedDirectoryReadRequest(completion completionID, directory directoryID, m *tdpbv1.SharedDirectoryRequest_Read) *events.DesktopSharedDirectoryRead {
	did := directory
	path := m.Path
	offset := m.Offset

	err := d.auditCache.SetReadRequestInfo(completion, readRequestInfo{
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
		DesktopAddr:   d.getName(),
		DirectoryName: string(name),
		DirectoryID:   uint32(did),
		Path:          path,
		Length:        m.Length,
		Offset:        offset,
		DesktopName:   d.getName(),
	}
}

// makeSharedDirectoryReadResponse creates a DesktopSharedDirectoryRead audit event.
func (d *desktopSessionAuditor) makeSharedDirectoryReadResponse(completion completionID, errorCode uint32, m *tdpbv1.SharedDirectoryResponse_Read) *events.DesktopSharedDirectoryRead {
	var did directoryID
	var name directoryName

	var path string
	var offset uint64

	code := libevents.DesktopSharedDirectoryReadCode

	// Gather info from the audit cache
	info, ok := d.auditCache.TakeReadRequestInfo(completion)
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

	if errorCode != legacy.ErrCodeNil {
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
		Status:             statusFromErrCode(errorCode),
		DesktopAddr:        d.getName(),
		DirectoryName:      string(name),
		DirectoryID:        uint32(did),
		Path:               path,
		Length:             uint32(len(m.Data)),
		Offset:             offset,
		DesktopName:        d.getName(),
	}
}

// onSharedDirectoryWriteRequest handles shared directory writes.
// In the happy path, no event is emitted here, but details from the operation
// are cached for future audit events. An event is returned only if there was
// an error.
func (d *desktopSessionAuditor) onSharedDirectoryWriteRequest(completion completionID, directory directoryID, m *tdpbv1.SharedDirectoryRequest_Write) *events.DesktopSharedDirectoryWrite {
	did := directory
	path := m.Path
	offset := m.Offset

	err := d.auditCache.SetWriteRequestInfo(
		completion,
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
		DesktopName:   d.getName(),
		DesktopAddr:   d.getName(),
		DirectoryName: string(name),
		DirectoryID:   uint32(did),
		Path:          path,
		Length:        uint32(len(m.Data)),
		Offset:        offset,
	}
}

// makeSharedDirectoryWriteResponse creates a DesktopSharedDirectoryWrite audit event.
func (d *desktopSessionAuditor) makeSharedDirectoryWriteResponse(completion completionID, errorCode uint32, m *tdpbv1.SharedDirectoryResponse_Write) *events.DesktopSharedDirectoryWrite {
	var did directoryID
	var name directoryName

	var path string
	var offset uint64

	code := libevents.DesktopSharedDirectoryWriteCode
	// Gather info from the audit cache
	info, ok := d.auditCache.TakeWriteRequestInfo(completion)
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

	if errorCode != legacy.ErrCodeNil {
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
		Status:             statusFromErrCode(errorCode),
		DesktopAddr:        d.getName(),
		DirectoryName:      string(name),
		DirectoryID:        uint32(did),
		Path:               path,
		Length:             m.BytesWritten,
		Offset:             offset,
		DesktopName:        d.getName(),
	}
}

func emit(ctx context.Context, emitter events.Emitter, logger *slog.Logger, event events.AuditEvent) {
	if err := emitter.EmitAuditEvent(ctx, event); err != nil {
		logger.ErrorContext(ctx, "Failed to emit audit event", "kind", event.GetType())
	}
}

func (s *WindowsService) emit(ctx context.Context, event events.AuditEvent) {
	emit(ctx, s.cfg.Emitter, s.cfg.Logger, event)
}

func (s *LinuxService) emit(ctx context.Context, event events.AuditEvent) {
	emit(ctx, s.cfg.Emitter, s.cfg.Logger, event)
}

func record(ctx context.Context, logger *slog.Logger, recorder libevents.SessionPreparerRecorder, event events.AuditEvent) {
	if err := libevents.SetupAndRecordEvent(ctx, recorder, event); err != nil {
		logger.ErrorContext(ctx, "Failed to record session event", "kind", event.GetType())
	}
}

func (s *WindowsService) record(ctx context.Context, recorder libevents.SessionPreparerRecorder, event events.AuditEvent) {
	record(ctx, s.cfg.Logger, recorder, event)
}

func (s *LinuxService) record(ctx context.Context, recorder libevents.SessionPreparerRecorder, event events.AuditEvent) {
	record(ctx, s.cfg.Logger, recorder, event)
}

func statusFromErrCode(errCode uint32) events.Status {
	success := errCode == legacy.ErrCodeNil

	// early return for most common case
	if success {
		return events.Status{
			Success: success,
		}
	}

	msg := unknownErrStatusMsg
	switch errCode {
	case legacy.ErrCodeFailed:
		msg = failedStatusMessage
	case legacy.ErrCodeDoesNotExist:
		msg = doesNotExistStatusMessage
	case legacy.ErrCodeAlreadyExists:
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

type emitter interface {
	emit(ctx context.Context, event events.AuditEvent)
}

func recordEvent(ctx context.Context, clock clockwork.Clock, logger *slog.Logger, delay int64, m tdp.Message, data []byte, recorder libevents.SessionPreparerRecorder) {
	e := &events.DesktopRecording{
		Metadata: events.Metadata{
			Type: libevents.DesktopRecordingEvent,
			Time: clock.Now().Round(time.Millisecond),
		},
		TDPBMessage:       data,
		DelayMilliseconds: delay,
	}

	if len(data) > constants.MaxProtoMessageSizeBytes {
		// Technically a PNG frame is unbounded and could be too big for a single protobuf.
		// In practice though, Windows limits RDP bitmaps to 64x64 pixels, and we compress
		// the PNGs before they get here, so most PNG frames are under 500 bytes. The largest
		// ones are around 2000 bytes. Anything approaching the limit of a single protobuf
		// is likely some sort of DoS attempt and not legitimate RDP traffic, so we don't log it.
		logger.WarnContext(ctx, "refusing to record message", "len", len(data), "type", logutils.TypeAttr(m))
	} else {
		if err := libevents.SetupAndRecordEvent(ctx, recorder, e); err != nil {
			logger.WarnContext(ctx, "could not record desktop recording event", "error", err)
		}
	}
}

func makeTDPSendHandler(
	ctx context.Context,
	s emitter,
	clock clockwork.Clock,
	logger *slog.Logger,
	recorder libevents.SessionPreparerRecorder,
	delay func() int64,
	tdpConn *tdp.Conn,
	audit *desktopSessionAuditor,
) func(m tdp.Message, b []byte) {
	return func(msg tdp.Message, data []byte) {
		switch m := msg.(type) {
		case *tdpb.ServerHello, *tdpb.FastPathPDU, *tdpb.PNGFrame, *tdpb.Alert:
			recordEvent(ctx, clock, logger, delay(), m, data, recorder)
		case *tdpb.ClipboardData:
			// the TDP send handler emits a clipboard receive event, because we
			// received clipboard data from the remote desktop and are sending
			// it on the TDP connection
			rxEvent := audit.makeClipboardReceive(int32(len(m.Data)))
			s.emit(ctx, rxEvent)
		case *tdpb.SharedDirectoryAcknowledge:
			s.emit(ctx, audit.makeSharedDirectoryStart(m))
		case *tdpb.SharedDirectoryRequest:
			switch req := m.Operation.(type) {
			case *tdpbv1.SharedDirectoryRequest_Write_:
				errorEvent := audit.onSharedDirectoryWriteRequest(completionID(m.CompletionId), directoryID(m.DirectoryId), req.Write)
				if errorEvent != nil {
					// if we can't audit due to a full cache, abort the connection
					// as a security measure
					if err := tdpConn.Close(); err != nil {
						logger.ErrorContext(ctx, "error when terminating session for audit cache maximum size violation", "session_id", audit.sessionID)
					}
					s.emit(ctx, errorEvent)
				}
			case *tdpbv1.SharedDirectoryRequest_Read_:
				errorEvent := audit.onSharedDirectoryReadRequest(completionID(m.CompletionId), directoryID(m.DirectoryId), req.Read)
				if errorEvent != nil {
					// if we can't audit due to a full cache, abort the connection
					// as a security measure
					if err := tdpConn.Close(); err != nil {
						logger.ErrorContext(ctx, "error when terminating session for audit cache maximum size violation", "session_id", audit.sessionID)
					}
					s.emit(ctx, errorEvent)
				}
			}
		}
	}
}

func makeTDPReceiveHandler(
	ctx context.Context,
	s emitter,
	clock clockwork.Clock,
	logger *slog.Logger,
	recorder libevents.SessionPreparerRecorder,
	delay func() int64,
	tdpConn *tdp.Conn,
	audit *desktopSessionAuditor,
) func(m tdp.Message) {
	return func(m tdp.Message) {
		switch msg := m.(type) {
		case *tdpb.ClientScreenSpec, *tdpb.MouseButton, *tdpb.MouseMove:
			b, err := m.Encode()
			if err != nil {
				logger.WarnContext(ctx, "could not emit desktop recording event", "error", err)
			}

			recordEvent(ctx, clock, logger, delay(), m, b, recorder)
		case *tdpb.ClipboardData:
			// the TDP receive handler emits a clipboard send event, because we
			// received clipboard data from the user (over TDP) and are sending
			// it to the remote desktop
			sendEvent := audit.makeClipboardSend(int32(len(msg.Data)))
			s.emit(ctx, sendEvent)
		case *tdpb.SharedDirectoryAnnounce:
			errorEvent := audit.onSharedDirectoryAnnounce(m.(*tdpb.SharedDirectoryAnnounce))
			if errorEvent != nil {
				// if we can't audit due to a full cache, abort the connection
				// as a security measure
				if err := tdpConn.Close(); err != nil {
					logger.ErrorContext(ctx, "error when terminating session for audit cache maximum size violation",
						"session_id", audit.sessionID, "error", err)
				}
				s.emit(ctx, errorEvent)
			}
		case *tdpb.SharedDirectoryResponse:
			// shared directory audit events can be noisy, so we use a compactor
			// to retain and delay them in an attempt to coalesce contiguous events
			switch op := msg.Operation.(type) {
			case *tdpbv1.SharedDirectoryResponse_Read_:
				audit.compactor.handleRead(ctx, audit.makeSharedDirectoryReadResponse(completionID(msg.CompletionId), msg.ErrorCode, op.Read))
			case *tdpbv1.SharedDirectoryResponse_Write_:
				audit.compactor.handleWrite(ctx, audit.makeSharedDirectoryWriteResponse(completionID(msg.CompletionId), msg.ErrorCode, op.Write))
			}
		}
	}
}
