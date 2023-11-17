package protocol

import (
	"crypto/tls"
	"net"

	"github.com/gravitational/trace"
)

// Conn is a structured uses to represent Client/Server oracle connections.
type Conn struct {
	oracleConn
	tcpConn       net.Conn
	tlsConfig     *tls.Config
	connectPacket Packet
	connAccepted  bool
}

// NewServerConn allows to create updated connection to Oracle Server.
func NewServerConn(addr string, tlsConfig *tls.Config) (*Conn, error) {
	tcpConn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsConn := tls.Client(tcpConn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Conn{
		tcpConn: tcpConn,
		oracleConn: oracleConn{
			Conn:         tlsConn,
			isServerConn: true,
		},
		tlsConfig: tlsConfig,
	}, nil
}

// NewClientConn  allows to wrap an upstream client connection to Oracle Server.
func NewClientConn(conn net.Conn) *Conn {
	return &Conn{
		oracleConn: oracleConn{Conn: conn},
	}
}

// ConnAccepted checks if accepted package was handle by the Conn.
func (c *Conn) ConnAccepted() bool {
	return c.connAccepted
}

// ReadPacket reads the Oracle Packet from the Conn.
func (c *Conn) ReadPacket() (Packet, error) {
	for {
		p, err := c.oracleConn.readPacket()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		switch t := p.(type) {
		case *ResendPacket:
			// The server sends a Resend packet that indicate that a client should reconnect with TLS handshake
			// inside inner raw net connection and send one more time the Connection package.
			// We keep this logic only between db agent server and upstream Oracle server connection.
			// https://github.com/oracle/python-oracledb/blob/main/src/oracledb/impl/thin/protocol.pyx#L184
			c.Conn = tls.Client(c.tcpConn, c.tlsConfig)
			if c.connectPacket == nil {
				return nil, trace.BadParameter("failed phase one")
			}
			err = c.oracleConn.writePacket(c.connectPacket)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			continue
		case *AcceptPacket:
			// Save negotiated protocol version that is used to check how to read package header size.
			c.protocolVersion = t.ProtocolVersion
			c.connAccepted = true
		case *RedirectPacket:
			return nil, trace.NotImplemented("Redirection is not supported")
		}
		return p, trace.Wrap(err)
	}
}

// WritePacket writes the Oracle Packet to the Conn.
func (c *Conn) WritePacket(p Packet) error {
	if err := c.oracleConn.writePacket(p); err != nil {
		return trace.Wrap(err)
	}
	switch t := p.(type) {
	case *ConnectPacket:
		c.connectPacket = p
	case *AcceptPacket:
		// Save negotiated protocol version that is used to check how to read package header size.
		c.protocolVersion = t.ProtocolVersion
		c.connAccepted = true
	}
	return nil
}
