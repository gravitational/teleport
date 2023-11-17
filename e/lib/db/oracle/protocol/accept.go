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
	*packet
	// ProtocolVersion contains Oracle Server protocol version
	ProtocolVersion uint16
	// ProtocolOptions contains Oracle Server protocol options.
	ProtocolOptions uint16
}

func parseAcceptPacket(basicPacket *packet) (Packet, error) {
	r := bytes.NewReader(basicPacket.buff)
	if _, err := r.Seek(PacketHeaderSize, io.SeekCurrent); err != nil {
		return nil, trace.Wrap(err)
	}
	acceptPaket := &AcceptPacket{
		packet: basicPacket,
	}
	if err := binary.Read(r, binary.BigEndian, &acceptPaket.ProtocolVersion); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := binary.Read(r, binary.BigEndian, &acceptPaket.ProtocolOptions); err != nil {
		return nil, trace.Wrap(err)
	}
	return acceptPaket, nil
}
