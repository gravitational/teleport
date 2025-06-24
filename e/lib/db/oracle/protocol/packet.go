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
	Type() PacketType
	// Payload returns raw Oracle packet data.
	Payload() []byte
	// Header returns underlying packet header.
	Header() PacketHeader
}

// PacketReader provides ReadPacket method.
type PacketReader interface {
	ReadPacket() (Packet, error)
}

// DebugPacket is implemented by Packets to provide debug information.
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

func (b *basePacket) Type() PacketType {
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
		return parseAcceptPacket(bp)
	case DATA:
		return parseDataPacket(bp)
	case REDIRECT:
		return parseRedirectPacket(bp)
	case CONNECT:
		return parseConnectPacket(bp)
	case REFUSE:
		return parseRefusePacket(bp)
	case MARKER:
		return parseMarkerPacket(bp)
	case RESEND:
		return &ResendPacket{basePacket: *bp}, nil
	default:
		return &UnknownPacket{basePacket: *bp}, nil
	}
}

// ReadPacketResult is a result of ReadPacket operation. May be partial.
type ReadPacketResult struct {
	// PartialHeader contains a header, if one was parsed successfully.
	// 'Partial' indicates it will be present even if the overall parse has failed.
	PartialHeader *PacketHeader
	// PartialBasePacket contains a base packet, which is an unexported underlying type for all packets.
	// 'Partial' indicates it will be present even if the overall parse has failed.
	PartialBasePacket Packet
	// SuccessPacket contains a final result of a successful parse.
	SuccessPacket Packet
}

// ReadPacket reads a packet from reader, using given protocol version (which dictates the format of a header).
// The result will always be present, even if there is a parsing error. The result is progressively filled.
// On complete success, the SuccessPacket field is set.
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
	return result, nil
}

type fixedPacketReader struct {
	packet Packet
}

func (r *fixedPacketReader) ReadPacket() (Packet, error) {
	return r.packet, nil
}
