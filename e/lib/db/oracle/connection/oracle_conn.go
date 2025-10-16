package connection

import (
	"context"
	"crypto/tls"
	"net"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/e/lib/db/oracle/protocol"
)

// OracleConn represents a connection to Oracle client or server.
type OracleConn struct {
	// conn is the underlying connection that is read and written.
	// It may be a plain TCP connection or a TLS connection.
	conn net.Conn

	// onReadHeader will be called after each header is successfully read and parsed.
	onReadHeader func(header protocol.PacketHeader)
	// onReadPacket will be called after each packet is successfully read and parsed.
	onReadPacket func(protocol.Packet)
	// onWritePacket will be called on each packet write.
	onWritePacket func(protocol.Packet)

	// protocolVersion is the negotiated protocol version.
	protocolVersion uint16
}

// ConnOption is an option that can modify OracleConn.
type ConnOption func(ctx context.Context, onn *OracleConn) error

func (c *OracleConn) Close() error {
	return trace.Wrap(c.conn.Close())
}

// NewConn creates a new OracleConn using pre-opened network connection conn.
func NewConn(ctx context.Context, conn net.Conn, options ...ConnOption) (*OracleConn, error) {
	oracleConn := &OracleConn{
		conn: conn,
	}
	for _, opt := range options {
		err := opt(ctx, oracleConn)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return oracleConn, nil
}

// WritePacket writes a packet to the connection.
func (c *OracleConn) WritePacket(p protocol.Packet) error {
	if c.onWritePacket != nil {
		c.onWritePacket(p)
	}
	_, err := c.conn.Write(p.Payload())
	return trace.Wrap(err)
}

// ReadPacket reads a packet from the connection.
func (c *OracleConn) ReadPacket() (protocol.Packet, error) {
	result, err := protocol.ReadPacket(c.protocolVersion, c.conn)

	// log appropriate result, the most complete one available.
	if result.SuccessPacket != nil && c.onReadPacket != nil {
		c.onReadPacket(result.SuccessPacket)
	} else if result.PartialBasePacket != nil && c.onReadPacket != nil {
		c.onReadPacket(result.PartialBasePacket)
	} else if result.PartialHeader != nil && c.onReadHeader != nil {
		c.onReadHeader(*result.PartialHeader)
	}

	return result.SuccessPacket, trace.Wrap(err)
}

// LargeSDU returns true if the negotiated protocol version supports 'large SDU' header format.
func (c *OracleConn) LargeSDU() bool {
	return c.protocolVersion >= protocol.TNSVersionMinLargeSdu
}

// SetProtocolVersion sets the protocol version that have been negotiated.
func (c *OracleConn) SetProtocolVersion(version uint16) {
	c.protocolVersion = version
}

// WithOnReadHeader updates the callback to call on each successful header parse.
func WithOnReadHeader(onReadHeader func(header protocol.PacketHeader)) ConnOption {
	return func(ctx context.Context, conn *OracleConn) error {
		conn.onReadHeader = onReadHeader
		return nil
	}
}

// WithOnReadPacket calls the specified function on each successfully parsed read packet.
func WithOnReadPacket(onReadPacket func(protocol.Packet)) ConnOption {
	return func(ctx context.Context, conn *OracleConn) error {
		conn.onReadPacket = onReadPacket
		return nil
	}
}

// WithOnWritePacket makes the supplied function to be called on each packet write.
func WithOnWritePacket(onWritePacket func(protocol.Packet)) ConnOption {
	return func(ctx context.Context, conn *OracleConn) error {
		conn.onWritePacket = onWritePacket
		return nil
	}
}

// WithTLS modifies the OracleConn by performing TLS over existing connection and replacing the connection with resulting TLS connection.
func WithTLS(config *tls.Config) ConnOption {
	return func(ctx context.Context, conn *OracleConn) error {
		tlsConn := tls.Client(conn.conn, config)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return trace.Wrap(err, "tls handshake failed")
		}
		conn.conn = tlsConn
		return nil
	}
}
