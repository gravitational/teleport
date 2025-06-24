package protocol

import (
	"bytes"
	"encoding/binary"

	"github.com/gravitational/trace"
)

// RedirectPacket defines TNS oracle package
// that is used by Oracle Server to indicate a client
// to redirect connection.
type RedirectPacket struct {
	base basePacket

	redirectDataLength uint16
	redirectData       []byte
}

func (rp *RedirectPacket) Size() uint32 {
	return rp.base.Size()
}

func (rp *RedirectPacket) Type() PacketType {
	return rp.base.Type()
}

func (rp *RedirectPacket) Payload() []byte {
	return rp.base.Payload()
}

func (rp *RedirectPacket) Header() PacketHeader {
	return rp.base.Header()
}

// DebugData returns extra data that may be worth including in the debug log.
func (rp *RedirectPacket) DebugData() map[string]any {
	out := map[string]any{
		"redirectDataLength": rp.redirectDataLength,
		"redirectData":       rp.redirectData,
		"needMoreData":       rp.needMoreData(),
	}

	if !rp.needMoreData() {
		addr, connStr, err := rp.redirectDataSplit()
		if err != nil {
			out["redirectDataSplit_error"] = trace.DebugReport(err)
		} else {
			out["redirectAddr"] = addr
			out["redirectConnStr"] = connStr
		}
	}

	return out
}

func parseRedirectPacket(bp *basePacket) (*RedirectPacket, error) {
	if len(bp.payload) < PacketHeaderSize+2 {
		return nil, trace.BadParameter("payload too small")
	}

	redirectDataLength := binary.BigEndian.Uint16(bp.payload[PacketHeaderSize : PacketHeaderSize+2])
	redirectData := bp.payload[PacketHeaderSize+2:]

	return &RedirectPacket{
		base:               *bp,
		redirectDataLength: redirectDataLength,
		redirectData:       redirectData,
	}, nil
}

func (rp *RedirectPacket) MaybeReadMoreData(conn PacketReader) error {
	if !rp.needMoreData() {
		return nil
	}

	pkt, err := conn.ReadPacket()
	if err != nil {
		return trace.Wrap(err)
	}
	data, ok := pkt.(*DataPacket)
	if !ok {
		return trace.BadParameter("expected DataPacket, got %T", pkt)
	}

	payload, err := data.DataPayload()
	if err != nil {
		return trace.Wrap(err)
	}
	rp.addMoreData(payload)

	if rp.needMoreData() {
		return trace.BadParameter("packet already extended with additional data yet still not enough.")
	}

	return nil
}

func (rp *RedirectPacket) redirectDataSplit() (string, string, error) {
	if len(rp.redirectData) == 0 {
		return "", "", trace.BadParameter("no redirect data available")
	}

	parts := bytes.Split(rp.redirectData, []byte{0})
	if len(parts) != 2 {
		return "", "", trace.BadParameter("unexpected redirect data format, expected 2 parts, got %d (%v)", len(parts), rp.redirectData)
	}

	return string(parts[0]), string(parts[1]), nil
}

func (rp *RedirectPacket) RedirectAddress() (string, error) {
	addr, _, err := rp.redirectDataSplit()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return addr, nil
}

func (rp *RedirectPacket) RedirectConnectionString() (string, error) {
	_, connStr, err := rp.redirectDataSplit()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return connStr, nil
}

func (rp *RedirectPacket) needMoreData() bool {
	return len(rp.redirectData) < int(rp.redirectDataLength)
}

func (rp *RedirectPacket) addMoreData(payload []byte) {
	rp.redirectData = append(rp.redirectData, payload...)
}
