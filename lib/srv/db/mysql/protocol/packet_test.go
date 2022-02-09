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
			bytes: []byte{0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
	}

	sampleQueryPacket = &Query{
		packet: packet{
			bytes: []byte{
				0x09, 0x00, 0x00, 0x00, // header
				0x03, // type
				0x73, 0x65, 0x6c, 0x65, 0x63, 0x74, 0x20, 0x31,
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
		message: "denied",
	}

	sampleErrorWithSQLStatePacket = &Error{
		packet: packet{
			bytes: []byte{
				0x0f, 0x00, 0x00, 0x00, // header
				0xff,       // type
				0x51, 0x04, // error code
				0x23, 0x48, 0x59, 0x30, 0x30, 0x30, // #HY000
				0x64, 0x65, 0x6e, 0x69, 0x65, 0x64, // message
			},
		},
		message: "denied",
	}

	sampleQuitPacket = &Quit{
		packet: packet{
			bytes: []byte{0x01, 0x00, 0x00, 0x00, 0x01},
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
