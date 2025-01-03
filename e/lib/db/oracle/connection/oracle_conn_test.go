package connection

import (
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/lib/db/oracle/protocol"
	"github.com/gravitational/teleport/e/lib/db/oracle/testdata"
	"github.com/gravitational/teleport/lib/utils"
)

// TestClientServerConnReg simulates the initial phases of connection negotiation. Database client connects to server over TLS,
// the server requests another TLS upgrade with a resend packet, client obliges. The next attempt succeeds and sever responds with 'ACCEPT' packet.
func TestClientServerConnReg(t *testing.T) {
	keyPEM, certPEM, err := utils.GenerateRSASelfSignedSigningCert(pkix.Name{
		Organization: []string{"Teleport Test"},
		CommonName:   "Teleport",
	}, []string{"localhost", "127.0.0.1"}, 10*365*24*time.Hour)
	require.NoError(t, err)

	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)

	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM(certPEM))

	tlsConfig := &tls.Config{Certificates: []tls.Certificate{certificate}}
	l, errTLS := net.Listen("tcp", "localhost:0")
	require.NoError(t, errTLS)
	defer l.Close()

	// run fake database server
	go func() {
		// accept connection from client
		conn, err := l.Accept()
		require.NoError(t, err)
		defer conn.Close()

		tlsConn := tls.Server(conn, tlsConfig)
		require.NoError(t, err)

		// oracleClient represents the client on the other side of the connection.
		oracleClient, err := NewConn(tlsConn)
		require.NoError(t, err)
		defer oracleClient.Close()

		// expect a connect request; verify contents, and ask reply with RESEND (which triggers another TLS upgrade)
		connectRequest, err := oracleClient.ReadPacket()
		require.IsType(t, &protocol.ConnectPacket{}, connectRequest)
		require.NotNil(t, connectRequest)
		require.NoError(t, err)
		err = oracleClient.WritePacket(mustParseDumpToPacket(t, testdata.ResendPacketDump))
		require.NoError(t, err)

		cStr, err := connectRequest.(*protocol.ConnectPacket).GetConnectionString()
		require.NoError(t, err)
		require.Equal(t, `(DESCRIPTION=(ADDRESS=(PROTOCOL=tcps)(HOST=127.0.0.1)(PORT=54557))(CONNECT_DATA=(CID=(PROGRAM=SQLcl)(HOST=__jdbc__)(USER=marek))(SERVICE_NAME=XE)(CONNECTION_ID=MAVsTlvrTyqsibsnisguzw==)))`, cStr)

		// After sending Resend packet the client connection should be upgrade on more time to TLS connection.
		tlsConn = tls.Server(conn, tlsConfig)
		oracleClient, err = NewConn(tlsConn)
		require.NoError(t, err)
		defer oracleClient.Close()

		// we are happy now, proceed with the accept packet
		err = oracleClient.WritePacket(mustParseDumpToPacket(t, testdata.AcceptPacketDump))
		require.NoError(t, err)
	}()

	// database client flow
	tcpConn, err := net.Dial("tcp", l.Addr().String())
	require.NoError(t, err)

	serverConn, err := NewConn(tcpConn, WithTLS(&tls.Config{
		ServerName: "localhost",
		RootCAs:    pool,
	}))
	require.NoError(t, err)

	// say hello
	err = serverConn.WritePacket(mustParseDumpToPacket(t, testdata.ConnectPacketDump))
	require.NoError(t, err)

	// expect 'RESEND'
	response, err := serverConn.ReadPacket()
	require.NoError(t, err)
	require.Equal(t, mustParseDumpToPacket(t, testdata.ResendPacketDump), response)

	// retry TLS
	serverConn, err = NewConn(tcpConn, WithTLS(&tls.Config{
		ServerName: "localhost",
		RootCAs:    pool,
	}))
	require.NoError(t, err)

	// say hello once again.
	err = serverConn.WritePacket(mustParseDumpToPacket(t, testdata.ConnectPacketDump))
	require.NoError(t, err)

	// expect 'ACCEPT' this time.
	response, err = serverConn.ReadPacket()
	require.NoError(t, err)
	require.Equal(t, mustParseDumpToPacket(t, testdata.AcceptPacketDump), response)

	// verify expected protocol version
	require.Equal(t, uint16(318), response.(*protocol.AcceptPacket).ProtocolVersion)
}

func mustParseDumpToPacket(t *testing.T, dump string) protocol.Packet {
	packet, err := protocol.ParseDumpToPacket(false, dump)
	require.NoError(t, err)
	return packet
}
