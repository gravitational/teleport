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

package postgres

import (
	"github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

// makeParseEvent returns audit event for Postgres Parse wire message which
// is sent by the client when creating a new prepared statement.
func makeParseEvent(session *common.Session, statementName, query string) events.AuditEvent {
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
func makeBindEvent(session *common.Session, statementName, portalName string, parameters []string) events.AuditEvent {
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
func makeExecuteEvent(session *common.Session, portalName string) events.AuditEvent {
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
func makeCloseEvent(session *common.Session, statementName, portalName string) events.AuditEvent {
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
func makeFuncCallEvent(session *common.Session, funcOID uint32, funcArgs []string) events.AuditEvent {
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
