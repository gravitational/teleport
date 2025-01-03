package protocol

import (
	"encoding/binary"
	"regexp"
	"slices"

	"github.com/gravitational/trace"
)

var (
	// serviceNameRegexp allows to extract SERVICE_NAME from Oracle ConnectionString.
	// Ref: https://github.com/sijms/go-ora/blob/master/network/connect_option.go#L131
	serviceNameRegexp = regexp.MustCompile(`(?i)\(\s*SERVICE_NAME\s*=\s*([\w,\.,\-]+)\s*\)`)
)

type ConnectPacket struct {
	base basePacket

	// connStringLength is length of connection string.
	connStringLength uint16
	// connStringOffset is an offset to the start of connection string in connStringData.
	connStringOffset uint16
	// connStringData contains connection string data, as well as a prefix and suffix we don't care about.
	// It is composed of payload of CONNECT packet and may be extended with additional data
	// from a follow-up DATA packet, which happens if the connection string is too long to fit into the initial CONNECT packet.
	//
	// To see if we expect a follow-up DATA packet, we check the declared connection string length
	// in contrast with what is available: see needMoreData() method.
	connStringData []byte

	DataPacket *DataPacket
}

func (cp *ConnectPacket) Size() uint32 {
	return cp.base.Size()
}

func (cp *ConnectPacket) Type() Type {
	return cp.base.Type()
}

func (cp *ConnectPacket) Payload() []byte {
	return cp.base.Payload()
}

func (cp *ConnectPacket) Header() PacketHeader {
	return cp.base.Header()
}

// DebugData returns extra data that may be worth including in the debug log.
func (cp *ConnectPacket) DebugData() map[string]any {
	out := map[string]any{
		"ConnStringLength": cp.connStringLength,
		"ConnStringOffset": cp.connStringOffset,
		"DataPacket":       "(missing)",
	}

	if cp.DataPacket != nil {
		out["DataPacket"] = cp.DataPacket.DebugData()
	}

	protoVersion, _ := cp.GetProtocolVersion()
	out["ProtocolVersion"] = protoVersion

	compatProtocolVersion, _ := cp.GetCompatProtocolVersion()
	out["CompatProtocolVersion"] = compatProtocolVersion

	return out
}

// needMoreData returns true if additional DATA packets are required to complete the connection string.
func (cp *ConnectPacket) needMoreData() bool {
	return int(cp.connStringLength) > len(cp.connStringData)-int(cp.connStringOffset)
}

// addMoreData appends additional data to the packet's connection string data.
func (cp *ConnectPacket) addMoreData(buffer []byte) {
	cp.connStringData = append(cp.connStringData, buffer...)
}

// MaybeReadMoreData optionally reads additional DATA packets from the connection and extends the ConnectPacket with it.
// This only happens if this data is actually needed.
//
// Parameter declared as inline interface type to avoid cyclic dependency.
func (cp *ConnectPacket) MaybeReadMoreData(conn interface{ ReadPacket() (Packet, error) }) error {
	if !cp.needMoreData() {
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

	cp.DataPacket = data

	// header + 2 bytes of junk data
	const offset = PacketHeaderSize + 2
	payload := data.Payload()

	if len(payload) < offset {
		return trace.BadParameter("expected at least %d bytes, got %d", offset, len(payload))
	}
	cp.addMoreData(payload[offset:])

	// should not happen!
	if cp.needMoreData() {
		return trace.BadParameter("ConnectPacket already extended with additional data, still not enough. (base:%v, data:%v)", cp.base, cp.DataPacket)
	}

	return nil
}

// GetConnectionString retrieves the Connection String if enough data is available.
func (cp *ConnectPacket) GetConnectionString() (string, error) {
	if cp.needMoreData() {
		return "", trace.BadParameter("not enough data for Connection String")
	}

	// Extract the Connection String slice from the payload.
	connStringSlice := cp.connStringData[cp.connStringOffset : cp.connStringOffset+cp.connStringLength]

	// Return the Connection String as a string.
	return string(connStringSlice), nil
}

func (cp *ConnectPacket) GetServiceName() (string, error) {
	connString, err := cp.GetConnectionString()
	if err != nil {
		return "", trace.Wrap(err)
	}

	match := serviceNameRegexp.FindStringSubmatch(connString)
	if len(match) != 2 {
		return "", trace.BadParameter("Failed to parse Connect Packet connection string: %q", connString)
	}

	return match[1], nil
}

func (cp *ConnectPacket) GetProtocolVersion() (uint16, error) {
	if len(cp.Payload()) < 4 {
		return 0, trace.BadParameter("not enough storage for protocol versions")
	}

	return binary.BigEndian.Uint16(cp.Payload()[PacketHeaderSize:]), nil
}

func (cp *ConnectPacket) GetCompatProtocolVersion() (uint16, error) {
	if len(cp.Payload()) < 4 {
		return 0, trace.BadParameter("not enough storage for protocol versions")
	}

	return binary.BigEndian.Uint16(cp.Payload()[PacketHeaderSize+2:]), nil
}

func (cp *ConnectPacket) SetProtocolVersion(protocolVersion uint16) error {
	if len(cp.Payload()) < 4 {
		return trace.BadParameter("not enough storage for protocol versions")
	}
	binary.BigEndian.PutUint16(cp.Payload()[PacketHeaderSize:], protocolVersion)
	return nil
}

func (cp *ConnectPacket) SetCompatProtocolVersion(compatProtocolVersion uint16) error {
	if len(cp.Payload()) < 4 {
		return trace.BadParameter("not enough storage for protocol versions")
	}
	binary.BigEndian.PutUint16(cp.Payload()[PacketHeaderSize+2:], compatProtocolVersion)
	return nil
}

func parseConnectPacket(bp *basePacket) (*ConnectPacket, error) {
	const connStrOffset = 24 // Offset where the Connection String length is stored.

	// Check if the payload is large enough to contain the necessary data.
	if len(bp.payload) < PacketHeaderSize+connStrOffset {
		return nil, trace.BadParameter("payload too small")
	}

	// Retrieve the Connection String length and offset from the payload.
	length := binary.BigEndian.Uint16(bp.payload[connStrOffset : connStrOffset+2])   // Length field size is 2 bytes
	offset := binary.BigEndian.Uint16(bp.payload[connStrOffset+2 : connStrOffset+4]) // Offset field size is 2 bytes

	return &ConnectPacket{
		base:             *bp,
		connStringLength: length,
		connStringOffset: offset,
		connStringData:   slices.Clone(bp.payload),
	}, nil
}
