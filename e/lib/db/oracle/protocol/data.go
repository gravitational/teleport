package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/gravitational/trace"
)

// DataFlags represents various flags that can be set on DATA packet.
// See: https://github.com/oracle/python-oracledb/blob/d7003f2f848c877cd8632c8c391b4eb3bfaae068/src/oracledb/impl/thin/constants.pxi#L48-L53
type DataFlags uint16

// DataID is a Data packet type.
// See also: https://github.com/oracle/python-oracledb/blob/d7003f2f848c877cd8632c8c391b4eb3bfaae068/src/oracledb/impl/thin/constants.pxi#L73-L96
type DataID int

const (
	DataIDProtocol            DataID = 1
	DataIDDataTypes           DataID = 2
	DataIDFunction            DataID = 3
	DataIDError               DataID = 4
	DataIDRowHeader           DataID = 6
	DataIDRowData             DataID = 7
	DataIDParameter           DataID = 8
	DataIDStatus              DataID = 9
	DataIDDescribeInformation DataID = 10
	DataIDIoVector            DataID = 11
	DataIDLobData             DataID = 14
	DataIDWarning             DataID = 15
	DataIDDescribeInfo        DataID = 16
	DataIDPiggyback           DataID = 17
	DataIDFlushOutBinds       DataID = 19
	DataIDBitVector           DataID = 21
	DataIDServerSidePiggyback DataID = 23
	DataIDOneWayFn            DataID = 26
	DataIDImplicitResultset   DataID = 27
	DataIDRenegotiate         DataID = 28
	DataIDEndOfResponse       DataID = 29
	DataIDToken               DataID = 33
	DataIDFastAuth            DataID = 34
)

func (d DataID) String() string {
	if d < 0 {
		return "(not set)"
	}

	names := map[DataID]string{
		DataIDProtocol:            "Protocol",
		DataIDDataTypes:           "DataTypes",
		DataIDFunction:            "Function",
		DataIDError:               "Error",
		DataIDRowHeader:           "RowHeader",
		DataIDRowData:             "RowData",
		DataIDParameter:           "Parameter",
		DataIDStatus:              "Status",
		DataIDDescribeInformation: "DescribeInformation",
		DataIDIoVector:            "IoVector",
		DataIDLobData:             "LobData",
		DataIDWarning:             "Warning",
		DataIDDescribeInfo:        "DescribeInfo",
		DataIDPiggyback:           "Piggyback",
		DataIDFlushOutBinds:       "FlushOutBinds",
		DataIDBitVector:           "BitVector",
		DataIDServerSidePiggyback: "ServerSidePiggyback",
		DataIDOneWayFn:            "OneWayFn",
		DataIDImplicitResultset:   "ImplicitResultset",
		DataIDRenegotiate:         "Renegotiate",
		DataIDEndOfResponse:       "EndOfResponse",
		DataIDToken:               "Token",
		DataIDFastAuth:            "FastAuth",
	}

	name, ok := names[d]
	if !ok {
		name = "Unknown"
	}

	return fmt.Sprintf("%v(%v)", name, byte(d))
}

const (
	// AuthSessionIDKey is the server params key used to obtain a unique Oracle user session ID.
	AuthSessionIDKey = "AUTH_SESSION_ID"
)

// DataPacket defines TNS data oracle packet that is used a generic transport unit in oracle wire protocol.
// See: TNS_DATA_FLAGS_EOF.
type DataPacket struct {
	basePacket

	Flags  DataFlags
	DataID DataID
	Data   []byte
}

type CallID byte

const (
	CallIdGetSessionKey             CallID = 0x76
	CallIDGenericAuthenticationCall CallID = 0x73
	CallIDPing                      CallID = 0x93
	CallIDBundledExecutionCall      CallID = 0x5e
)

func (c CallID) String() string {
	var description string

	switch c {
	case CallIDGenericAuthenticationCall:
		description = "Generic Authentication Call"
	case CallIDPing:
		description = "Ping"
	case CallIDBundledExecutionCall:
		description = "Bundled Execution Call"
	case CallIdGetSessionKey:
		description = "Get Session Key"
	default:
		description = "Unknown"
	}

	return fmt.Sprintf("CallID(%v:%v)", byte(c), description)
}

