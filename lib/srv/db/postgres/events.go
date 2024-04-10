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
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/jackc/pgproto3/v2"
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

func makeBackendEvent(ctx context.Context, i pgproto3.BackendMessage) events.AuditEvent {
	switch message := i.(type) {
	case *pgproto3.RowDescription:
		event := makeRowDescriptionEvent(message)
		return event
	case *pgproto3.DataRow:
		return makeDataRowEvent(message)
	case *pgproto3.CommandComplete:
		return makeCommandCompleteEvent(message)
	case *pgproto3.ReadyForQuery:
		return makeReadyForQueryEvent(message)
	case *pgproto3.ErrorResponse:
		return makeErrorResponseEvent(message)
	default:
		return nil
	}
}

func makeBackendEventMetadata(message pgproto3.BackendMessage) events.Metadata {
	// TODO eventType and code
	messageType := fmt.Sprintf("%T", message)
	eventType := "Postgres" + strings.TrimPrefix(messageType, "*pgproto3.")
	now := time.Now().UTC().Round(time.Millisecond)
	return events.Metadata{
		Type: eventType,
		Time: now,
		Code: "POSTGRESDEMO",
	}
}

func makeRowDescriptionEvent(message *pgproto3.RowDescription) events.AuditEvent {
	event := &events.PostgresRowDescription{
		Metadata: makeBackendEventMetadata(message),
		Fields:   make([]*events.PostgresFieldDescription, 0, len(message.Fields)),
	}
	for _, field := range message.Fields {
		event.Fields = append(event.Fields, &events.PostgresFieldDescription{
			Name:                 string(field.Name),
			TableOID:             field.TableOID,
			TableAttributeNumber: uint32(field.TableAttributeNumber),
			DataTypeOID:          field.DataTypeOID,
			DataTypeSize:         int32(field.DataTypeSize),
			TypeModifier:         field.TypeModifier,
			Format:               int32(field.Format),
		})
	}
	return event
}

func makeDataRowEvent(message *pgproto3.DataRow) events.AuditEvent {
	event := &events.PostgresDataRow{
		Metadata: makeBackendEventMetadata(message),
		Values:   make([][]byte, 0, len(message.Values)),
	}
	for _, values := range message.Values {
		event.Values = append(event.Values, bytes.Clone(values))
	}
	return event
}

func makeCommandCompleteEvent(message *pgproto3.CommandComplete) events.AuditEvent {
	return &events.PostgresCommandComplete{
		Metadata: makeBackendEventMetadata(message),
		// TODO Fix typo
		CommandTags: bytes.Clone(message.CommandTag),
	}
}

func makeReadyForQueryEvent(message *pgproto3.ReadyForQuery) events.AuditEvent {
	return &events.PostgresReadyForQuery{
		Metadata: makeBackendEventMetadata(message),
		TxStatus: int32(message.TxStatus),
	}
}

func makeErrorResponseEvent(message *pgproto3.ErrorResponse) events.AuditEvent {
	return &events.PostgresErrorResponse{
		Metadata:            makeBackendEventMetadata(message),
		Severity:            message.Severity,
		SeverityUnlocalized: message.SeverityUnlocalized,
		ErrorCode:           message.Code,
		Message:             message.Message,
		Detail:              message.Detail,
		Hint:                message.Hint,
		Position:            message.Position,
		InternalPosition:    message.InternalPosition,
		InternalQuery:       message.InternalQuery,
		Where:               message.Where,
		// TODO Fix typo
		SchemeName:     message.SchemaName,
		TableName:      message.TableName,
		ColumnName:     message.ColumnName,
		DataTypeName:   message.DataTypeName,
		ConstraintName: message.ConstraintName,
		File:           message.File,
		Line:           message.Line,
		Routine:        message.Routine,
		// TODO UnknownFields
	}
}
