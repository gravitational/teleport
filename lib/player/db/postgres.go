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
	sessionStartAt                 time.Time
	preparedStatements             map[string]string
	lastPreparedStatement          string
	lastPreparedStatementArguments []string
	expectingResult                bool
}

// NewPostgresTranslator constructs a new instance of PostgreSQL translator.
func NewPostgresTranslator() *PostgresTranslator {
	return &PostgresTranslator{
		preparedStatements: make(map[string]string),
		expectingResult:    false,
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
		p.lastPreparedStatement = e.StatementName
		p.lastPreparedStatementArguments = e.Parameters
	case *events.PostgresExecute:
		printEvent := p.generatePreparedStatementPrint(e.Metadata, e.DatabaseMetadata)
		p.expectingResult = printEvent != nil
		p.lastPreparedStatement = ""
		p.lastPreparedStatementArguments = nil
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

func (p *PostgresTranslator) generatePreparedStatementPrint(metadata events.Metadata, databaseMetadata events.DatabaseMetadata) *events.SessionPrint {
	// If there wasn't a bind before the execute, we don't know which prepared
	// statement or arguments were executed. In this case, we skip the prepared
	// statement, ignore any possible result too.
	if p.lastPreparedStatement == "" {
		return nil
	}

	var sb strings.Builder
	if stmt, ok := p.preparedStatements[p.lastPreparedStatement]; ok {
		sb.WriteString(stmt)
	} else {
		fmt.Fprintf(&sb, "EXECUTE %s", p.lastPreparedStatement)
	}

	if len(p.lastPreparedStatementArguments) > 0 {
		sb.WriteString(" (")

		for i, param := range p.lastPreparedStatementArguments {
			fmt.Fprintf(&sb, "$%d = %q", i+1, param)
			if i != len(p.lastPreparedStatementArguments)-1 {
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
