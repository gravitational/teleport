package protocol

import (
	"bytes"
	"io"

	"github.com/gravitational/trace"
)

// RefusePacket struct { defines TNS client refuse oracle packet.
type RefusePacket struct {
	*packet
	Message string
}

func parseRefusePacket(bp *packet) (Packet, error) {
	r := bytes.NewReader(bp.buff)
	if _, err := r.Seek(PacketHeaderSize, io.SeekCurrent); err != nil {
		return nil, trace.Wrap(err)
	}
	if _, err := r.Seek(3, io.SeekCurrent); err != nil {
		return nil, trace.Wrap(err)
	}
	message, err := readString(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rp := &RefusePacket{
		packet:  bp,
		Message: message,
	}
	return rp, nil
}
