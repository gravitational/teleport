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
	"fmt"

	"github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/mysql/protocol"
	"github.com/siddontang/go-mysql/mysql"
	"github.com/sirupsen/logrus"
)

// makeStatementPrepareEvent creates an audit event for MySQL statement prepare
// command.
func makeStatementPrepareEvent(session *common.Session, packet *protocol.StatementPreparePacket) events.AuditEvent {
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
func makeStatementExecuteEvent(session *common.Session, packet *protocol.StatementExecutePacket, parameterDefinitions []mysql.Field) events.AuditEvent {
	formatedParameters := []string{}
	if len(parameterDefinitions) > 0 {
		if values, ok := packet.Parameters(parameterDefinitions); ok {
			formatedParameters = formatParameters(parameterDefinitions, values)
		} else {
			logrus.Debugf("Failed to get parameters from MySQL packet %v.", packet.Bytes())
		}
	}
	return &events.MySQLStatementExecute{
		Metadata: common.MakeEventMetadata(session,
			libevents.DatabaseSessionMySQLStatementExecuteEvent,
			libevents.MySQLStatementExecuteCode),
		UserMetadata:     common.MakeUserMetadata(session),
		SessionMetadata:  common.MakeSessionMetadata(session),
		DatabaseMetadata: common.MakeDatabaseMetadata(session),
		StatementID:      packet.StatementID(),
		Parameters:       formatedParameters,
	}
}

// makeStatementSendLongDataEvent creates an audit event for MySQL statement
// send long data command.
func makeStatementSendLongDataEvent(session *common.Session, packet *protocol.StatementSendLongDataPacket) events.AuditEvent {
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
func makeStatementCloseEvent(session *common.Session, packet *protocol.StatementClosePacket) events.AuditEvent {
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
func makeStatementResetEvent(session *common.Session, packet *protocol.StatementResetPacket) events.AuditEvent {
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

// formatParameters converts parameters from MySQL field types into their
// string representations for including in the audit log.
func formatParameters(definitions []mysql.Field, values []interface{}) (formatted []string) {
	if len(definitions) != len(values) {
		logrus.Warnf("MySQL parameter definitions and values don't match: %#v %#v.", definitions, values)
		return nil
	}

	for i, definition := range definitions {
		switch definition.Type {
		case mysql.MYSQL_TYPE_NULL:
			formatted = append(formatted, "<nil>")

		case mysql.MYSQL_TYPE_GEOMETRY:
			formatted = append(formatted, "<geometry>")

		default:
			if definition.Flag&mysql.BINARY_FLAG == mysql.BINARY_FLAG {
				formatted = append(formatted, "<binary>")
			} else {
				formatted = append(formatted, fmt.Sprintf("%v", values[i]))
			}
		}
	}
	return formatted
}