// DebugData returns extra data that may be worth including in the debug log.
func (dp *DataPacket) DebugData() map[string]any {

	out := map[string]any{
		"DataID":       dp.DataID.String(),
		"Flags_binary": formatBinary(dp.Flags),
		"Type":         dp.Type().String(),
	}

	callId, err := dp.CallID()
	if err != nil {
		if dp.DataID == DataIDFunction {
			out["CallID_err"] = err.Error()
		}
	} else {
		out["CallID"] = callId.String()
	}

	params, err := dp.AuthParameters()
	if err != nil {
		if dp.DataID == DataIDParameter {
			out["AuthParameters_err"] = err.Error()
		}
	} else {
		out["AuthParameters"] = params
	}

	out["HasSecureNetworkServices"] = dp.HasSecureNetworkServices()

	return out
}

func (dp *DataPacket) CallID() (CallID, error) {
	if dp.DataID != DataIDFunction {
		return 0, trace.BadParameter("CallID not available for non-function calls (DataID = %v)", dp.DataID.String())
	}

	r := bytes.NewReader(dp.Data)

	callId, err := r.ReadByte()
	if err != nil {
		return 0, trace.Wrap(err)
	}

	return CallID(callId), nil
}

// ParamValue holds a string value with optional integer tag.
type ParamValue struct {
	// Value is the parameter value.
	Value string `json:"value"`
	// Tag is the optional "type" tag on value.
	Tag int64 `json:"tag"`
}

// ParamParseResult holds result of parsing the parameter map. The result may be partial.
type ParamParseResult struct {
	// Parameters is a map of results.
	Parameters map[string]ParamValue
	// IsPartial will be true if the parsing was incomplete.
	// This may happen if the param map is spread across multiple messages due to being too large to fit in a single one under configured SDU.
	IsPartial bool
}

// ToDictionary converts ParamParseResult to a plain string->string dictionary, discarding the tags.
func (r *ParamParseResult) ToDictionary() map[string]string {
	out := make(map[string]string)
	for k, v := range r.Parameters {
		out[k] = v.Value
	}
	return out
}

func (dp *DataPacket) HasAuthParameters() bool {
	if dp.DataID != DataIDParameter {
		return false
	}

	dataPrefix := string(dp.Data[:min(len(dp.Data), 64)])
	return strings.Contains(dataPrefix, "AUTH_VERSION_STRING")
}

// AuthParameters returns parsed params, if present. By necessity, it involves some heuristics, which may need to be updated as needs arise.
func (dp *DataPacket) AuthParameters() (*ParamParseResult, error) {
	if !dp.HasAuthParameters() {
		return nil, trace.BadParameter("Parameters not available for non-parameter packets (DataID = %v)", dp.DataID.String())
	}

	// check the offset of AUTH_VERSION_STRING to determine format.
	dataPrefix := string(dp.Data[:min(len(dp.Data), 64)])
	authVersionStringIndex := strings.Index(dataPrefix, "AUTH_VERSION_STRING")

	r := bytes.NewReader(dp.Data)

	var readInt func(reader *bytes.Reader) (int64, error)
	var paramCount int

	const sqlclAuthVersionStringOffset = 5
	const sqlplusAuthVersionStringOffset = 7

	switch authVersionStringIndex {
	case sqlclAuthVersionStringOffset: // SQLcl and friends.
		readInt = readVarInt64

		count, err := readVarInt64(r)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		paramCount = int(count)
	case sqlplusAuthVersionStringOffset: // SQL*Plus and other users of ojdbc11.jar
		readInt = readLittleEndianUint32

		count, err := r.ReadByte()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		paramCount = int(count)

		// skip zero byte.
		wantZero, err := r.ReadByte()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if wantZero != 0 {
			return nil, trace.BadParameter("Data is missing zero byte")
		}
	default:
		return nil, trace.BadParameter("Unexpected offset to AUTH_VERSION_STRING: %v", authVersionStringIndex)
	}

	parameters := map[string]ParamValue{}

	for range paramCount {
		key, val, tag, err := readKeyValueTag(readInt, r)
		if err != nil {
			// this can happen with low SDU, as the packet payload has been fragmented and the part that we want to read will arrive in the future.
			// we could try reassembling it, but this is difficult with our code structure; we try to pass the packets along in a stateless manner.
			//
			// return the parameters parsed so far, typically the keys we care about are present in the first packet to arrive.
			if errors.Is(err, io.ErrUnexpectedEOF) {
				return &ParamParseResult{Parameters: parameters, IsPartial: true}, nil
			}
			return nil, trace.Wrap(err)
		}
		parameters[key] = ParamValue{Value: val, Tag: tag}
	}

	return &ParamParseResult{Parameters: parameters, IsPartial: false}, nil
}

