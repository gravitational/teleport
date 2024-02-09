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

package protocol

import (
	"bytes"
	"io"
	"net"
	"testing"
	"testing/iotest"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

var (
	sampleOKPacket = &OK{
		packet: packet{
			bytes: []byte{
				0x03, 0x00, 0x00, 0x00, // header
				0x00, // type
				0x00, 0x00,
			},
		},
	}

	sampleQueryPacket = &Query{
		packet: packet{
			bytes: []byte{
				0x09, 0x00, 0x00, 0x00, // header
				0x03,                                           // type
				0x73, 0x65, 0x6c, 0x65, 0x63, 0x74, 0x20, 0x31, // query
			},
		},
		query: "select 1",
	}

	sampleErrorPacket = &Error{
		packet: packet{
			bytes: []byte{
				0x09, 0x00, 0x00, 0x00, // header
				0xff,       // type
				0x51, 0x04, // error code
				0x64, 0x65, 0x6e, 0x69, 0x65, 0x64, // message
			},
		},
		Code:    1105,
		Message: "denied",
	}

	sampleErrorWithSQLStatePacket = &Error{
		packet: packet{
			bytes: []byte{
				0x0f, 0x00, 0x00, 0x00, // header
				0xff,       // type
				0x51, 0x04, // error code
				0x23,                         // marker #
				0x48, 0x59, 0x30, 0x30, 0x30, // state - HY000
				0x64, 0x65, 0x6e, 0x69, 0x65, 0x64, // message
			},
		},
		Code:    1105,
		Message: "denied",
	}

	sampleQuitPacket = &Quit{
		packet: packet{
			bytes: []byte{
				0x01, 0x00, 0x00, 0x00, // header
				0x01, //type
			},
		},
	}

	sampleChangeUserPacket = &ChangeUser{
		packet: packet{
			bytes: []byte{
				0x05, 0x00, 0x00, 0x04, // header
				0x11,                   // type
				0x62, 0x6f, 0x62, 0x00, // null terminated "bob"
			},
		},
		user: "bob",
	}

	sampleInitDBPacket = &InitDB{
		schemaNamePacket: schemaNamePacket{
			packet: packet{
				bytes: []byte{
					0x05, 0x00, 0x00, 0x00, // header
					0x02,                   // type
					0x74, 0x65, 0x73, 0x74, // schema "test"
				},
			},
			schemaName: "test",
		},
	}

	sampleCreateDBPacket = &CreateDB{
		schemaNamePacket: schemaNamePacket{
			packet: packet{
				bytes: []byte{
					0x05, 0x00, 0x00, 0x00, // header
					0x05,                   // type
					0x74, 0x65, 0x73, 0x74, // schema "test"
				},
			},
			schemaName: "test",
		},
	}

	sampleDropDBPacket = &DropDB{
		schemaNamePacket: schemaNamePacket{
			packet: packet{
				bytes: []byte{
					0x05, 0x00, 0x00, 0x00, // header
					0x06,                   // type
					0x74, 0x65, 0x73, 0x74, // schema "test"
				},
			},
			schemaName: "test",
		},
	}

	sampleShutDownPacket = &ShutDown{
		packet: packet{
			bytes: []byte{
				0x02, 0x00, 0x00, 0x00, // header
				0x08, // type
				0x00, // optional shutdown type
			},
		},
	}

	sampleProcessKillPacket = &ProcessKill{
		packet: packet{
			bytes: []byte{
				0x05, 0x00, 0x00, 0x00, // header
				0x0c,                   // type
				0x15, 0x00, 0x00, 0x00, // process ID
			},
		},
		processID: 21,
	}

	sampleDebugPacket = &Debug{
		packet: packet{
			bytes: []byte{
				0x01, 0x00, 0x00, 0x00, // header
				0x0d, // type
			},
		},
	}

	sampleRefreshPacket = &Refresh{
		packet: packet{
			bytes: []byte{
				0x02, 0x00, 0x00, 0x00, // header
				0x07, // type
				0x40, // subcommand
			},
		},
		subcommand: "REFRESH_SLAVE",
	}

	sampleStatementPreparePacket = &StatementPreparePacket{
		packet: packet{
			bytes: []byte{
				0x09, 0x00, 0x00, 0x00, // header
				0x16,                                           // type
				0x73, 0x65, 0x6c, 0x65, 0x63, 0x74, 0x20, 0x31, // query
			},
		},
		query: "select 1",
	}

	sampleStatementSendLongDataPacket = &StatementSendLongDataPacket{
		statementIDPacket: statementIDPacket{
			packet: packet{
				bytes: []byte{
					0x0a, 0x00, 0x00, 0x00, // header
					0x18,                   // type
					0x05, 0x00, 0x00, 0x00, // statement ID
					0x02, 0x00, // parameter ID
					0x62, 0x6f, 0x62, //data
				},
			},
			statementID: 5,
		},
		parameterID: 2,
		data:        []byte("bob"),
	}

	sampleStatementExecutePacket = &StatementExecutePacket{
		statementIDPacket: statementIDPacket{
			packet: packet{
				bytes: []byte{
					0x1e, 0x00, 0x00, 0x00, // header
					0x17,                   // type
					0x02, 0x00, 0x00, 0x00, // statement ID
					0x00,                   // cursor flag
					0x01, 0x00, 0x00, 0x00, // iteration count
					0x00, // nullbit map
					0x01, // new-params-bound flag

					// https://dev.mysql.com/doc/internals/en/com-query-response.html#column-type
					0xfe, 0x00, // param 1 type - MYSQL_TYPE_STRING
					0x08, 0x00, // param 2 type - MYSQL_TYPE_LONGLONG

					0x05, 0x68, 0x65, 0x6c, 0x6c, 0x6f, // param 1 value - "hello"
					0xc8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // param 2 value - 200
				},
			},
			statementID: 2,
		},
		cursorFlag: 0x00,
		iterations: 1,
		nullBitmapAndParameters: []byte{
			0x00,       // null bitmap
			0x01,       // new-params-bound flag
			0xfe, 0x00, // param 1 type - MYSQL_TYPE_STRING
			0x08, 0x00, // param 2 type - MYSQL_TYPE_LONGLONG
			0x05, 0x68, 0x65, 0x6c, 0x6c, 0x6f, // param 1 value - "hello"
			0xc8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // param 2 value - 200
		},
	}

	sampleStatementClosePacket = &StatementClosePacket{
		statementIDPacket: statementIDPacket{
			packet: packet{
				bytes: []byte{
					0x05, 0x00, 0x00, 0x00, // header
					0x19,                   // type
					0x01, 0x00, 0x00, 0x00, // statement ID
				},
			},
			statementID: 1,
		},
	}

	sampleStatementResetPacket = &StatementResetPacket{
		statementIDPacket: statementIDPacket{
			packet: packet{
				bytes: []byte{
					0x05, 0x00, 0x00, 0x00, // header
					0x1a,                   // type
					0x01, 0x00, 0x00, 0x00, // statement ID
				},
			},
			statementID: 1,
		},
	}

	sampleStatementFetchPacket = &StatementFetchPacket{
		statementIDPacket: statementIDPacket{
			packet: packet{
				bytes: []byte{
					0x09, 0x00, 0x00, 0x00, // header
					0x1c,                   // type
					0x01, 0x00, 0x00, 0x00, // statement ID
					0x0a, 0x00, 0x00, 0x00, // num rows
				},
			},
			statementID: 1,
		},
		rowsCount: 10,
	}

	sampleStatementBulkExecutePacket = &StatementBulkExecutePacket{
		statementIDPacket: statementIDPacket{
			packet: packet{
				bytes: []byte{
					0x15, 0x00, 0x00, 0x00, // header
					0xfa,                   // type
					0x01, 0x00, 0x00, 0x00, // statement ID
					0x80, 0x00, // bulkFlag
					0xfe, 0x00, // param 1 type - MYSQL_TYPE_STRING
					0x08, 0x00, // param 2 type - MYSQL_TYPE_LONGLONG
					0x01,                                                 // param 1 - null
					0x00, 0xc8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // param 2 value - 200
				},
			},
			statementID: 1,
		},
		bulkFlag: 128,
		parameters: []byte{
			0xfe, 0x00, // param 1 type - MYSQL_TYPE_STRING
			0x08, 0x00, // param 2 type - MYSQL_TYPE_LONGLONG
			0x01,                                                 // param 1 - null
			0x00, 0xc8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // param 2 value - 200
		},
	}
)

func TestParsePacket(t *testing.T) {
	tests := []struct {
		name           string
		input          io.Reader
		expectedPacket Packet
		expectErrorIs  func(error) bool
	}{
		{
			name:          "network error",
			input:         iotest.ErrReader(&net.OpError{}),
			expectErrorIs: trace.IsConnectionProblem,
		},
		{
			name:          "not enough data for header",
			input:         bytes.NewBuffer([]byte{0x00}),
			expectErrorIs: isUnexpectedEOFError,
		},
		{
			name:          "not enough data for payload",
			input:         bytes.NewBuffer([]byte{0xff, 0xff, 0xff, 0x00, 0x01}),
			expectErrorIs: isUnexpectedEOFError,
		},
		{
			name: "unrecognized type",
			input: bytes.NewBuffer([]byte{
				0x01, 0x00, 0x00, 0x00, // header
				0x44, // type
			}),
			expectedPacket: &Generic{
				packet: packet{
					bytes: []byte{0x01, 0x00, 0x00, 0x00, 0x44},
				},
			},
		},
		{
			name:           "OK_HEADER",
			input:          bytes.NewBuffer(sampleOKPacket.Bytes()),
			expectedPacket: sampleOKPacket,
		},
		{
			name:           "ERR_HEADER",
			input:          bytes.NewBuffer(sampleErrorPacket.Bytes()),
			expectedPacket: sampleErrorPacket,
		},
		{
			name:           "ERR_HEADER protocol 4.1",
			input:          bytes.NewBuffer(sampleErrorWithSQLStatePacket.Bytes()),
			expectedPacket: sampleErrorWithSQLStatePacket,
		},
		{
			name:           "COM_QUERY",
			input:          bytes.NewBuffer(sampleQueryPacket.Bytes()),
			expectedPacket: sampleQueryPacket,
		},
		{
			name:           "COM_QUIT",
			input:          bytes.NewBuffer(sampleQuitPacket.Bytes()),
			expectedPacket: sampleQuitPacket,
		},
		{
			name:           "COM_CHANGE_USER",
			input:          bytes.NewBuffer(sampleChangeUserPacket.Bytes()),
			expectedPacket: sampleChangeUserPacket,
		},
		{
			name: "COM_CHANGE_USER invalid",
			input: bytes.NewBuffer([]byte{
				0x04, 0x00, 0x00, 0x00, // header
				0x11,             // type
				0x62, 0x6f, 0x62, // missing null at the end of the string
			}),
			expectErrorIs: trace.IsBadParameter,
		},
		{
			name:           "COM_INIT_DB",
			input:          bytes.NewBuffer(sampleInitDBPacket.Bytes()),
			expectedPacket: sampleInitDBPacket,
		},
		{
			name:           "COM_CREATE_DB",
			input:          bytes.NewBuffer(sampleCreateDBPacket.Bytes()),
			expectedPacket: sampleCreateDBPacket,
		},
		{
			name:           "COM_DROP_DB",
			input:          bytes.NewBuffer(sampleDropDBPacket.Bytes()),
			expectedPacket: sampleDropDBPacket,
		},
		{
			name:           "COM_SHUTDOWN",
			input:          bytes.NewBuffer(sampleShutDownPacket.Bytes()),
			expectedPacket: sampleShutDownPacket,
		},
		{
			name:           "COM_PROCESS_KILL",
			input:          bytes.NewBuffer(sampleProcessKillPacket.Bytes()),
			expectedPacket: sampleProcessKillPacket,
		},
		{
			name:           "COM_DEBUG",
			input:          bytes.NewBuffer(sampleDebugPacket.Bytes()),
			expectedPacket: sampleDebugPacket,
		},
		{
			name:           "COM_REFRESH",
			input:          bytes.NewBuffer(sampleRefreshPacket.Bytes()),
			expectedPacket: sampleRefreshPacket,
		},
		{
			name:           "COM_STMT_PREPARE",
			input:          bytes.NewBuffer(sampleStatementPreparePacket.Bytes()),
			expectedPacket: sampleStatementPreparePacket,
		},
		{
			name:           "COM_STMT_SEND_LONG_DATA",
			input:          bytes.NewBuffer(sampleStatementSendLongDataPacket.Bytes()),
			expectedPacket: sampleStatementSendLongDataPacket,
		},
		{
			name:           "COM_STMT_EXECUTE",
			input:          bytes.NewBuffer(sampleStatementExecutePacket.Bytes()),
			expectedPacket: sampleStatementExecutePacket,
		},
		{
			name:           "COM_STMT_CLOSE",
			input:          bytes.NewBuffer(sampleStatementClosePacket.Bytes()),
			expectedPacket: sampleStatementClosePacket,
		},
		{
			name:           "COM_STMT_RESET",
			input:          bytes.NewBuffer(sampleStatementResetPacket.Bytes()),
			expectedPacket: sampleStatementResetPacket,
		},
		{
			name:           "COM_STMT_FETCH",
			input:          bytes.NewBuffer(sampleStatementFetchPacket.Bytes()),
			expectedPacket: sampleStatementFetchPacket,
		},
		{
			name:           "COM_STMT_BULK_EXECUTE",
			input:          bytes.NewBuffer(sampleStatementBulkExecutePacket.Bytes()),
			expectedPacket: sampleStatementBulkExecutePacket,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			actualPacket, err := ParsePacket(test.input)
			if test.expectErrorIs != nil {
				require.Error(t, err)
				require.True(t, test.expectErrorIs(err))
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expectedPacket, actualPacket)
			}
		})
	}
}

func isUnexpectedEOFError(err error) bool {
	return trace.Unwrap(err) == io.ErrUnexpectedEOF
}
