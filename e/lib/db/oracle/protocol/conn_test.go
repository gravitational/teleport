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
