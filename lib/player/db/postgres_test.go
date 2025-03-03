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
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils"
)

func TestPostgresRecording(t *testing.T) {
	fixedTime := time.Now()

	for name, test := range map[string]struct {
		events         []events.AuditEvent
		expectedPrints [][]byte
	}{
		"session start": {
			events: []events.AuditEvent{
				&events.DatabaseSessionStart{
					Metadata:         events.Metadata{Time: fixedTime},
					DatabaseMetadata: events.DatabaseMetadata{DatabaseService: "my-pg-instance"},
				},
			},
			expectedPrints: [][]byte{
				[]byte("Session started to database \"my-pg-instance\" at " + utils.HumanTimeFormat(fixedTime) + "\r\n"),
			},
		},
		"session end": {
			events: []events.AuditEvent{
				&events.DatabaseSessionEnd{
					Metadata: events.Metadata{Time: fixedTime},
				},
			},
			expectedPrints: [][]byte{
				[]byte("\r\nSession ended at " + utils.HumanTimeFormat(fixedTime) + "\r\n"),
			},
		},
		"queries": {
			events: []events.AuditEvent{
				&events.DatabaseSessionQuery{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					DatabaseQuery:    "SELECT 1;",
				},
				&events.DatabaseSessionCommandResult{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					Status:           events.Status{Success: true},
					AffectedRecords:  1,
				},
				&events.DatabaseSessionQuery{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					DatabaseQuery:    "SELECT * from events;",
				},
				&events.DatabaseSessionCommandResult{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					Status:           events.Status{Success: true},
					AffectedRecords:  15,
				},
			},
			expectedPrints: [][]byte{
				[]byte("\r\ntest=> SELECT 1;\r\n"),
				[]byte("SUCCESS\r\n(1 row affected)\r\n"),
				[]byte("\r\ntest=> SELECT * from events;\r\n"),
				[]byte("SUCCESS\r\n(15 rows affected)\r\n"),
			},
		},
		"queries with errors": {
			events: []events.AuditEvent{
				&events.DatabaseSessionQuery{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					DatabaseQuery:    "SELECT err;",
				},
				&events.DatabaseSessionCommandResult{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					Status:           events.Status{Success: false, Error: "ERROR: column error"},
				},
			},
			expectedPrints: [][]byte{
				[]byte("\r\ntest=> SELECT err;\r\n"),
				[]byte("ERROR: column error\r\n"),
			},
		},
		"prepared statements": {
			events: []events.AuditEvent{
				&events.PostgresParse{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					StatementName:    "test",
					Query:            "SELECT $1::varchar",
				},
				&events.PostgresBind{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					StatementName:    "test",
					Parameters:       []string{"hello world"},
				},
				&events.PostgresExecute{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
				},
				&events.DatabaseSessionCommandResult{
					Status:          events.Status{Success: true},
					AffectedRecords: 1,
				},
				&events.PostgresBind{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					StatementName:    "test",
					Parameters:       []string{"hello new execution"},
				},
				&events.PostgresExecute{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
				},
				&events.DatabaseSessionCommandResult{
					Status:          events.Status{Success: true},
					AffectedRecords: 1,
				},
			},
			expectedPrints: [][]byte{
				nil,
				nil,
				[]byte("\r\ntest=> SELECT $1::varchar ($1 = \"hello world\")\r\n"),
				[]byte("SUCCESS\r\n(1 row affected)\r\n"),
				nil,
				[]byte("\r\ntest=> SELECT $1::varchar ($1 = \"hello new execution\")\r\n"),
				[]byte("SUCCESS\r\n(1 row affected)\r\n"),
				nil,
				[]byte("\r\ntest=> EXECUTE random ($1 = \"hello new execution\")\r\n"),
				[]byte("SUCCESS\r\n(1 row affected)\r\n"),
			},
		},
		"multiple parameters prepared statements": {
			events: []events.AuditEvent{
				&events.PostgresParse{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					StatementName:    "test",
					Query:            "SELECT $1::varchar",
				},
				&events.PostgresBind{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					StatementName:    "test",
					Parameters:       []string{"hello", "world"},
				},
				&events.PostgresExecute{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
				},
				&events.DatabaseSessionCommandResult{
					Status:          events.Status{Success: true},
					AffectedRecords: 1,
				},
			},
			expectedPrints: [][]byte{
				nil,
				nil,
				[]byte("\r\ntest=> SELECT $1::varchar ($1 = \"hello\", $2 = \"world\")\r\n"),
				[]byte("SUCCESS\r\n(1 row affected)\r\n"),
			},
		},
		"unknown prepared statements": {
			events: []events.AuditEvent{
				&events.PostgresBind{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					StatementName:    "random",
					Parameters:       []string{"hello world"},
				},
				&events.PostgresExecute{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
				},
				&events.DatabaseSessionCommandResult{
					Status:          events.Status{Success: true},
					AffectedRecords: 1,
				},
			},
			expectedPrints: [][]byte{
				nil,
				[]byte("\r\ntest=> EXECUTE random ($1 = \"hello world\")\r\n"),
				[]byte("SUCCESS\r\n(1 row affected)\r\n"),
			},
		},
		"prepared statements without binding": {
			events: []events.AuditEvent{
				&events.PostgresExecute{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
				},
				&events.DatabaseSessionCommandResult{
					Status:          events.Status{Success: true},
					AffectedRecords: 1,
				},
			},
			expectedPrints: [][]byte{
				nil,
				nil,
			},
		},
		"prepared statements with multiple portal bindings": {
			events: []events.AuditEvent{
				&events.PostgresParse{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					StatementName:    "test",
					Query:            "SELECT $1::varchar || ' ' || $2::varchar",
				},
				&events.PostgresBind{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					StatementName:    "test",
					PortalName:       "hello1",
					Parameters:       []string{"hello", "first"},
				},
				&events.PostgresBind{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					StatementName:    "test",
					PortalName:       "hello2",
					Parameters:       []string{"hello", "second"},
				},
				&events.PostgresExecute{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					PortalName:       "hello1",
				},
				&events.DatabaseSessionCommandResult{
					Status:          events.Status{Success: true},
					AffectedRecords: 1,
				},
				&events.PostgresExecute{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					PortalName:       "hello2",
				},
				&events.DatabaseSessionCommandResult{
					Status:          events.Status{Success: true},
					AffectedRecords: 1,
				},
			},
			expectedPrints: [][]byte{
				nil,
				nil,
				nil,
				[]byte("\r\ntest=> SELECT $1::varchar || ' ' || $2::varchar ($1 = \"hello\", $2 = \"first\")\r\n"),
				[]byte("SUCCESS\r\n(1 row affected)\r\n"),
				[]byte("\r\ntest=> SELECT $1::varchar || ' ' || $2::varchar ($1 = \"hello\", $2 = \"second\")\r\n"),
				[]byte("SUCCESS\r\n(1 row affected)\r\n"),
			},
		},
		"prepared statements mixed name and unnamed portals": {
			events: []events.AuditEvent{
				&events.PostgresParse{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					StatementName:    "test",
					Query:            "SELECT $1::varchar || ' ' || $2::varchar",
				},
				&events.PostgresBind{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					StatementName:    "test",
					PortalName:       "hello1",
					Parameters:       []string{"hello", "first"},
				},
				&events.PostgresBind{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					StatementName:    "test",
					PortalName:       "hello2",
					Parameters:       []string{"hello", "second"},
				},
				// unnamed bind
				&events.PostgresBind{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					StatementName:    "test",
					Parameters:       []string{"hello", "unamed1"},
				},
				&events.PostgresExecute{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
				},
				&events.DatabaseSessionCommandResult{
					Status:          events.Status{Success: true},
					AffectedRecords: 1,
				},
				// named hello1
				&events.PostgresExecute{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					PortalName:       "hello1",
				},
				&events.DatabaseSessionCommandResult{
					Status:          events.Status{Success: true},
					AffectedRecords: 1,
				},
				// unnamed 2 bind
				&events.PostgresBind{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					StatementName:    "test",
					Parameters:       []string{"hello", "unamed2"},
				},
				&events.PostgresExecute{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
				},
				&events.DatabaseSessionCommandResult{
					Status:          events.Status{Success: true},
					AffectedRecords: 1,
				},
				// named hello2
				&events.PostgresExecute{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					PortalName:       "hello2",
				},
				&events.DatabaseSessionCommandResult{
					Status:          events.Status{Success: true},
					AffectedRecords: 1,
				},
			},
			expectedPrints: [][]byte{
				nil,
				nil,
				nil,
				nil, // unnamed bind
				[]byte("\r\ntest=> SELECT $1::varchar || ' ' || $2::varchar ($1 = \"hello\", $2 = \"unamed1\")\r\n"),
				[]byte("SUCCESS\r\n(1 row affected)\r\n"),
				[]byte("\r\ntest=> SELECT $1::varchar || ' ' || $2::varchar ($1 = \"hello\", $2 = \"first\")\r\n"),
				[]byte("SUCCESS\r\n(1 row affected)\r\n"),
				nil, // unnamed 2 bind
				[]byte("\r\ntest=> SELECT $1::varchar || ' ' || $2::varchar ($1 = \"hello\", $2 = \"unamed2\")\r\n"),
				[]byte("SUCCESS\r\n(1 row affected)\r\n"),
				[]byte("\r\ntest=> SELECT $1::varchar || ' ' || $2::varchar ($1 = \"hello\", $2 = \"second\")\r\n"),
				[]byte("SUCCESS\r\n(1 row affected)\r\n"),
			},
		},
		"skip unexpected command results": {
			events: []events.AuditEvent{
				&events.DatabaseSessionCommandResult{
					Status:          events.Status{Success: true},
					AffectedRecords: 1,
				},
			},
			expectedPrints: [][]byte{
				nil,
			},
		},
		"queries with line spacing": {
			events: []events.AuditEvent{
				&events.DatabaseSessionQuery{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					DatabaseQuery:    "SELECT \n1;",
				},
				&events.DatabaseSessionCommandResult{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					Status:           events.Status{Success: true},
					AffectedRecords:  1,
				},
				&events.DatabaseSessionQuery{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					DatabaseQuery:    "SELECT *\nfrom events\nlimit 1;",
				},
				&events.DatabaseSessionCommandResult{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					Status:           events.Status{Success: true},
					AffectedRecords:  1,
				},
				&events.DatabaseSessionQuery{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					DatabaseQuery:    "SELECT *\r\nfrom events\r\nlimit 1;",
				},
				&events.DatabaseSessionCommandResult{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					Status:           events.Status{Success: true},
					AffectedRecords:  1,
				},
				&events.DatabaseSessionQuery{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					DatabaseQuery:    "SELECT *\r\nfrom events\r\nlimit 1;",
				},
				&events.DatabaseSessionCommandResult{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					Status:           events.Status{Success: true},
					AffectedRecords:  1,
				},
				&events.DatabaseSessionQuery{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					DatabaseQuery:    "SELECT *\t\r\n   from events\t\r\n    limit 1;",
				},
				&events.DatabaseSessionCommandResult{
					DatabaseMetadata: events.DatabaseMetadata{DatabaseName: "test"},
					Status:           events.Status{Success: true},
					AffectedRecords:  1,
				},
			},
			expectedPrints: [][]byte{
				[]byte("\r\ntest=> SELECT\r\n       1;\r\n"),
				[]byte("SUCCESS\r\n(1 row affected)\r\n"),
				[]byte("\r\ntest=> SELECT *\r\n       from events\r\n       limit 1;\r\n"),
				[]byte("SUCCESS\r\n(1 row affected)\r\n"),
				[]byte("\r\ntest=> SELECT *\r\n       from events\r\n       limit 1;\r\n"),
				[]byte("SUCCESS\r\n(1 row affected)\r\n"),
				[]byte("\r\ntest=> SELECT *\r\n       from events\r\n       limit 1;\r\n"),
				[]byte("SUCCESS\r\n(1 row affected)\r\n"),
				[]byte("\r\ntest=> SELECT *\r\n       from events\r\n       limit 1;\r\n"),
				[]byte("SUCCESS\r\n(1 row affected)\r\n"),
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			translator := NewPostgresTranslator()

			for i, evt := range test.events {
				expectedResult := test.expectedPrints[i]
				result := translator.TranslateEvent(evt)

				if expectedResult == nil {
					require.Nil(t, result, "expected event to be skipped")
					continue
				}

				require.NotNil(t, result, "expected event to not be skipped")
				require.True(t, bytes.Equal(expectedResult, result.Data), "expected data %q but got %q", expectedResult, result.Data)
			}
		})
	}
}
