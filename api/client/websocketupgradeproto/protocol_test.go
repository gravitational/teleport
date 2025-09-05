/*
Copyright 2025 Gravitational, Inc.

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
package websocketupgradeproto

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/api/constants"
)

func TestProtocol(t *testing.T) {
	t.Parallel()

	server := createHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := NewServerConnection(slog.Default(), r, w)
		assert.NoError(t, err, "Failed to create server connection")
		_, err = conn.Write([]byte(payload))
		assert.NoError(t, err, "Failed to write to server connection")
		// Read the message back to verify it was sent correctly.
		buf := make([]byte, len(payload))
		n, err := conn.Read(buf)
		assert.NoError(t, err, "Failed to read from server connection")
		assert.Equal(t, len(payload), n, "Read length should match written length")

		conn.Close()
	}))

	u, err := url.Parse(server.URL)
	assert.NoError(t, err, "Failed to parse server URL")

	client, err := NewWebSocketALPNClientConn(context.Background(), WebSocketALPNClientConnConfig{
		URL:       u,
		TLSConfig: tlsConfigForHTTPServer(t, server),
		Protocols: []string{constants.WebAPIConnUpgradeProtocolWebSocketClose},
		Dialer:    (&net.Dialer{}).DialContext,
	})
	assert.NoError(t, err, "Failed to create client connection")
	defer client.Close()

	// Write a message to the server.
	_, err = client.Write([]byte(payload))
	assert.NoError(t, err, "Failed to write to client connection")
	// Read the message back to verify it was sent correctly.
	buf := make([]byte, len(payload))
	n, err := client.Read(buf)
	assert.NoError(t, err, "Failed to read from client connection")
	assert.Equal(t, len(payload), n, "Read length should match written length")
	assert.Equal(t, payload, string(buf[:n]), "Read payload should match written payload")
	_, err = client.Read(buf)
	assert.ErrorIs(t, err, io.EOF, "Expected EOF after reading all data")
}

func tlsConfigForHTTPServer(t *testing.T, srv *httptest.Server) *tls.Config {
	t.Helper()
	rootCA := x509.NewCertPool()
	rootCA.AddCert(srv.Certificate())

	return &tls.Config{
		RootCAs: rootCA,
	}
}

func TestProtocolPayloadPreserved(t *testing.T) {
	t.Parallel()
	dataSent := make(chan struct{})
	server := createHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := NewServerConnection(slog.Default(), r, w)
		assert.NoError(t, err, "Failed to create server connection")
		_, err = conn.Write([]byte(payload))
		assert.NoError(t, err, "Failed to write to server connection")

		// ensure the client has written the data before closing the connection
		<-dataSent
		// Close the connection.
		err = conn.Close()
		assert.NoError(t, err, "Failed to close server connection")

		// Read the data back to ensure it was preserved.
		buf := make([]byte, len(payload))
		n, err := conn.Read(buf)
		assert.NoError(t, err, "Failed to read from server connection")
		assert.Equal(t, len(payload), n, "Read length should match written length")

		n, err = conn.Read(buf)
		assert.ErrorIs(t, err, io.EOF, "Expected EOF after reading all data")
		assert.Equal(t, 0, n, "Read length should be zero after EOF")
	}))

	u, err := url.Parse(server.URL)
	assert.NoError(t, err, "Failed to parse server URL")

	client, err := NewWebSocketALPNClientConn(context.Background(), WebSocketALPNClientConnConfig{
		URL:       u,
		TLSConfig: tlsConfigForHTTPServer(t, server),
		Protocols: []string{constants.WebAPIConnUpgradeProtocolWebSocketClose},
		Dialer:    (&net.Dialer{}).DialContext,
	})
	assert.NoError(t, err, "Failed to create client connection")
	defer client.Close()

	// Read the message back to verify it was sent correctly.
	buf := make([]byte, len(payload))
	n, err := client.Read(buf)
	assert.NoError(t, err, "Failed to read from client connection")
	assert.Equal(t, len(payload), n, "Read length should match written length")
	assert.Equal(t, payload, string(buf[:n]), "Read payload should match written payload")

	// Write a message to the server.
	_, err = client.Write([]byte(payload))
	assert.NoError(t, err, "Failed to write to client connection")

	close(dataSent)

	_, err = client.Read(buf)
	assert.ErrorIs(t, err, io.EOF, "Expected EOF after reading all data")
}