// HasSecureNetworkServices returns true if given payload concerns Secure Network Services negotiation.
func (dp *DataPacket) HasSecureNetworkServices() bool {
	dataPayload, err := dp.DataPayload()
	if err != nil {
		return false
	}

	// verify the marker 0xDEADBEEF
	return bytes.HasPrefix(dataPayload, []byte{0xDE, 0xAD, 0xBE, 0xEF})
}

const dataFlagSize = 2

// DataPayload returns unparsed payload of DATA packet, i.e. entire payload sans header and data flag.
func (dp *DataPacket) DataPayload() ([]byte, error) {
	const skipBytes = PacketHeaderSize + dataFlagSize
	payload := dp.Payload()
	if len(payload) < skipBytes {
		return nil, trace.BadParameter("data packet too small")
	}
	return bytes.Clone(payload[skipBytes:]), nil
}

func parseDataPacket(bp *basePacket) (*DataPacket, error) {
	const minSize = PacketHeaderSize + dataFlagSize

	if len(bp.Payload()) < minSize {
		return nil, trace.BadParameter("packet DataPacket too short (len=%v, buf=%v)", len(bp.Payload()), bp.Payload())
	}

	r := bytes.NewReader(bp.Payload())

	// header: ignored.
	_, err := readNBytes(r, PacketHeaderSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// flags
	buf, err := readNBytes(r, dataFlagSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	flags := binary.BigEndian.Uint16(buf)
	var data []byte

	// data id. commonly supplied, yet optional: in particular when connection is ending.
	dataId := DataID(-1)
	dataIdByte, err := r.ReadByte()
	if err == nil {
		dataId = DataID(dataIdByte)
		// remaining data, if present.
		if r.Len() > 0 {
			data, err = io.ReadAll(r)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
	}

	return &DataPacket{
		basePacket: *bp,
		Flags:      DataFlags(flags),
		DataID:     dataId,
		Data:       data,
	}, nil
}

func dataPacketBytes(largeSDU bool, payload []byte) []byte {
	output := bytes.Buffer{}

	const headerLen = 10

	header := make([]byte, headerLen)

	// first four bytes: payload length.
	if largeSDU {
		binary.BigEndian.PutUint32(header, uint32(headerLen+len(payload)))
		// header[5] is packet flag.
		header[5] = uint8(PacketFlagLargeSDU)
	} else {
		binary.BigEndian.PutUint16(header, uint16(headerLen+len(payload)))
	}

	// this is DATA packet, so always use that type.
	header[4] = uint8(DATA)

	// header[8:9] is for data flags, but we keep it zeroed out.

	output.Write(header)
	output.Write(payload)

	rawBytes := output.Bytes()

	return rawBytes
}

// DataPacketFromPayload creates a DATA packet from given payload and configuration.
func DataPacketFromPayload(largeSDU bool, payload []byte) (*DataPacket, error) {
	protocolVersion := uint16(TNSVersionMinLargeSdu)
	if !largeSDU {
		protocolVersion--
	}

	rawBytes := dataPacketBytes(largeSDU, payload)
	readResult, err := ReadPacket(protocolVersion, bytes.NewReader(rawBytes))
	if err != nil {
		return nil, trace.BadParameter("Error trying to round-trip DATA packet: %v (this is a bug)", err)
	}

	if readResult.SuccessPacket == nil {
		return nil, trace.BadParameter("Invalid DATA packet parse result, nil data (this is a bug)")
	}

	dataPacket, ok := readResult.SuccessPacket.(*DataPacket)
	if !ok {
		return nil, trace.BadParameter("Type error: expected DATA packet, but got %v (this is a bug)", readResult)
	}

	return dataPacket, nil
}
