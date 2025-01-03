package protocol

import (
	"bytes"
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
	// Header returns underlying packet header.
	Header() PacketHeader
}

type DebugPacket interface {
	DebugData() map[string]any
}

type basePacket struct {
	header  PacketHeader
	payload []byte
}

var _ Packet = (*basePacket)(nil)

func (b *basePacket) Size() uint32 {
	return b.header.PacketSize
}

func (b *basePacket) Type() Type {
	return b.header.PacketType
}

func (b *basePacket) Payload() []byte {
	return b.payload
}

func (b *basePacket) Header() PacketHeader {
	return b.header
}

// UnknownPacket represents a packet not explicitly supported by the library.
type UnknownPacket struct {
	basePacket
}

// parsePacket read the package payload and returns Oracle Packet.
func parsePacket(bp *basePacket) (Packet, error) {
	switch bp.Type() {
	case ACCEPT:
		ac, err := parseAcceptPacket(bp)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return ac, nil
	case RESEND:
		return &ResendPacket{basePacket: *bp}, nil
	case DATA:
		dp, err := parseDataPacket(bp)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return dp, nil
	case REDIRECT:
		return &RedirectPacket{basePacket: *bp}, nil
	case CONNECT:
		cp, err := parseConnectPacket(bp)
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
	case MARKER:
		marker, err := parseMarkerPacket(bp)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return marker, nil
	default:
		return &UnknownPacket{basePacket: *bp}, nil
	}
}

// ReadPacketResult is a result of ReadPacket operation. May be partial.
type ReadPacketResult struct {
	PartialHeader     *PacketHeader
	PartialBasePacket Packet // will always be basePacket.
	SuccessPacket     Packet
}

func ReadPacket(protocolVersion uint16, reader io.Reader) (ReadPacketResult, error) {
	result := ReadPacketResult{}

	header, err := parseHeader(protocolVersion, reader)
	if err != nil {
		return result, trace.Wrap(err)
	}

	result.PartialHeader = header

	payload := bytes.NewBuffer(make([]byte, 0, 32*1024))

	if header.PacketSize < PacketHeaderSize {
		return result, trace.BadParameter("invalid packet size %v, lower than minimum size %v", header.PacketSize, PacketHeaderSize)
	}

	remLen := int64(header.PacketSize - PacketHeaderSize)
	if _, err := io.CopyN(payload, reader, remLen); err != nil {
		return result, trace.Wrap(err)
	}

	bp := &basePacket{
		header:  *header,
		payload: append(header.HeaderBytes, payload.Bytes()...),
	}

	result.PartialBasePacket = bp

	pck, err := parsePacket(bp)
	if err != nil {
		return result, trace.Wrap(err)
	}

	result.SuccessPacket = pck

	return result, trace.Wrap(err)
}
