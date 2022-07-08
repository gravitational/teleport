/*
Copyright 2022 Gravitational, Inc.

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

package mysql

import (
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/cloud/clients"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/mysql/protocol"
)

// makeStatementPrepareEvent creates an audit event for MySQL statement prepare
// command.
func makeStatementPrepareEvent(session *clients.Session, packet *protocol.StatementPreparePacket) events.AuditEvent {
	return &events.MySQLStatementPrepare{
		Metadata: common.MakeEventMetadata(session,
			libevents.DatabaseSessionMySQLStatementPrepareEvent,
			libevents.MySQLStatementPrepareCode),
		UserMetadata:     common.MakeUserMetadata(session),
		SessionMetadata:  common.MakeSessionMetadata(session),
		DatabaseMetadata: common.MakeDatabaseMetadata(session),
		Query:            packet.Query(),
	}
}

// makeStatementExecuteEvent creates an audit event for MySQL statement execute
// command.
func makeStatementExecuteEvent(session *clients.Session, packet *protocol.StatementExecutePacket) events.AuditEvent {
	// TODO(greedy52) get parameters from packet and format them for audit.
	return &events.MySQLStatementExecute{
		Metadata: common.MakeEventMetadata(session,
			libevents.DatabaseSessionMySQLStatementExecuteEvent,
			libevents.MySQLStatementExecuteCode),
		UserMetadata:     common.MakeUserMetadata(session),
		SessionMetadata:  common.MakeSessionMetadata(session),
		DatabaseMetadata: common.MakeDatabaseMetadata(session),
		StatementID:      packet.StatementID(),
	}
}

// makeStatementSendLongDataEvent creates an audit event for MySQL statement
// send long data command.
func makeStatementSendLongDataEvent(session *clients.Session, packet *protocol.StatementSendLongDataPacket) events.AuditEvent {
	return &events.MySQLStatementSendLongData{
		Metadata: common.MakeEventMetadata(session,
			libevents.DatabaseSessionMySQLStatementSendLongDataEvent,
			libevents.MySQLStatementSendLongDataCode),
		UserMetadata:     common.MakeUserMetadata(session),
		SessionMetadata:  common.MakeSessionMetadata(session),
		DatabaseMetadata: common.MakeDatabaseMetadata(session),
		StatementID:      packet.StatementID(),
		ParameterID:      uint32(packet.ParameterID()),
		DataSize:         uint32(len(packet.Data())),
	}
}

// makeStatementCloseEvent creates an audit event for MySQL statement close
// command.
func makeStatementCloseEvent(session *clients.Session, packet *protocol.StatementClosePacket) events.AuditEvent {
	return &events.MySQLStatementClose{
		Metadata: common.MakeEventMetadata(session,
			libevents.DatabaseSessionMySQLStatementCloseEvent,
			libevents.MySQLStatementCloseCode),
		UserMetadata:     common.MakeUserMetadata(session),
		SessionMetadata:  common.MakeSessionMetadata(session),
		DatabaseMetadata: common.MakeDatabaseMetadata(session),
		StatementID:      packet.StatementID(),
	}
}

// makeStatementResetEvent creates an audit event for MySQL statement close
// command.
func makeStatementResetEvent(session *clients.Session, packet *protocol.StatementResetPacket) events.AuditEvent {
	return &events.MySQLStatementReset{
		Metadata: common.MakeEventMetadata(session,
			libevents.DatabaseSessionMySQLStatementResetEvent,
			libevents.MySQLStatementResetCode),
		UserMetadata:     common.MakeUserMetadata(session),
		SessionMetadata:  common.MakeSessionMetadata(session),
		DatabaseMetadata: common.MakeDatabaseMetadata(session),
		StatementID:      packet.StatementID(),
	}
}

// makeStatementFetchEvent creates an audit event for MySQL statement fetch
// command.
func makeStatementFetchEvent(session *clients.Session, packet *protocol.StatementFetchPacket) events.AuditEvent {
	return &events.MySQLStatementFetch{
		Metadata: common.MakeEventMetadata(session,
			libevents.DatabaseSessionMySQLStatementFetchEvent,
			libevents.MySQLStatementFetchCode),
		UserMetadata:     common.MakeUserMetadata(session),
		SessionMetadata:  common.MakeSessionMetadata(session),
		DatabaseMetadata: common.MakeDatabaseMetadata(session),
		StatementID:      packet.StatementID(),
		RowsCount:        packet.RowsCount(),
	}
}

// makeStatementBulkExecuteEvent creates an audit event for MySQL statement
// bulk execute command.
func makeStatementBulkExecuteEvent(session *clients.Session, packet *protocol.StatementBulkExecutePacket) events.AuditEvent {
	// TODO(greedy52) get parameters from packet and format them for audit.
	return &events.MySQLStatementBulkExecute{
		Metadata: common.MakeEventMetadata(session,
			libevents.DatabaseSessionMySQLStatementBulkExecuteEvent,
			libevents.MySQLStatementBulkExecuteCode),
		UserMetadata:     common.MakeUserMetadata(session),
		SessionMetadata:  common.MakeSessionMetadata(session),
		DatabaseMetadata: common.MakeDatabaseMetadata(session),
		StatementID:      packet.StatementID(),
	}
}

// makeInitDBEvent creates an audit event for MySQL init DB command.
func makeInitDBEvent(session *clients.Session, packet *protocol.InitDB) events.AuditEvent {
	return &events.MySQLInitDB{
		Metadata: common.MakeEventMetadata(session,
			libevents.DatabaseSessionMySQLInitDBEvent,
			libevents.MySQLInitDBCode),
		UserMetadata:     common.MakeUserMetadata(session),
		SessionMetadata:  common.MakeSessionMetadata(session),
		DatabaseMetadata: common.MakeDatabaseMetadata(session),
		SchemaName:       packet.SchemaName(),
	}
}

// makeCreateDBEvent creates an audit event for MySQL create DB command.
func makeCreateDBEvent(session *clients.Session, packet *protocol.CreateDB) events.AuditEvent {
	return &events.MySQLCreateDB{
		Metadata: common.MakeEventMetadata(session,
			libevents.DatabaseSessionMySQLCreateDBEvent,
			libevents.MySQLCreateDBCode),
		UserMetadata:     common.MakeUserMetadata(session),
		SessionMetadata:  common.MakeSessionMetadata(session),
		DatabaseMetadata: common.MakeDatabaseMetadata(session),
		SchemaName:       packet.SchemaName(),
	}
}

// makeDropDBEvent creates an audit event for MySQL drop DB command.
func makeDropDBEvent(session *clients.Session, packet *protocol.DropDB) events.AuditEvent {
	return &events.MySQLDropDB{
		Metadata: common.MakeEventMetadata(session,
			libevents.DatabaseSessionMySQLDropDBEvent,
			libevents.MySQLDropDBCode),
		UserMetadata:     common.MakeUserMetadata(session),
		SessionMetadata:  common.MakeSessionMetadata(session),
		DatabaseMetadata: common.MakeDatabaseMetadata(session),
		SchemaName:       packet.SchemaName(),
	}
}

// makeShutDownEvent creates an audit event for MySQL shut down command.
func makeShutDownEvent(session *clients.Session, packet *protocol.ShutDown) events.AuditEvent {
	return &events.MySQLShutDown{
		Metadata: common.MakeEventMetadata(session,
			libevents.DatabaseSessionMySQLShutDownEvent,
			libevents.MySQLShutDownCode),
		UserMetadata:     common.MakeUserMetadata(session),
		SessionMetadata:  common.MakeSessionMetadata(session),
		DatabaseMetadata: common.MakeDatabaseMetadata(session),
	}
}

// makeProcessKillEvent creates an audit event for MySQL process kill command.
func makeProcessKillEvent(session *clients.Session, packet *protocol.ProcessKill) events.AuditEvent {
	return &events.MySQLProcessKill{
		Metadata: common.MakeEventMetadata(session,
			libevents.DatabaseSessionMySQLProcessKillEvent,
			libevents.MySQLProcessKillCode),
		UserMetadata:     common.MakeUserMetadata(session),
		SessionMetadata:  common.MakeSessionMetadata(session),
		DatabaseMetadata: common.MakeDatabaseMetadata(session),
		ProcessID:        packet.ProcessID(),
	}
}

// makeDebugEvent creates an audit event for MySQL debug command.
func makeDebugEvent(session *clients.Session, packet *protocol.Debug) events.AuditEvent {
	return &events.MySQLDebug{
		Metadata: common.MakeEventMetadata(session,
			libevents.DatabaseSessionMySQLDebugEvent,
			libevents.MySQLDebugCode),
		UserMetadata:     common.MakeUserMetadata(session),
		SessionMetadata:  common.MakeSessionMetadata(session),
		DatabaseMetadata: common.MakeDatabaseMetadata(session),
	}
}

// makeRefreshEvent creates an audit event for MySQL refresh command.
func makeRefreshEvent(session *clients.Session, packet *protocol.Refresh) events.AuditEvent {
	return &events.MySQLRefresh{
		Metadata: common.MakeEventMetadata(session,
			libevents.DatabaseSessionMySQLRefreshEvent,
			libevents.MySQLRefreshCode),
		UserMetadata:     common.MakeUserMetadata(session),
		SessionMetadata:  common.MakeSessionMetadata(session),
		DatabaseMetadata: common.MakeDatabaseMetadata(session),
		Subcommand:       packet.Subcommand(),
	}
}
