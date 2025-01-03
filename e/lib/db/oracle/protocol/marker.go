package protocol

import (
	"fmt"

	"github.com/gravitational/trace"
)

type MarkerPacket struct {
	basePacket

	MarkerType MarkerType
}

type MarkerType uint8

const (
	MarkerTypeUnknown MarkerType = iota
	MarkerTypeBreak
	MarkerTypeReset
	MarkerTypeInterrupt
)

func (mt MarkerType) String() string {
	var name string
	switch mt {
	case MarkerTypeBreak:
		name = "BREAK"
	case MarkerTypeReset:
		name = "RESET"
	case MarkerTypeInterrupt:
		name = "INTERRUPT"
	default:
		name = "UNKNOWN"
	}
	return fmt.Sprintf("%v(%v)", name, int(mt))
}

func parseMarkerPacket(bp *basePacket) (*MarkerPacket, error) {
	if bp.Type() != MARKER {
		return nil, trace.BadParameter("unexpected marker packet type: %v", bp.Type())
	}

	// 3 bytes of data here:
	// (ignored)
	// (ignored)
	// marker value
	const markerPacketLength = PacketHeaderSize + 3

	if len(bp.Payload()) != markerPacketLength {
		return nil, trace.BadParameter("malformed marker packet, unexpected length (%v bytes)", len(bp.Payload()))
	}

	pkt := &MarkerPacket{
		basePacket: *bp,
		MarkerType: MarkerType(bp.Payload()[markerPacketLength-1]),
	}

	return pkt, nil
}

func (pkt *MarkerPacket) DebugData() map[string]any {
	return map[string]any{
		"marker_type": pkt.MarkerType.String(),
	}
}
