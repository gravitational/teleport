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

package common

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
)

// Audit defines an interface for database access audit events logger.
type Audit interface {
	// OnSessionStart is called on successful/unsuccessful database session start.
	OnSessionStart(ctx context.Context, session *Session, sessionErr error)
	// OnSessionEnd is called when database session terminates.
	OnSessionEnd(ctx context.Context, session *Session)
	// OnQuery is called when a database query or command is executed.
	OnQuery(ctx context.Context, session *Session, query Query)
	// OnResult is called when a database query or command returns.
	OnResult(ctx context.Context, session *Session, result Result)
	// EmitEvent emits the provided audit event to audit log and session recording.
	EmitEvent(ctx context.Context, event events.AuditEvent)
	// RecordEvent emits event to the session recording.
	RecordEvent(ctx context.Context, event events.AuditEvent)
	// OnPermissionsUpdate is called when granular database-level user permissions are updated.
	OnPermissionsUpdate(ctx context.Context, session *Session, entries []events.DatabasePermissionEntry)
	// OnDatabaseUserCreate is called when a database user is provisioned.
	OnDatabaseUserCreate(ctx context.Context, session *Session, err error)
	// OnDatabaseUserDeactivate is called when a database user is disabled or deleted.
	// Shouldn't be called if deactivation failed due to the user being active.
	OnDatabaseUserDeactivate(ctx context.Context, session *Session, delete bool, err error)
}

// Query combines database query parameters.
type Query struct {
	// Query is the SQL query text.
	Query string
	// Parameters contains optional prepared statement parameters.
	Parameters []string
	// Database is optional database name the query is executed in.
	Database string
	// Error contains error, if any, signaling query failure.
	Error error
}

// Result represents a query or command result.
type Result struct {
	// Error is the error message. If error is nil, then the result represents a
	// success.
	Error error
	// AffectedRecords is the number of records affected by the query/command.
	AffectedRecords uint64
	// UserMessage is a user-friendly message for successful or unsuccessful
	// results.
	UserMessage string
}

// AuditConfig is the audit events emitter configuration.
type AuditConfig struct {
	// Emitter is used to emit audit events.
	Emitter events.Emitter
	// Recorder is used to record session events.
	Recorder libevents.SessionPreparerRecorder
	// Database is the database in context.
	Database types.Database
	// Component is the component in use.
	Component string
	// Clock used to control time.
	Clock clockwork.Clock
}

