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

package postgres

import (
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/cloud/clients"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

// makeParseEvent returns audit event for Postgres Parse wire message which
// is sent by the client when creating a new prepared statement.
func makeParseEvent(session *clients.Session, statementName, query string) events.AuditEvent {
	return &events.PostgresParse{
		Metadata: common.MakeEventMetadata(session,
			libevents.DatabaseSessionPostgresParseEvent,
			libevents.PostgresParseCode),
		UserMetadata:     common.MakeUserMetadata(session),
		SessionMetadata:  common.MakeSessionMetadata(session),
		DatabaseMetadata: common.MakeDatabaseMetadata(session),
		StatementName:    statementName,
		Query:            query,
	}
}

// makeBindEvent returns audit event for Postgres Bind wire message which is
// sent by the client when binding a prepared statement to parameters into a
// destination portal.
func makeBindEvent(session *clients.Session, statementName, portalName string, parameters []string) events.AuditEvent {
	return &events.PostgresBind{
		Metadata: common.MakeEventMetadata(session,
			libevents.DatabaseSessionPostgresBindEvent,
			libevents.PostgresBindCode),
		UserMetadata:     common.MakeUserMetadata(session),
		SessionMetadata:  common.MakeSessionMetadata(session),
		DatabaseMetadata: common.MakeDatabaseMetadata(session),
		StatementName:    statementName,
		PortalName:       portalName,
		Parameters:       parameters,
	}
}

// makeExecuteEvent returns audit event for Postgres Execute wire message which
// is sent by the client when executing a destination portal.
func makeExecuteEvent(session *clients.Session, portalName string) events.AuditEvent {
	return &events.PostgresExecute{
		Metadata: common.MakeEventMetadata(session,
			libevents.DatabaseSessionPostgresExecuteEvent,
			libevents.PostgresExecuteCode),
		UserMetadata:     common.MakeUserMetadata(session),
		SessionMetadata:  common.MakeSessionMetadata(session),
		DatabaseMetadata: common.MakeDatabaseMetadata(session),
		PortalName:       portalName,
	}
}

// makeCloseEvent returns audit event for Postgres Close wire message which is
// sent by the client when closing a prepared statement or a destination portal.
func makeCloseEvent(session *clients.Session, statementName, portalName string) events.AuditEvent {
	return &events.PostgresClose{
		Metadata: common.MakeEventMetadata(session,
			libevents.DatabaseSessionPostgresCloseEvent,
			libevents.PostgresCloseCode),
		UserMetadata:     common.MakeUserMetadata(session),
		SessionMetadata:  common.MakeSessionMetadata(session),
		DatabaseMetadata: common.MakeDatabaseMetadata(session),
		StatementName:    statementName,
		PortalName:       portalName,
	}
}

// makeFuncCallEvent returns audit event for Postgres FunctionCall wire message
// which is sent by the client when invoking an internal function.
func makeFuncCallEvent(session *clients.Session, funcOID uint32, funcArgs []string) events.AuditEvent {
	return &events.PostgresFunctionCall{
		Metadata: common.MakeEventMetadata(session,
			libevents.DatabaseSessionPostgresFunctionEvent,
			libevents.PostgresFunctionCallCode),
		UserMetadata:     common.MakeUserMetadata(session),
		SessionMetadata:  common.MakeSessionMetadata(session),
		DatabaseMetadata: common.MakeDatabaseMetadata(session),
		FunctionOID:      funcOID,
		FunctionArgs:     funcArgs,
	}
}
