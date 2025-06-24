package protocol

import (
	"bytes"
	"encoding/binary"
	"math"
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

func (cp *ConnectPacket) Type() PacketType {
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

	serviceOptions, _ := cp.GetServiceOptions()
	out["ServiceOptions"] = serviceOptions

	connString, _ := cp.GetConnectionString()
	out["ConnectionString"] = connString

	serviceName, _ := cp.GetServiceName()
	out["ServiceName"] = serviceName

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
func (cp *ConnectPacket) MaybeReadMoreData(conn PacketReader) error {
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

	payload, err := data.DataPayload()
	if err != nil {
		return trace.Wrap(err)
	}
	cp.addMoreData(payload)

	// should not happen unless the client is doing something very unusual.
	if cp.needMoreData() {
		return trace.BadParameter("packet already extended with additional data, still not enough. (base:%v, data:%v)", cp.base, cp.DataPacket)
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

// GetServiceName returns the service name extracted from the connection string.
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

// GetProtocolVersion returns the requested protocol version.
func (cp *ConnectPacket) GetProtocolVersion() (uint16, error) {
	if len(cp.Payload()) < 4 {
		return 0, trace.BadParameter("not enough storage for protocol versions")
	}

	return binary.BigEndian.Uint16(cp.Payload()[PacketHeaderSize:]), nil
}

// SetProtocolVersion sets the requested protocol version.
func (cp *ConnectPacket) SetProtocolVersion(protocolVersion uint16) error {
	if len(cp.Payload()) < 4 {
		return trace.BadParameter("not enough storage for protocol versions")
	}
	binary.BigEndian.PutUint16(cp.Payload()[PacketHeaderSize:], protocolVersion)
	return nil
}

// ServiceOptions represent service options.
// Example, from Wireshark dissector:
//
// 0x0c41, Header Checksum, Full Duplex
//
//	..0. .... .... .... = Broken Connect Notify: False
//	...0 .... .... .... = Packet Checksum: False
//	.... 1... .... .... = Header Checksum: True
//	.... .1.. .... .... = Full Duplex: True
//	.... ..0. .... .... = Half Duplex: False
//	.... .... ...0 .... = Direct IO to Transport: False
//	.... .... .... 0... = Attention Processing: False
//	.... .... .... .0.. = Can Receive Attention: False
//	.... .... .... ..0. = Can Send Attention: False
type ServiceOptions uint16

const (
	ServiceOptionCanSendAttention    ServiceOptions = 0x1 << 1
	ServiceOptionCanReceiveAttention ServiceOptions = 0x1 << 2
	ServiceOptionAttentionProcessing ServiceOptions = 0x1 << 3
	ServiceOptionDirectIOToTransport ServiceOptions = 0x1 << 4
	ServiceOptionFullDuplex          ServiceOptions = 0x1 << 10
	ServiceOptionHeaderChecksum      ServiceOptions = 0x1 << 11
)

func (so ServiceOptions) HasFlag(flag ServiceOptions) bool {
	return so&flag != 0
}

func (so ServiceOptions) WithFlagSet(flag ServiceOptions) ServiceOptions {
	return so | flag
}

func (so ServiceOptions) WithFlagUnset(flag ServiceOptions) ServiceOptions {
	return so &^ flag
}

// GetServiceOptions returns the requested service options.
func (cp *ConnectPacket) GetServiceOptions() (ServiceOptions, error) {
	if len(cp.Payload()) < PacketHeaderSize+6 {
		return 0, trace.BadParameter("payload too short for service options: %d", len(cp.Payload()))
	}

	return ServiceOptions(binary.BigEndian.Uint16(cp.Payload()[PacketHeaderSize+4:])), nil
}

// SetServiceOptions sets the requested service options.
func (cp *ConnectPacket) SetServiceOptions(options ServiceOptions) error {
	if len(cp.Payload()) < PacketHeaderSize+6 {
		return trace.BadParameter("payload too short for service options: %d", len(cp.Payload()))
	}
	binary.BigEndian.PutUint16(cp.Payload()[PacketHeaderSize+4:], uint16(options))
	return nil
}

// WithConnectionString returns a fresh copy of ConnectPacket but with connection string substituted for the provided value.
func (cp *ConnectPacket) WithConnectionString(connStr string) (*ConnectPacket, error) {
	if cp.needMoreData() {
		return nil, trace.BadParameter("connect packet is incomplete, updating connection string is not supported")
	}

	connStrData := []byte(connStr)

	if len(connStrData) > math.MaxUint16 {
		return nil, trace.BadParameter("connection string is too long: %d bytes", len(connStrData))
	}

	// Modifying the connect packet in place would be fairly complex:
	// - there are multiple fields that come into play,
	// - there is either one or two packets,
	// - there are semi-independent raw payload bytes to update as well.
	//
	// To makes things more manageable we will take a detour through raw byte buffer.
	// The key parts of the function are operating on a serialized packet form.
	//
	// Once we are done with the modifications we will parse the packet back, yielding a fresh but modified copy.
	//
	// For the result we will unconditionally produce two packets:
	// - main connect packet with protocol options etc.
	// - auxiliary data packet with the actual connection string.
	//
	// This is allowed by the protocol and simplifies the function by having just a single possible (valid) outcome.

	// get the payload for the modification
	payload := bytes.Clone(cp.Payload())

	// extend the payload with data packet contents
	if cp.DataPacket != nil {
		payload = append(payload, bytes.Clone(cp.DataPacket.Payload())...)
	}

	// At this point, payload contains:
	// - 24 bytes: packet header with a bunch of options: protocol version, SDU size, ...
	// - 2 bytes: connection string length
	// - 2 bytes: connection string offset
	// - X variable bytes: some extra data
	// - connection string length bytes: actual connection string, starting at specified offset.
	// - possibly extra trailing bytes (not actually observed so far).
	//
	// This data might have been split into two packets, but we no longer care about this detail.

	// After an update, we will have two parts:
	// 1. Connect packet:
	//    - 24 bytes of header. The header contains packet length (first two bytes), *updated* with current value.
	//    - 2 bytes of *updated* connection string length
	//    - 2 bytes of unchanged connection string offset
	//    - X bytes of extra data (variable, client-dependent)
	//
	// 2. Data packet *payload* (no header; DataPacketFromPayload will add that):
	//    - *updated* connection string bytes
	//    - trailing bytes

	// ensure sufficient payload length to ensure no panics.
	maxIndex := int(cp.connStringOffset + cp.connStringLength)
	if len(payload) < maxIndex {
		return nil, trace.BadParameter("incorrect packet: payload too short (%d)", len(payload))
	}

	// construct connect packet.
	// contents: everything up until the start of connection string data.
	connectPacketBytes := payload[:cp.connStringOffset]
	if len(connectPacketBytes) < connStrOffsetConnectPacket+2 {
		return nil, trace.BadParameter("incorrect packet: connect packet too short")
	}
	// update packet length in header (first two bytes)
	binary.BigEndian.PutUint16(connectPacketBytes[0:], uint16(len(connectPacketBytes)))
	// update connection string length
	binary.BigEndian.PutUint16(connectPacketBytes[connStrOffsetConnectPacket:], uint16(len(connStrData)))

	// construct data packet *payload* with connection string and optional suffix.
	dataPacketPayload := connStrData
	suffix := payload[cp.connStringOffset+cp.connStringLength:]
	dataPacketPayload = append(dataPacketPayload, suffix...)

	// parse the updated payload back.
	result, err := ReadPacket(0, bytes.NewReader(connectPacketBytes))
	if err != nil || result.SuccessPacket == nil {
		return nil, trace.BadParameter("failed to clone packet: %v", err)
	}

	clonedPacket, ok := result.SuccessPacket.(*ConnectPacket)
	if !ok {
		return nil, trace.BadParameter("expected ConnectPacket, got %T", result.SuccessPacket)
	}

	// append data packet with actual connection string.
	dp, err := DataPacketFromPayload(false, dataPacketPayload)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = clonedPacket.MaybeReadMoreData(&fixedPacketReader{packet: dp})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// sanity check #1
	if clonedPacket.needMoreData() {
		return nil, trace.BadParameter("not enough data for connection string: %q (this is a bug)", connStr)
	}

	// sanity check #2
	connStrParsed, err := clonedPacket.GetConnectionString()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if connStrParsed != connStr {
		return nil, trace.BadParameter("connection string mangled after update: %q != %q (this is a bug)", connStrParsed, connStr)
	}

	return clonedPacket, nil
}

// connStrOffsetConnectPacket is the offset to connection string length in the connect packet.
const connStrOffsetConnectPacket = 24

func parseConnectPacket(bp *basePacket) (*ConnectPacket, error) {
	// Check if the payload is large enough to contain the necessary data.
	if len(bp.payload) < PacketHeaderSize+connStrOffsetConnectPacket {
		return nil, trace.BadParameter("payload too small")
	}

	// Retrieve the Connection String length and offset from the payload.
	length := binary.BigEndian.Uint16(bp.payload[connStrOffsetConnectPacket : connStrOffsetConnectPacket+2])   // Length field size is 2 bytes
	offset := binary.BigEndian.Uint16(bp.payload[connStrOffsetConnectPacket+2 : connStrOffsetConnectPacket+4]) // Offset field size is 2 bytes

	// sanity check offset value
	if offset < (connStrOffsetConnectPacket + 2 + 2) {
		return nil, trace.BadParameter("connect packet invalid: invalid connection string offset (%d)", offset)
	}

	return &ConnectPacket{
		base:             *bp,
		connStringLength: length,
		connStringOffset: offset,
		connStringData:   slices.Clone(bp.payload),
	}, nil
}