// Check validates the config.
func (c *AuditConfig) Check() error {
	if c.Emitter == nil {
		return trace.BadParameter("missing Emitter")
	}
	if c.Recorder == nil {
		return trace.BadParameter("missing Recorder")
	}
	if c.Database == nil {
		return trace.BadParameter("missing Database")
	}
	if c.Component == "" {
		c.Component = "db:audit"
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	return nil
}

// audit provides methods for emitting database access audit events.
type audit struct {
	// cfg is the audit events emitter configuration.
	cfg AuditConfig
	// log is used for logging
	logger *slog.Logger
}

// NewAudit returns a new instance of the audit events emitter.
func NewAudit(config AuditConfig) (Audit, error) {
	if err := config.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &audit{
		cfg:    config,
		logger: slog.With(teleport.ComponentKey, config.Component),
	}, nil
}

// OnSessionStart emits an audit event when database session starts.
func (a *audit) OnSessionStart(ctx context.Context, session *Session, sessionErr error) {
	event := &events.DatabaseSessionStart{
		Metadata: MakeEventMetadata(session,
			libevents.DatabaseSessionStartEvent,
			libevents.DatabaseSessionStartCode),
		ServerMetadata:   MakeServerMetadata(session),
		UserMetadata:     MakeUserMetadata(session),
		SessionMetadata:  MakeSessionMetadata(session),
		DatabaseMetadata: MakeDatabaseMetadata(session),
		Status: events.Status{
			Success: true,
		},
		PostgresPID: session.PostgresPID,
		ClientMetadata: events.ClientMetadata{
			UserAgent: session.UserAgent,
		},
	}
	event.SetTime(session.StartTime)

	// If the database session wasn't started successfully, emit
	// a failure event with error details.
	if sessionErr != nil {
		event.Metadata.Code = libevents.DatabaseSessionStartFailureCode
		event.Status = events.Status{
			Success:     false,
			Error:       trace.Unwrap(sessionErr).Error(),
			UserMessage: sessionErr.Error(),
		}
	}
	a.EmitEvent(ctx, event)
}

// OnSessionEnd emits an audit event when database session ends.
func (a *audit) OnSessionEnd(ctx context.Context, session *Session) {
	event := &events.DatabaseSessionEnd{
		Metadata: MakeEventMetadata(session,
			libevents.DatabaseSessionEndEvent,
			libevents.DatabaseSessionEndCode),
		UserMetadata:     MakeUserMetadata(session),
		SessionMetadata:  MakeSessionMetadata(session),
		DatabaseMetadata: MakeDatabaseMetadata(session),
		StartTime:        session.StartTime,
	}
	endTime := a.cfg.Clock.Now()
	event.SetTime(endTime)
	event.EndTime = endTime

	a.EmitEvent(ctx, event)
}

// OnQuery emits an audit event when a database query is executed.
func (a *audit) OnQuery(ctx context.Context, session *Session, query Query) {
	event := &events.DatabaseSessionQuery{
		Metadata: MakeEventMetadata(session,
			libevents.DatabaseSessionQueryEvent,
			libevents.DatabaseSessionQueryCode),
		UserMetadata:            MakeUserMetadata(session),
		SessionMetadata:         MakeSessionMetadata(session),
		DatabaseMetadata:        MakeDatabaseMetadata(session),
		DatabaseQuery:           query.Query,
		DatabaseQueryParameters: query.Parameters,
		Status: events.Status{
			Success: true,
		},
	}
	if query.Database != "" {
		event.DatabaseMetadata.DatabaseName = query.Database
	}
	if query.Error != nil {
		event.Metadata.Type = libevents.DatabaseSessionQueryFailedEvent
		event.Metadata.Code = libevents.DatabaseSessionQueryFailedCode
		event.Status = events.Status{
			Success:     false,
			Error:       trace.Unwrap(query.Error).Error(),
			UserMessage: query.Error.Error(),
		}
	}
	a.EmitEvent(ctx, event)
}

// OnResult is called when a database query or command returns.
func (a *audit) OnResult(ctx context.Context, session *Session, result Result) {
	event := &events.DatabaseSessionCommandResult{
		Metadata: MakeEventMetadata(session,
			libevents.DatabaseSessionCommandResultEvent,
			libevents.DatabaseSessionCommandResultCode),
		UserMetadata:     MakeUserMetadata(session),
		SessionMetadata:  MakeSessionMetadata(session),
		DatabaseMetadata: MakeDatabaseMetadata(session),
		Status: events.Status{
			Success:     true,
			UserMessage: result.UserMessage,
		},
		AffectedRecords: result.AffectedRecords,
	}
	if result.Error != nil {
		event.Status.Success = false
		event.Status.Error = trace.Unwrap(result.Error).Error()
	}

	a.RecordEvent(ctx, event)
}

func (a *audit) OnPermissionsUpdate(ctx context.Context, session *Session, entries []events.DatabasePermissionEntry) {
	event := &events.DatabasePermissionUpdate{
		Metadata: MakeEventMetadata(session,
			libevents.DatabaseSessionPermissionsUpdateEvent,
			libevents.DatabaseSessionPermissionUpdateCode),
		UserMetadata:      MakeUserMetadata(session),
		SessionMetadata:   MakeSessionMetadata(session),
		DatabaseMetadata:  MakeDatabaseMetadata(session),
		PermissionSummary: entries,
	}
	a.EmitEvent(ctx, event)
}

func (a *audit) OnDatabaseUserCreate(ctx context.Context, session *Session, err error) {
	event := &events.DatabaseUserCreate{
		Metadata: MakeEventMetadata(session,
			libevents.DatabaseSessionUserCreateEvent,
			libevents.DatabaseSessionUserCreateCode,
		),
		UserMetadata:     MakeUserMetadata(session),
		SessionMetadata:  MakeSessionMetadata(session),
		DatabaseMetadata: MakeDatabaseMetadata(session),

		Status:   events.Status{Success: true},
		Username: session.DatabaseUser,
		Roles:    session.DatabaseRoles,
	}

	if err != nil {
		event.Metadata.Code = libevents.DatabaseSessionUserCreateFailureCode
		event.Status = events.Status{
			Success:     false,
			Error:       trace.Unwrap(err).Error(),
			UserMessage: err.Error(),
		}
	}
	a.EmitEvent(ctx, event)
}

func (a *audit) OnDatabaseUserDeactivate(ctx context.Context, session *Session, delete bool, err error) {
	event := &events.DatabaseUserDeactivate{
		Metadata: MakeEventMetadata(session,
			libevents.DatabaseSessionUserDeactivateEvent,
			libevents.DatabaseSessionUserDeactivateCode,
		),
		UserMetadata:     MakeUserMetadata(session),
		SessionMetadata:  MakeSessionMetadata(session),
		DatabaseMetadata: MakeDatabaseMetadata(session),
		Status:           events.Status{Success: true},
		Username:         session.DatabaseUser,
		Delete:           delete,
	}

	if err != nil {
		event.Metadata.Code = libevents.DatabaseSessionUserDeactivateFailureCode
		event.Status = events.Status{
			Success:     false,
			Error:       trace.Unwrap(err).Error(),
			UserMessage: err.Error(),
		}
	}
	a.EmitEvent(ctx, event)
}

// EmitEvent emits the provided audit event using configured emitter and
// recorder.
func (a *audit) EmitEvent(ctx context.Context, event events.AuditEvent) {
	defer methodCallMetrics("EmitEvent", a.cfg.Component, a.cfg.Database)()
	preparedEvent, err := a.cfg.Recorder.PrepareSessionEvent(event)
	if err != nil {
		a.logger.ErrorContext(ctx, "Failed to setup event",
			"error", err,
			"event_type", event.GetType(),
			"event_id", event.GetID(),
		)
		return
	}
	if err := a.cfg.Recorder.RecordEvent(ctx, preparedEvent); err != nil {
		a.logger.ErrorContext(ctx, "Failed to record session event",
			"error", err,
			"event_type", event.GetType(),
			"event_id", event.GetID(),
		)
	}
	if err := a.cfg.Emitter.EmitAuditEvent(ctx, preparedEvent.GetAuditEvent()); err != nil {
		a.logger.ErrorContext(ctx, "Failed to emit audit event",
			"error", err,
			"event_type", event.GetType(),
			"event_id", event.GetID(),
		)
	}
}

// RecordEvent emits event to the session recording.
func (a *audit) RecordEvent(ctx context.Context, event events.AuditEvent) {
	defer methodCallMetrics("RecordEvent", a.cfg.Component, a.cfg.Database)()
	preparedEvent, err := a.cfg.Recorder.PrepareSessionEvent(event)
	if err != nil {
		a.logger.ErrorContext(ctx, "Failed to setup event",
			"error", err,
			"event_type", event.GetType(),
			"event_id", event.GetID(),
		)
		return
	}
	if err := a.cfg.Recorder.RecordEvent(ctx, preparedEvent); err != nil {
		a.logger.ErrorContext(ctx, "Failed to record session event",
			"error", err,
			"event_type", event.GetType(),
			"event_id", event.GetID(),
		)
	}
}

// MakeEventMetadata returns common event metadata for database session.
func MakeEventMetadata(session *Session, eventType, eventCode string) events.Metadata {
	return events.Metadata{
		Type:        eventType,
		Code:        eventCode,
		ClusterName: session.ClusterName,
	}
}

// MakeServerMetadata returns common server metadata for database session.
func MakeServerMetadata(session *Session) events.ServerMetadata {
	return events.ServerMetadata{
		ServerVersion:   teleport.Version,
		ServerID:        session.HostID,
		ServerNamespace: apidefaults.Namespace,
	}
}

// MakeUserMetadata returns common user metadata for database session.
func MakeUserMetadata(session *Session) events.UserMetadata {
	return session.Identity.GetUserMetadata()
}

// MakeSessionMetadata returns common session metadata for database session.
func MakeSessionMetadata(session *Session) events.SessionMetadata {
	return events.SessionMetadata{
		SessionID:        session.ID,
		WithMFA:          session.Identity.MFAVerified,
		PrivateKeyPolicy: string(session.Identity.PrivateKeyPolicy),
	}
}

// MakeDatabaseMetadata returns common database metadata for database session.
func MakeDatabaseMetadata(session *Session) events.DatabaseMetadata {
	return events.DatabaseMetadata{
		DatabaseService:  session.Database.GetName(),
		DatabaseProtocol: session.Database.GetProtocol(),
		DatabaseURI:      session.Database.GetURI(),
		DatabaseName:     session.DatabaseName,
		DatabaseUser:     session.DatabaseUser,
		DatabaseRoles:    session.DatabaseRoles,
		DatabaseType:     session.Database.GetType(),
		DatabaseOrigin:   session.Database.Origin(),
	}
}
