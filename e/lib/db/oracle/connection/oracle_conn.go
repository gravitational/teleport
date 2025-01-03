package connection

import (
	"crypto/tls"
	"net"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/e/lib/db/oracle/protocol"
)

// OracleConn is a helper connection structured used to read write oracle package.
type OracleConn struct {
	// conn is the underlying connection that is read and written.
	// It may be a plain TCP connection or a TLS connection.
	conn net.Conn

	// protocolVersion is the negotiated protocol version.
	protocolVersion uint16
}

type ConnOption func(conn *OracleConn) error

func (c *OracleConn) Close() error {
	return trace.Wrap(c.conn.Close())
}

// NewConn creates a new OracleConn using pre-opened network connection conn.
func NewConn(conn net.Conn, options ...ConnOption) (*OracleConn, error) {
	oracleConn := &OracleConn{
		conn: conn,
	}
	for _, opt := range options {
		err := opt(oracleConn)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return oracleConn, nil
}

func (c *OracleConn) WritePacket(p protocol.Packet) error {
	_, err := c.conn.Write(p.Payload())
	return trace.Wrap(err)
}

func (c *OracleConn) ReadPacket() (protocol.Packet, error) {
	result, err := protocol.ReadPacket(c.protocolVersion, c.conn)

	return result.SuccessPacket, trace.Wrap(err)
}

func (c *OracleConn) LargeSDU() bool {
	return c.protocolVersion >= protocol.TNSVersionMinLargeSdu
}

func (c *OracleConn) SetProtocolVersion(version uint16) {
	c.protocolVersion = version
}

// WithTLS modifies the OracleConn by performing TLS over existing connection and replacing the connection with resulting TLS connection.
func WithTLS(config *tls.Config) ConnOption {
	return func(conn *OracleConn) error {
		tlsConn := tls.Client(conn.conn, config)
		if err := tlsConn.Handshake(); err != nil {
			return trace.Wrap(err)
		}
		conn.conn = tlsConn
		return nil
	}
}
