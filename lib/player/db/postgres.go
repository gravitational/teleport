// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package db

import (
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/teleport/api/types/events"
)

// PostgresTranslator is responsible for converting PostgreSQL session recording
// events into their text representation.
type PostgresTranslator struct {
	sessionStartAt time.Time
	// preparedStatements is a map of parsed prepared statements.
	preparedStatements map[string]string
	// preparedStatementPortalBindings is a map of binded prepared statements
	// portals and their arguments.
	preparedStatemetnPortalBidings map[string]preparedStatementBind
	// expectingResult determines if the translator is expecting a command
	// result. This is used to skip commands and their results.
	expectingResult bool
}

type preparedStatementBind struct {
	stmtName string
	params   []string
}

func newPreparedStatementBind(stmt string, params []string) preparedStatementBind {
	return preparedStatementBind{stmt, params}
}

// NewPostgresTranslator constructs a new instance of PostgreSQL translator.
func NewPostgresTranslator() *PostgresTranslator {
	return &PostgresTranslator{
		preparedStatements:             make(map[string]string),
		preparedStatemetnPortalBidings: make(map[string]preparedStatementBind),
		expectingResult:                false,
	}
}

// TranslateEvent converts PostgreSQL audit events into session print events.
func (p *PostgresTranslator) TranslateEvent(evt events.AuditEvent) *events.SessionPrint {
	switch e := evt.(type) {
	case *events.DatabaseSessionQuery:
		p.expectingResult = true
		return p.generatePrintEvent(e.Metadata, p.generateCommandPrint(e.DatabaseMetadata, e.DatabaseQuery))
	case *events.DatabaseSessionCommandResult:
		if p.expectingResult {
			p.expectingResult = false
			return p.generatePrintEvent(e.Metadata, p.generateResultPrint(e))
		}
	case *events.PostgresParse:
		p.expectingResult = false
		p.preparedStatements[e.StatementName] = e.Query
	case *events.PostgresBind:
		p.expectingResult = false
		p.preparedStatemetnPortalBidings[e.PortalName] = newPreparedStatementBind(e.StatementName, e.Parameters)
	case *events.PostgresExecute:
		printEvent := p.generatePreparedStatementPrint(e.Metadata, e.DatabaseMetadata, e.PortalName)
		p.expectingResult = printEvent != nil
		return printEvent
	case *events.PostgresFunctionCall:
		p.expectingResult = false
	case *events.DatabaseSessionStart:
		p.sessionStartAt = e.Time
	case *events.DatabaseSessionEnd:
	}

	return nil
}

const lineBreak = "\r\n"

func (p *PostgresTranslator) generateCommandPrint(metadata events.DatabaseMetadata, command string) string {
	return lineBreak + fmt.Sprintf("%s=> %s", metadata.DatabaseName, command) + lineBreak
}

func (p *PostgresTranslator) generatePreparedStatementPrint(metadata events.Metadata, databaseMetadata events.DatabaseMetadata, portalName string) *events.SessionPrint {
	bind, hasBind := p.preparedStatemetnPortalBidings[portalName]

	// In case of an unbinded portal, we cannot present the execution properly,
	// so we skip those executions.
	if !hasBind {
		return nil
	}

	var sb strings.Builder
	if stmt, ok := p.preparedStatements[bind.stmtName]; ok {
		sb.WriteString(stmt)
	} else {
		fmt.Fprintf(&sb, "EXECUTE %s", bind.stmtName)
	}

	if len(bind.params) > 0 {
		sb.WriteString(" (")

		for i, param := range bind.params {
			fmt.Fprintf(&sb, "$%d = %q", i+1, param)
			if i != len(bind.params)-1 {
				sb.WriteString(", ")
			}
		}

		sb.WriteString(")")
	}

	return p.generatePrintEvent(metadata, p.generateCommandPrint(databaseMetadata, sb.String()))
}

func (p *PostgresTranslator) generateResultPrint(evt *events.DatabaseSessionCommandResult) string {
	if !evt.Status.Success {
		return evt.Status.Error + lineBreak
	}

	return fmt.Sprintf("SUCCESS%s(%d row%s affected)", lineBreak, evt.AffectedRecords, pluralizeRows(int(evt.AffectedRecords))) + lineBreak
}

func (p *PostgresTranslator) generatePrintEvent(metadata events.Metadata, data string) *events.SessionPrint {
	return &events.SessionPrint{
		Metadata:          metadata,
		Data:              []byte(data),
		DelayMilliseconds: metadata.Time.Sub(p.sessionStartAt).Milliseconds(),
	}
}

func pluralizeRows(count int) string {
	if count == 1 {
		return ""
	}

	return "s"
}
