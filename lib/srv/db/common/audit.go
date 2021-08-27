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

package common

import (
	"context"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// Audit defines an interface for database access audit events logger.
type Audit interface {
	// OnSessionStart is called on successful/unsuccessful database session start.
	OnSessionStart(ctx context.Context, session *Session, sessionErr error)
	// OnSessionEnd is called when database session terminates.
	OnSessionEnd(ctx context.Context, session *Session)
	// OnQuery is called when a database query or command is executed.
	OnQuery(ctx context.Context, session *Session, query Query)
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

// AuditConfig is the audit events emitter configuration.
type AuditConfig struct {
	// Emitter is used to emit audit events.
	Emitter events.Emitter
}

// Check validates the config.
func (c *AuditConfig) Check() error {
	if c.Emitter == nil {
		return trace.BadParameter("missing Emitter")
	}
	return nil
}

// audit provides methods for emitting database access audit events.
type audit struct {
	// cfg is the audit events emitter configuration.
	cfg AuditConfig
	// log is used for logging
	log logrus.FieldLogger
}

// NewAudit returns a new instance of the audit events emitter.
func NewAudit(config AuditConfig) (Audit, error) {
	if err := config.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &audit{
		cfg: config,
		log: logrus.WithField(trace.Component, "db:audit"),
	}, nil
}

// OnSessionStart emits an audit event when database session starts.
func (a *audit) OnSessionStart(ctx context.Context, session *Session, sessionErr error) {
	event := &events.DatabaseSessionStart{
		Metadata: events.Metadata{
			Type:        libevents.DatabaseSessionStartEvent,
			Code:        libevents.DatabaseSessionStartCode,
			ClusterName: session.ClusterName,
		},
		ServerMetadata: events.ServerMetadata{
			ServerID:        session.Server.GetHostID(),
			ServerNamespace: apidefaults.Namespace,
		},
		UserMetadata: events.UserMetadata{
			User:         session.Identity.Username,
			Impersonator: session.Identity.Impersonator,
		},
		SessionMetadata: events.SessionMetadata{
			SessionID: session.ID,
			WithMFA:   session.Identity.MFAVerified,
		},
		DatabaseMetadata: events.DatabaseMetadata{
			DatabaseService:  session.Database.GetName(),
			DatabaseProtocol: session.Database.GetProtocol(),
			DatabaseURI:      session.Database.GetURI(),
			DatabaseName:     session.DatabaseName,
			DatabaseUser:     session.DatabaseUser,
		},
		Status: events.Status{
			Success: true,
		},
	}
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
	a.emitAuditEvent(ctx, event)
}

// OnSessionEnd emits an audit event when database session ends.
func (a *audit) OnSessionEnd(ctx context.Context, session *Session) {
	a.emitAuditEvent(ctx, &events.DatabaseSessionEnd{
		Metadata: events.Metadata{
			Type:        libevents.DatabaseSessionEndEvent,
			Code:        libevents.DatabaseSessionEndCode,
			ClusterName: session.ClusterName,
		},
		UserMetadata: events.UserMetadata{
			User:         session.Identity.Username,
			Impersonator: session.Identity.Impersonator,
		},
		SessionMetadata: events.SessionMetadata{
			SessionID: session.ID,
			WithMFA:   session.Identity.MFAVerified,
		},
		DatabaseMetadata: events.DatabaseMetadata{
			DatabaseService:  session.Database.GetName(),
			DatabaseProtocol: session.Database.GetProtocol(),
			DatabaseURI:      session.Database.GetURI(),
			DatabaseName:     session.DatabaseName,
			DatabaseUser:     session.DatabaseUser,
		},
	})
}

// OnQuery emits an audit event when a database query is executed.
func (a *audit) OnQuery(ctx context.Context, session *Session, query Query) {
	database := session.DatabaseName
	if query.Database != "" {
		database = query.Database
	}
	event := &events.DatabaseSessionQuery{
		Metadata: events.Metadata{
			Type:        libevents.DatabaseSessionQueryEvent,
			Code:        libevents.DatabaseSessionQueryCode,
			ClusterName: session.ClusterName,
		},
		UserMetadata: events.UserMetadata{
			User:         session.Identity.Username,
			Impersonator: session.Identity.Impersonator,
		},
		SessionMetadata: events.SessionMetadata{
			SessionID: session.ID,
			WithMFA:   session.Identity.MFAVerified,
		},
		DatabaseMetadata: events.DatabaseMetadata{
			DatabaseService:  session.Database.GetName(),
			DatabaseProtocol: session.Database.GetProtocol(),
			DatabaseURI:      session.Database.GetURI(),
			DatabaseName:     database,
			DatabaseUser:     session.DatabaseUser,
		},
		DatabaseQuery:           query.Query,
		DatabaseQueryParameters: query.Parameters,
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
	a.emitAuditEvent(ctx, event)
}

func (a *audit) emitAuditEvent(ctx context.Context, event events.AuditEvent) {
	if err := a.cfg.Emitter.EmitAuditEvent(ctx, event); err != nil {
		a.log.WithError(err).Errorf("Failed to emit audit event: %v.", event)
	}
}
