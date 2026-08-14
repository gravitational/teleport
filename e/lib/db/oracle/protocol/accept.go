package protocol

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/gravitational/trace"
)

// AcceptPacket defines TNS Oracle package
// that is sent by the Oracle Server as a successful
// response on Connect packet.
type AcceptPacket struct {
	basePacket

	// ProtocolVersion contains Oracle Server protocol version
	ProtocolVersion uint16
	// ProtocolOptions contains Oracle Server protocol options.
	ProtocolOptions uint16
}

func parseAcceptPacket(bp *basePacket) (Packet, error) {
	r := bytes.NewReader(bp.payload)
	if _, err := r.Seek(PacketHeaderSize, io.SeekCurrent); err != nil {
		return nil, trace.Wrap(err)
	}
	acceptPaket := &AcceptPacket{
		basePacket: *bp,
	}
	if err := binary.Read(r, binary.BigEndian, &acceptPaket.ProtocolVersion); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := binary.Read(r, binary.BigEndian, &acceptPaket.ProtocolOptions); err != nil {
		return nil, trace.Wrap(err)
	}
	return acceptPaket, nil
}
