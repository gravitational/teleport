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

package events

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	apievents "github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
)

// FindSessionEndEvent streams session events to find the session end event for the given session ID.
// It returns:
// - SessionEnd
// - DatabaseSessionEnd
// - WindowsDesktopSessionEnd
// - AppSessionEnd
// - MCPSessionEnd,
// or nil if none is found.
// TODO(tigrato): Revisit this approach for large sessions, as it's highly inefficient.
// Instead, consider downloading the last few parts of the recording to find the session end event
// instead of streaming all events.
func FindSessionEndEvent(ctx context.Context, streamer SessionStreamer, sessionID session.ID) (apievents.AuditEvent, error) {
	switch {
	case streamer == nil:
		return nil, trace.BadParameter("session streamer is required")
	case sessionID == "":
		return nil, trace.BadParameter("session ID is required")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	eventsCh, errCh := streamer.StreamSessionEvents(ctx, sessionID, 0)
	for {
		select {
		case event, ok := <-eventsCh:
			if !ok {
				return nil, trace.NotFound("session end event not found")
			}
			switch e := event.(type) {
			case *apievents.WindowsDesktopSessionEnd:
				return e, nil
			case *apievents.SessionEnd:
				return e, nil
			case *apievents.DatabaseSessionEnd:
				return e, nil
			case *apievents.AppSessionEnd:
				return e, nil
			case *apievents.MCPSessionEnd:
				return e, nil
			}
		case err := <-errCh:
			return nil, trace.Wrap(err)
		case <-ctx.Done():
			return nil, trace.Wrap(ctx.Err())
		}
	}
}

// FindOrRecoverSessionEndConfig holds the configuration for FindOrRecoverSessionEnd.
type FindOrRecoverSessionEndConfig struct {
	// ClusterName is the name of the cluster where the session took place.
	ClusterName string
	// Streamer is used to stream session events.
	Streamer SessionStreamer
	// SessionID is the ID of the session to recover.
	SessionID session.ID
	// AuditLog is the emitter used to emit the recovered session end event.
	AuditLog apievents.Emitter
	// Log is the logger.
	Log *slog.Logger
	// Clock is used to timestamp the recovered event.
	Clock clockwork.Clock
}

// FindOrRecoverSessionEnd streams the events for the given session and returns
// the session end event.
//
// If a session end event already exists in the stream, it is returned directly.
// Otherwise, the function reconstructs (recovers) a session end event from the
// session start event and any additional events found in the stream, emits it to
// the audit log, and returns it.
//
// Supported session types are:
//   - SSH / Kubernetes (session.start -> session.end)
//   - Windows Desktop (windows.desktop.session.start -> windows.desktop.session.end)
//   - Database (db.session.start -> db.session.end)
//   - Application (app.session.start -> app.session.end)
//   - MCP (mcp.session.start -> mcp.session.end)
//
// An error is returned if no events are found for the session, if the session
// start event cannot be identified, or if emitting the recovered event fails.
func FindOrRecoverSessionEnd(ctx context.Context, cfg FindOrRecoverSessionEndConfig) (apievents.AuditEvent, error) {
	if err := validateFindOrRecoverSessionEndConfig(cfg); err != nil {
		return nil, trace.Wrap(err)
	}
	// at this point, we don't know which session type we're dealing with, but as
	// soon as we see the session start we'll be able to start filling in the details
	var sshSessionEnd apievents.SessionEnd
	var desktopSessionEnd apievents.WindowsDesktopSessionEnd
	var dbSessionEnd apievents.DatabaseSessionEnd
	var appSessionEnd apievents.AppSessionEnd
	var mcpSessionEnd apievents.MCPSessionEnd

	// We use the streaming events API to search through the session events, because it works
	// for all session types
	var lastEvent apievents.AuditEvent
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	evts, errors := cfg.Streamer.StreamSessionEvents(ctx, cfg.SessionID, 0)

loop:
	for {
		select {
		case evt, more := <-evts:
			if !more {
				break loop
			}

			lastEvent = evt

			switch e := evt.(type) {
			// Return if session end event already exists
			case *apievents.SessionEnd, *apievents.WindowsDesktopSessionEnd,
				*apievents.DatabaseSessionEnd, *apievents.AppSessionEnd, *apievents.MCPSessionEnd:
				return e, nil

			case *apievents.WindowsDesktopSessionStart:
				desktopSessionEnd.Type = WindowsDesktopSessionEndEvent
				desktopSessionEnd.Code = DesktopSessionEndCode
				desktopSessionEnd.ClusterName = e.ClusterName
				desktopSessionEnd.StartTime = e.Time
				desktopSessionEnd.Participants = append(desktopSessionEnd.Participants, transformedUsername(e.UserMetadata, cfg.ClusterName))
				desktopSessionEnd.Recorded = true
				desktopSessionEnd.UserMetadata = e.UserMetadata
				desktopSessionEnd.SessionMetadata = e.SessionMetadata
				desktopSessionEnd.WindowsDesktopService = e.WindowsDesktopService
				desktopSessionEnd.Domain = e.Domain
				desktopSessionEnd.DesktopAddr = e.DesktopAddr
				desktopSessionEnd.DesktopLabels = e.DesktopLabels
				desktopSessionEnd.DesktopName = fmt.Sprintf("%v (recovered)", e.DesktopName)

			case *apievents.SessionStart:
				sshSessionEnd.Type = SessionEndEvent
				sshSessionEnd.Code = SessionEndCode
				sshSessionEnd.ClusterName = e.ClusterName
				sshSessionEnd.StartTime = e.Time
				sshSessionEnd.UserMetadata = e.UserMetadata
				sshSessionEnd.SessionMetadata = e.SessionMetadata
				sshSessionEnd.ServerMetadata = e.ServerMetadata
				sshSessionEnd.ConnectionMetadata = e.ConnectionMetadata
				sshSessionEnd.KubernetesClusterMetadata = e.KubernetesClusterMetadata
				sshSessionEnd.KubernetesPodMetadata = e.KubernetesPodMetadata
				sshSessionEnd.InitialCommand = e.InitialCommand
				sshSessionEnd.SessionRecording = e.SessionRecording
				sshSessionEnd.Interactive = e.TerminalSize != ""
				sshSessionEnd.Participants = append(sshSessionEnd.Participants, transformedUsername(e.UserMetadata, cfg.ClusterName))

			case *apievents.SessionJoin:
				sshSessionEnd.Participants = append(sshSessionEnd.Participants, transformedUsername(e.UserMetadata, cfg.ClusterName))

			case *apievents.DatabaseSessionStart:
				dbSessionEnd.Type = DatabaseSessionEndEvent
				dbSessionEnd.Code = DatabaseSessionEndCode
				dbSessionEnd.ClusterName = e.ClusterName
				dbSessionEnd.StartTime = e.Time
				dbSessionEnd.UserMetadata = e.UserMetadata
				dbSessionEnd.SessionMetadata = e.SessionMetadata
				dbSessionEnd.DatabaseMetadata = e.DatabaseMetadata
				dbSessionEnd.ConnectionMetadata = e.ConnectionMetadata
				dbSessionEnd.Participants = append(dbSessionEnd.Participants, transformedUsername(e.UserMetadata, cfg.ClusterName))

			case *apievents.AppSessionStart:
				appSessionEnd.Type = AppSessionEndEvent
				appSessionEnd.Code = AppSessionEndCode
				appSessionEnd.ClusterName = e.ClusterName
				appSessionEnd.UserMetadata = e.UserMetadata
				appSessionEnd.SessionMetadata = e.SessionMetadata
				appSessionEnd.ServerMetadata = e.ServerMetadata
				appSessionEnd.ConnectionMetadata = e.ConnectionMetadata
				appSessionEnd.AppMetadata = e.AppMetadata

			case *apievents.MCPSessionStart:
				mcpSessionEnd.Type = MCPSessionEndEvent
				mcpSessionEnd.Code = MCPSessionEndCode
				mcpSessionEnd.ClusterName = e.ClusterName
				mcpSessionEnd.UserMetadata = e.UserMetadata
				mcpSessionEnd.SessionMetadata = e.SessionMetadata
				mcpSessionEnd.ServerMetadata = e.ServerMetadata
				mcpSessionEnd.ConnectionMetadata = e.ConnectionMetadata
				mcpSessionEnd.AppMetadata = e.AppMetadata
			}

		case err := <-errors:
			return nil, trace.Wrap(err)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if lastEvent == nil {
		return nil, trace.Errorf("could not find any events for session %v", cfg.SessionID)
	}

	sshSessionEnd.Participants = apiutils.Deduplicate(sshSessionEnd.Participants)
	sshSessionEnd.EndTime = lastEvent.GetTime()
	desktopSessionEnd.EndTime = lastEvent.GetTime()
	dbSessionEnd.EndTime = lastEvent.GetTime()

	var sessionEndEvent apievents.AuditEvent
	switch {
	case sshSessionEnd.Code != "":
		sessionEndEvent = &sshSessionEnd
	case desktopSessionEnd.Code != "":
		sessionEndEvent = &desktopSessionEnd
	case dbSessionEnd.Code != "":
		sessionEndEvent = &dbSessionEnd
	case appSessionEnd.Code != "":
		sessionEndEvent = &appSessionEnd
	case mcpSessionEnd.Code != "":
		sessionEndEvent = &mcpSessionEnd
	default:
		return nil, trace.BadParameter("invalid session, could not find session start")
	}

	cfg.Log.InfoContext(ctx, "emitting event for completed session",
		"event_type", sessionEndEvent.GetType(),
		"event_code", sessionEndEvent.GetCode(),
		"session_id", cfg.SessionID,
	)

	sessionEndEvent.SetTime(lastEvent.GetTime())

	// Check and set event fields
	if err := checkAndSetEventFields(sessionEndEvent, cfg.Clock, utils.NewRealUID(), sessionEndEvent.GetClusterName()); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := cfg.AuditLog.EmitAuditEvent(ctx, sessionEndEvent); err != nil {
		return nil, trace.Wrap(err)
	}
	return sessionEndEvent, nil
}

func validateFindOrRecoverSessionEndConfig(cfg FindOrRecoverSessionEndConfig) error {
	switch {
	case cfg.ClusterName == "":
		return trace.BadParameter("ClusterName is required")
	case cfg.Streamer == nil:
		return trace.BadParameter("Streamer is required")
	case cfg.SessionID == "":
		return trace.BadParameter("SessionID is required")
	case cfg.AuditLog == nil:
		return trace.BadParameter("AuditLog is required")
	case cfg.Log == nil:
		return trace.BadParameter("Log is required")
	case cfg.Clock == nil:
		return trace.BadParameter("Clock is required")
	}
	return nil
}
