/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

// Package player includes an API to play back recorded sessions.
package player

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jackc/pgtype"
	"github.com/olekukonko/tablewriter"

	"github.com/gravitational/teleport/api/types/events"
)

type translater interface {
	Translate(evt events.AuditEvent) (events.AuditEvent, bool)
}

type noopTranslater struct {
}

func (n *noopTranslater) Translate(evt events.AuditEvent) (events.AuditEvent, bool) {
	return evt, true
}

type postgresTranslater struct {
	buf *bytes.Buffer

	ci    *pgtype.ConnInfo
	table *tablewriter.Table

	columnNames []string
	columnTypes []uint32
	totalRows   int
	startTime   time.Time
}

func newPostgresTranslater() *postgresTranslater {
	return &postgresTranslater{
		buf: new(bytes.Buffer),
		ci:  pgtype.NewConnInfo(),
	}
}

func (s *postgresTranslater) setColumns(evt *events.PostgresRowDescription) error {
	if s.table != nil {
		// TODO(gabrielcorado): check if we should handle out-of-order events
		s.reset()
	}

	columnLen := len(evt.Fields)
	s.columnNames = make([]string, columnLen)
	s.columnTypes = make([]uint32, columnLen)
	for i, field := range evt.Fields {
		s.columnNames[i] = field.Name
		s.columnTypes[i] = field.DataTypeOID
	}

	s.table = tablewriter.NewWriter(s.buf)
	s.table.SetAutoFormatHeaders(false)
	s.table.SetAutoWrapText(false)
	s.table.SetHeader(s.columnNames)
	// Set borders to psql default format.
	s.table.SetBorder(false)
	s.table.SetRowLine(false)
	s.table.SetReflowDuringAutoWrap(false)
	s.table.SetColumnAlignment([]int{tablewriter.ALIGN_LEFT})
	s.table.SetNewLine("\r\n")

	return nil
}

// TODO(gabrielcorado): use columnTypes to define table alignment.
func (s *postgresTranslater) append(evt *events.PostgresDataRow) error {
	if s.table == nil || len(s.columnNames) == 0 || len(s.columnTypes) == 0 {
		return trace.BadParameter("Unable to print rows: columns not defined.")
	}

	rowValues := make([]string, len(evt.Values))
	for i, value := range evt.Values {
		rowValues[i] = string(value)
	}

	s.table.Append(rowValues)
	s.totalRows++
	return nil
}

func (s *postgresTranslater) reset() {
	s.buf.Reset()
	s.table = nil
	s.columnNames = nil
	s.columnTypes = nil
	s.totalRows = 0
}

func (s *postgresTranslater) flush(metadata events.Metadata) (events.AuditEvent, bool) {
	defer s.reset()

	if s.table != nil {
		s.table.Render()
		fmt.Fprintf(s.buf, "(%d row%s)\r\n\r\n", s.totalRows, pluralize(s.totalRows))
	}

	if s.buf.Len() == 0 {
		return nil, false
	}

	// TODO(gabrielcorado): check if we should enforce the \r\n here.
	// append CR LF
	// s.buf.Write([]byte{13, 10})

	return &events.SessionPrint{
		Metadata:          metadata,
		Data:              s.buf.Bytes(),
		DelayMilliseconds: metadata.Time.Sub(s.startTime).Milliseconds(),
	}, true
}

func (s *postgresTranslater) error(evt *events.PostgresErrorResponse) error {
	// TODO(gabrielcorado): add missing error details.
	if _, err := fmt.Fprintf(s.buf, "%s: %s\r\n", evt.Severity, evt.Message); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (s *postgresTranslater) query(evt *events.DatabaseSessionQuery) error {
	query := strings.ReplaceAll(evt.DatabaseQuery, "\r\n", "\n")
	query = strings.ReplaceAll(evt.DatabaseQuery, "\n", "\r\n")
	if _, err := fmt.Fprintf(s.buf, "%s=# %s\r\n", evt.DatabaseName, query); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (s *postgresTranslater) complete(evt *events.PostgresCommandComplete) error {
	if s.table != nil {
		return nil // noop on selects
	}

	if _, err := fmt.Fprint(s.buf, string(evt.CommandTags)); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (p *postgresTranslater) Translate(event events.AuditEvent) (events.AuditEvent, bool) {
	switch evt := event.(type) {
	case *events.DatabaseSessionStart:
		p.startTime = evt.Time
		return &events.SessionStart{
			Metadata: evt.Metadata,
		}, true
	case *events.DatabaseSessionEnd:
		return &events.SessionEnd{
			Metadata: evt.Metadata,
		}, true
	case *events.DatabaseSessionQuery:
		_ = p.query(evt)
	case *events.PostgresRowDescription:
		_ = p.setColumns(evt)
	case *events.PostgresDataRow:
		_ = p.append(evt)
	case *events.PostgresErrorResponse:
		_ = p.error(evt)
	case *events.PostgresCommandComplete:
		_ = p.complete(evt)
		return p.flush(evt.Metadata)
	case *events.PostgresReadyForQuery:
		// TODO(gabrielcorado): double check this.
		return p.flush(evt.Metadata)
	}

	return nil, false
}

func pluralize(n int) string {
	if n == 1 {
		return ""
	}

	return "s"
}
