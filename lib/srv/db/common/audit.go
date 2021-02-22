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

	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/defaults"
	libevents "github.com/gravitational/teleport/lib/events"

	"github.com/gravitational/trace"
)

// AuditConfig is the audit events emitter configuration.
type AuditConfig struct {
	// StreamWriter is used to emit audit events.
	StreamWriter libevents.StreamWriter
}

// Check validates the config.
func (c *AuditConfig) Check() error {
	if c.StreamWriter == nil {
		return trace.BadParameter("missing StreamWriter")
	}
	return nil
}

// Audit provides methods for emitting database access audit events.
type Audit struct {
	// cfg is the audit events emitter configuration.
	cfg AuditConfig
}

// NewAudit returns a new instance of the audit events emitter.
func NewAudit(config AuditConfig) (*Audit, error) {
	if err := config.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Audit{
		cfg: config,
	}, nil
}

// OnSessionStart emits an audit event when database session starts.
func (a *Audit) OnSessionStart(ctx context.Context, session Session, sessionErr error) error {
	event := &events.DatabaseSessionStart{
		Metadata: events.Metadata{
			Type:        libevents.DatabaseSessionStartEvent,
			Code:        libevents.DatabaseSessionStartCode,
			ClusterName: session.ClusterName,
		},
		ServerMetadata: events.ServerMetadata{
			ServerID:        session.Server.GetHostID(),
			ServerNamespace: defaults.Namespace,
		},
		UserMetadata: events.UserMetadata{
			User: session.Identity.Username,
		},
		SessionMetadata: events.SessionMetadata{
			SessionID: session.ID,
		},
		Status: events.Status{
			Success: true,
		},
		DatabaseMetadata: events.DatabaseMetadata{
			DatabaseService:  session.Server.GetName(),
			DatabaseProtocol: session.Server.GetProtocol(),
			DatabaseURI:      session.Server.GetURI(),
			DatabaseName:     session.DatabaseName,
			DatabaseUser:     session.DatabaseUser,
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
	err := a.cfg.StreamWriter.EmitAuditEvent(ctx, event)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// OnSessionEnd emits an audit event when database session ends.
func (a *Audit) OnSessionEnd(ctx context.Context, session Session) error {
	err := a.cfg.StreamWriter.EmitAuditEvent(ctx, &events.DatabaseSessionEnd{
		Metadata: events.Metadata{
			Type:        libevents.DatabaseSessionEndEvent,
			Code:        libevents.DatabaseSessionEndCode,
			ClusterName: session.ClusterName,
		},
		UserMetadata: events.UserMetadata{
			User: session.Identity.Username,
		},
		SessionMetadata: events.SessionMetadata{
			SessionID: session.ID,
		},
		DatabaseMetadata: events.DatabaseMetadata{
			DatabaseService:  session.Server.GetName(),
			DatabaseProtocol: session.Server.GetProtocol(),
			DatabaseURI:      session.Server.GetURI(),
			DatabaseName:     session.DatabaseName,
			DatabaseUser:     session.DatabaseUser,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// OnQuery emits an audit event when a database query is executed.
func (a *Audit) OnQuery(ctx context.Context, session Session, query string) error {
	err := a.cfg.StreamWriter.EmitAuditEvent(ctx, &events.DatabaseSessionQuery{
		Metadata: events.Metadata{
			Type:        libevents.DatabaseSessionQueryEvent,
			Code:        libevents.DatabaseSessionQueryCode,
			ClusterName: session.ClusterName,
		},
		UserMetadata: events.UserMetadata{
			User: session.Identity.Username,
		},
		SessionMetadata: events.SessionMetadata{
			SessionID: session.ID,
		},
		DatabaseMetadata: events.DatabaseMetadata{
			DatabaseService:  session.Server.GetName(),
			DatabaseProtocol: session.Server.GetProtocol(),
			DatabaseURI:      session.Server.GetURI(),
			DatabaseName:     session.DatabaseName,
			DatabaseUser:     session.DatabaseUser,
		},
		DatabaseQuery: query,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
