package protocol

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/gravitational/trace"
)

// Packet defines a basic packet interfaces.
type Packet interface {
	// Size returns the Oracle packet size.
	Size() uint32
	// Type returns the Oracle packet type.
	Type() Type
	// Payload returns raw Oracle packet data.
	Payload() []byte
	// writer is used internally to send a packet
	writer
}

type packet struct {
	*Header
	buff   []byte
	reader io.Reader
}

func (b *packet) write(conn *oracleConn) error {
	_, err := conn.Write(b.buff)
	return trace.Wrap(err)
}

// Size return the total size of the packet.
func (b *packet) Size() uint32 {
	return b.PacketSize
}

// Type returns packet type
func (b *packet) Type() Type {
	return b.PacketType
}

// Payload returns all package data payload.
func (b *packet) Payload() []byte {
	return b.buff
}

// Header defines the TNS Oracle packet header.
type Header struct {
	// PacketSize is the size of a packet.
	PacketSize uint32
	// PacketType is a TNS Oracle packet type.
	PacketType Type
	// headerBytes contains raw bytes of the header.
	headerBytes []byte
}

// parsePacket read the package payload and returns Oracle Packet.
func parsePacket(bp *packet, c *oracleConn) (Packet, error) {
	switch bp.PacketType {
	case ACCEPT:
		ac, err := parseAcceptPacket(bp)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return ac, nil
	case RESEND:
		return &ResendPacket{
			packet: bp,
		}, nil
	case DATA:
		if !c.isServerConn || c.connParamReceived {
			return bp, nil
		}
		dp, err := parseDataPacket(bp, c)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return dp, nil
	case REDIRECT:
		return &RedirectPacket{
			packet: bp,
		}, nil
	case CONNECT:
		cp, err := parseConnectPacket(bp, c)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return cp, nil
	case REFUSE:
		ac, err := parseRefusePacket(bp)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return ac, nil
	default:
		return bp, nil
	}
}

func readString(r io.Reader) (string, error) {
	buff, err := readByteArray(r)
	return string(buff), trace.Wrap(err)
}

func readByteArray(r io.Reader) ([]byte, error) {
	var len uint8
	if err := binary.Read(r, binary.BigEndian, &len); err != nil {
		return nil, trace.Wrap(err)
	}
	buff := bytes.NewBuffer(make([]byte, 0, len))
	if _, err := io.CopyN(buff, r, int64(len)); err != nil {
		return nil, trace.Wrap(err)
	}
	return buff.Bytes(), nil

}

func readInt64(r io.Reader) (int64, error) {
	var length uint8
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return 0, trace.Wrap(err)
	}
	if length > 8 {
		return 0, trace.BadParameter("invalid length value: %d", length)
	}

	buff := bytes.NewBuffer(make([]byte, 0, length))
	if _, err := io.CopyN(buff, r, int64(length)); err != nil {
		return 0, trace.Wrap(err)
	}
	temp := make([]byte, 8)
	copy(temp[8-length:], buff.Bytes())
	return int64(binary.BigEndian.Uint64(temp)), nil
}

func readDataLengthContent(r io.Reader) ([]byte, error) {
	n, err := readInt64(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if n > 0 {
		out, err := readByteArray(r)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return out[:n], nil
	}
	return nil, nil
}
