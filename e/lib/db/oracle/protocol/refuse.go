package protocol

import (
	"bytes"
	"io"

	"github.com/gravitational/trace"
)

// RefusePacket struct { defines TNS client refuse oracle packet.
type RefusePacket struct {
	basePacket

	Message string
}

func parseRefusePacket(bp *basePacket) (*RefusePacket, error) {
	r := bytes.NewReader(bp.payload)
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
		basePacket: *bp,
		Message:    message,
	}
	return rp, nil
}
