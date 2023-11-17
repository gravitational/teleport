package protocol

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"net"

	"github.com/gravitational/trace"
)

// oracleConn is a helper connection structured used to read write oracle package.
type oracleConn struct {
	net.Conn
	protocolVersion   uint16
	sessionID         string
	isServerConn      bool
	connParamReceived bool
}

type writer interface {
	write(conn *oracleConn) error
}

func (c *oracleConn) writePacket(p writer) error {
	return trace.Wrap(p.write(c))
}

func (c *oracleConn) readPacket() (Packet, error) {
	r := bufio.NewReader(c)
	header, err := c.readHeader(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	payload := bytes.NewBuffer(make([]byte, 0, defaultReaderCapacity))

	if header.PacketSize < PacketHeaderSize {
		return nil, trace.BadParameter("invalid packet size")
	}

	remLen := int64(header.PacketSize - PacketHeaderSize)
	if _, err := io.CopyN(payload, r, remLen); err != nil {
		return nil, trace.Wrap(err)
	}

	basicPacket := &packet{
		Header: header,
		buff:   append(header.headerBytes, payload.Bytes()...),
		reader: r,
	}

	pck, err := parsePacket(basicPacket, c)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return pck, nil
}

func (c *oracleConn) readMoreData() ([]byte, error) {
	pck, err := c.readPacket()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Only DATA package is used to send more data and in case of other package type
	// return an error.
	if packetType := pck.Type(); packetType != DATA {
		return nil, trace.BadParameter("unexpected packet type %T", packetType)
	}
	return pck.Payload()[PacketHeaderSize:], nil
}

func (c *oracleConn) readHeader(r io.Reader) (*Header, error) {
	buff := make([]byte, PacketHeaderSize)
	_, err := r.Read(buff)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	size, err := getPacketSize(buff, c.protocolVersion)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Header{
		PacketSize:  size,
		PacketType:  Type(buff[4]),
		headerBytes: buff,
	}, nil
}

// getPacketSize allows to read packet size from the header depending on
// undervaluing protocol version. If protocol version was not negotiation - Accept package was yet received
// a server anc client is using 16 bytes package size.
func getPacketSize(header []byte, protocolVersion uint16) (uint32, error) {
	if protocolVersion >= TNSVersionMinLargeSdu {
		return binary.BigEndian.Uint32(header), nil
	}
	return uint32(binary.BigEndian.Uint16(header)), nil
}
