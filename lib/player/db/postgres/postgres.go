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

package postgres

import (
	"fmt"
	"io"

	"github.com/jackc/pgtype"
	"github.com/olekukonko/tablewriter"

	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/trace"
)

type playerState struct {
	w io.Writer

	ci    *pgtype.ConnInfo
	table *tablewriter.Table

	columnNames []string
	columnTypes []uint32
	totalRows   int
}

func (s *playerState) SetColumns(evt *events.PostgresRowDescription) error {
	if s.table != nil {
		// TODO(gabrielcorado): check if we should handle out-of-order events
		s.Reset()
	}

	columnLen := len(evt.Fields)
	s.columnNames = make([]string, columnLen)
	s.columnTypes = make([]uint32, columnLen)
	for i, field := range evt.Fields {
		s.columnNames[i] = field.Name
		s.columnTypes[i] = field.DataTypeOID
	}

	s.table = tablewriter.NewWriter(s.w)
	s.table.SetAutoFormatHeaders(false)
	s.table.SetAutoWrapText(false)
	s.table.SetHeader(s.columnNames)
	// Set borders to psql default format.
	s.table.SetBorder(false)
	s.table.SetRowLine(false)
	s.table.SetReflowDuringAutoWrap(false)
	s.table.SetColumnAlignment([]int{tablewriter.ALIGN_LEFT})

	return nil
}

// TODO(gabrielcorado): use columnTypes to define table alignment.
func (s *playerState) Append(evt *events.PostgresDataRow) error {
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

func (s *playerState) Reset() {
	s.table = nil
	s.columnNames = nil
	s.columnTypes = nil
	s.totalRows = 0
}

func (s *playerState) Flush() error {
	defer s.Reset()

	if s.table != nil {
		s.table.Render()
		fmt.Fprintf(s.w, "(%d row%s)\r\n", s.totalRows, pluralize(s.totalRows))
		return nil
	}

	return nil
}

func (s *playerState) Error(evt *events.PostgresErrorResponse) error {
	defer s.Reset()

	// TODO(gabrielcorado): add missing error details.
	if _, err := fmt.Fprintf(s.w, "%s: %s\r\n", evt.Severity, evt.Message); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (s *playerState) Query(evt *events.DatabaseSessionQuery) error {
	defer s.Reset()

	if _, err := fmt.Fprintf(s.w, "%s=# %s\r\n", evt.DatabaseName, evt.DatabaseQuery); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (s *playerState) Complete(evt *events.PostgresCommandComplete) error {
	if s.table != nil {
		return nil // noop on selects
	}

	defer s.Reset()
	if _, err := fmt.Fprintln(s.w, string(evt.CommandTags)); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func newState(w io.Writer) *playerState {
	return &playerState{
		w:  w,
		ci: pgtype.NewConnInfo(),
	}
}

type Player struct {
	state *playerState
}

func New(w io.Writer) *Player {
	return &Player{state: newState(w)}
}

// works towards state progression.
func (p *Player) Event(e events.AuditEvent) error {
	switch evt := e.(type) {
	case *events.PostgresRowDescription:
		if err := p.state.SetColumns(evt); err != nil {
			return trace.Wrap(err)
		}
	case *events.PostgresDataRow:
		if err := p.state.Append(evt); err != nil {
			return trace.Wrap(err)
		}
	case *events.PostgresErrorResponse:
		if err := p.state.Error(evt); err != nil {
			return trace.Wrap(err)
		}
	case *events.PostgresCommandComplete:
		if err := trace.NewAggregate(p.state.Complete(evt), p.state.Flush()); err != nil {
			return trace.Wrap(err)
		}
	case *events.DatabaseSessionQuery:
		if err := p.state.Query(evt); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func pluralize(n int) string {
	if n == 1 {
		return ""
	}

	return "s"
}
