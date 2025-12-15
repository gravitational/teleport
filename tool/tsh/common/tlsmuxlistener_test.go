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

package common

import (
	"crypto/tls"
	"crypto/x509/pkix"
	"errors"
	"io"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestTLSMuxListenerDataTransfer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	keyPEM, certPEM, err := tlsca.GenerateSelfSignedCA(pkix.Name{CommonName: "tsh"}, []string{"localhost"}, defaults.CATTL)
	require.NoError(t, err)

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
	}

	muxListener := NewTLSMuxListener(ln, tlsConfig)

	// Start a goroutine to accept connections and echo data back.
	go func() {
		for {
			conn, errAccept := muxListener.Accept()
			if errAccept != nil {
				return // Exit on listener closure.
			}

			// Echo the received data back to the client.
			go func() {
				defer conn.Close()
				buf := make([]byte, 1024)
				for {
					n, err := conn.Read(buf)
					if errors.Is(err, io.EOF) {
						return
					}
					assert.NoError(t, err)

					_, err = conn.Write(buf[:n])
					assert.NoError(t, err)
					if err != nil {
						return
					}
				}
			}()
		}
	}()

	// Test plain connection.
	t.Run("Plain Connection", func(t *testing.T) {
		conn, err := net.Dial("tcp", ln.Addr().String())
		require.NoError(t, err)
		defer conn.Close()

		message := "Hello, non-TLS!"
		_, err = conn.Write([]byte(message))
		require.NoError(t, err)

		buf := make([]byte, len(message))
		_, err = conn.Read(buf)
		require.NoError(t, err)
		require.Equal(t, message, string(buf), "Plain connection message mismatch")
	})

	// Test TLS connection.
	t.Run("TLS Connection", func(t *testing.T) {
		tlsConn, err := tls.Dial("tcp", ln.Addr().String(), &tls.Config{
			InsecureSkipVerify: true,
		})
		require.NoError(t, err)
		defer tlsConn.Close()

		message := "Hello, TLS!"
		_, err = tlsConn.Write([]byte(message))
		require.NoError(t, err)

		buf := make([]byte, len(message))
		_, err = tlsConn.Read(buf)
		require.NoError(t, err)
		require.Equal(t, message, string(buf), "TLS connection message mismatch")
	})
}
