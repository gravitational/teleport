// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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
	"encoding/binary"
	"io"
	"net"
	"testing"

	"cloud.google.com/go/alloydb/connectors/apiv1beta/connectorspb"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// TestMetadataExchangeAlloyDB tests the AlloyDB metadata exchange protocol.
func TestMetadataExchangeAlloyDB(t *testing.T) {
	t.Parallel()

	// testToken is a fixed access token used for testing.
	const testToken = "test-token"

	// testCases defines the test scenarios.
	testCases := []struct {
		name string

		// server represents the server-side of the metadata exchange.
		// It reads the client's request, writes a response, and returns any
		// unexpected errors that occurred during the process.
		server func(conn net.Conn) error

		wantErr string
	}{
		{
			name: "all good",
			server: func(conn net.Conn) error {
				// Read the request from the client.
				req, err := readRequest(conn)
				if err != nil {
					return trace.Wrap(err, "server failed to read request")
				}

				// Verify the request payload.
				if req.GetOauth2Token() != testToken {
					return trace.BadParameter("server expected different token, got %q", req.GetOauth2Token())
				}

				// Send a successful response.
				return writeResponse(conn, &connectorspb.MetadataExchangeResponse{
					ResponseCode: connectorspb.MetadataExchangeResponse_OK,
				})
			},
			wantErr: "",
		},
		{
			name: "server responds with error",
			server: func(conn net.Conn) error {
				// Read and discard the request from the client.
				if _, err := readRequest(conn); err != nil {
					return trace.Wrap(err)
				}

				// Send an error response.
				return writeResponse(conn, &connectorspb.MetadataExchangeResponse{
					ResponseCode: connectorspb.MetadataExchangeResponse_ERROR,
				})
			},
			wantErr: "metadata exchange failed: ERROR",
		},
		{
			name: "server sends malformed protobuf response",
			server: func(conn net.Conn) error {
				// Read and discard the request from the client.
				if _, err := readRequest(conn); err != nil {
					return trace.Wrap(err)
				}

				// Write a garbage response.
				malformedResp := []byte("this-is-not-a-protobuf")
				var buf []byte
				buf = binary.BigEndian.AppendUint32(buf, uint32(len(malformedResp)))
				buf = append(buf, malformedResp...)
				_, err := conn.Write(buf)
				return trace.Wrap(err)
			},
			wantErr: "cannot parse invalid wire-format data",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clientSide, serverSide := net.Pipe()

			errChan := make(chan error, 1)

			// Run the mock server in a separate goroutine.
			go func() {
				errChan <- tc.server(serverSide)
				_ = serverSide.Close()
			}()

			// Run the function under test.
			clientErr := metadataExchangeAlloyDB(testToken, clientSide)

			serverErr := <-errChan

			// First, check if the mock server itself ran into an unexpected error.
			require.NoError(t, serverErr, "mock server encountered an error")

			// Then, assert the outcome from the client's perspective.
			if tc.wantErr != "" {
				require.Error(t, clientErr)
				require.ErrorContains(t, clientErr, tc.wantErr)
			} else {
				require.NoError(t, clientErr)
			}
		})
	}
}

// readRequest is a helper to read a length-prefixed protobuf message from the connection.
func readRequest(conn io.Reader) (*connectorspb.MetadataExchangeRequest, error) {
	// Read message length.
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, lenBuf); err != nil {
		return nil, trace.Wrap(err)
	}
	msgLen := binary.BigEndian.Uint32(lenBuf)

	// A basic sanity check for the message length to avoid huge allocations.
	const maxLen = 1 << 20 // 1MB
	if msgLen > maxLen {
		return nil, trace.BadParameter("message length %d exceeds maximum of %d", msgLen, maxLen)
	}

	// Read message body.
	msgBuf := make([]byte, msgLen)
	if _, err := io.ReadFull(conn, msgBuf); err != nil {
		return nil, trace.Wrap(err)
	}

	// Unmarshal.
	var req connectorspb.MetadataExchangeRequest
	if err := proto.Unmarshal(msgBuf, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	return &req, nil
}

// writeResponse is a helper to write a length-prefixed protobuf message to the connection.
func writeResponse(conn io.Writer, resp *connectorspb.MetadataExchangeResponse) error {
	// Marshal response.
	m, err := proto.Marshal(resp)
	if err != nil {
		return trace.Wrap(err)
	}

	// Prepend length and write.
	buf := make([]byte, 4+len(m))
	binary.BigEndian.PutUint32(buf[:4], uint32(len(m)))
	copy(buf[4:], m)

	_, err = conn.Write(buf)
	return trace.Wrap(err)
}
