/*
Copyright 2023 Gravitational, Inc.

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
	"crypto/tls"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/lib/db/oracle/protocol/testdata"
)

func TestClientServerConnReg(t *testing.T) {
	t.Parallel()

	certs, pool := MustCreateSelfSignedCert(t)
	tlsConfig := &tls.Config{Certificates: []tls.Certificate{certs}}
	l, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer l.Close()

	go func() {
		conn, err := l.Accept()
		require.NoError(t, err)
		defer conn.Close()

		tlsConn := tls.Server(conn, tlsConfig)
		require.NoError(t, err)
		oracleClient := NewClientConn(tlsConn)
		defer oracleClient.Close()
		_, err = oracleClient.ReadPacket()
		require.NoError(t, err)
		err = oracleClient.WritePacket(MustParseDumpToPacket(t, testdata.ResendPacketDump))
		require.NoError(t, err)

		// After sending Resend packet the client connection should be upgrade on more time to TLS connection.
		tlsConn = tls.Server(conn, tlsConfig)
		require.NoError(t, err)
		oracleClient = NewClientConn(tlsConn)
		defer oracleClient.Close()

		err = oracleClient.WritePacket(MustParseDumpToPacket(t, testdata.AcceptPacketDump))
		require.NoError(t, err)

		// Check if protocolVersion was negotiation.
		require.NotEqual(t, uint16(0), oracleClient.protocolVersion)
	}()

	serverConn, err := NewServerConn(l.Addr().String(), &tls.Config{
		ServerName: "localhost",
		RootCAs:    pool,
	})
	defer func() {
		require.NoError(t, err)
	}()
	require.NoError(t, err)
	err = serverConn.WritePacket(MustParseDumpToPacket(t, testdata.ConnectPacketDump))
	require.NoError(t, err)

	_, err = serverConn.ReadPacket()
	require.NoError(t, err)

	// Check if protocolVersion was negotiation.
	require.NotEqual(t, uint16(0), serverConn.protocolVersion)
}
