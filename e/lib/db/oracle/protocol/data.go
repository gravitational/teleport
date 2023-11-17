package protocol

import (
	"bytes"
	"io"

	"github.com/gravitational/trace"
)

// DataID is a Data packet type.
type DataID byte

var (
	// ReturnOPIParameterDataID is a Data Packet type TNS_MSG_TYPE_PARAMETER
	// That indicates that the Data Packet contains server configuration parameters.
	// https://github.com/oracle/python-oracledb/blob/main/src/oracledb/impl/thin/constants.pxi#L402
	ReturnOPIParameterDataID DataID = 8
)

const (
	// AuthSessionIDKey is the server params key used to obtain a unique Oracle user session ID.
	AuthSessionIDKey = "AUTH_SESSION_ID"
	// AuthSCServiceNameKey is the server param key use to obtain the Oracle Service Name.
	AuthSCServiceNameKey = "AUTH_SC_SERVICE_NAME"
)

// DataPacket defines TNS data oracle packet
// that is used a generic transport unit in oracle wire protocol.
type DataPacket struct {
	*packet
	DataType   DataID
	Parameters map[string]string
}

func parseDataPacket(bp *packet, c *oracleConn) (Packet, error) {
	r := bytes.NewReader(bp.buff)
	if _, err := r.Seek(PacketHeaderSize, io.SeekCurrent); err != nil {
		return nil, trace.Wrap(err)
	}
	if _, err := r.Seek(2, io.SeekCurrent); err != nil {
		return nil, trace.Wrap(err)
	}

	dataType, err := r.ReadByte()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var paramMap = map[string]string{}
	switch DataID(dataType) {
	case ReturnOPIParameterDataID:
		p, err := parseDataPacketOPIParameter(bp, r, c)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return p, nil
	}

	return &DataPacket{
		packet:     bp,
		DataType:   DataID(dataType),
		Parameters: paramMap,
	}, nil
}

// parseDataPacketOPIParameters  extracts server parameters from the ReturnOPIParameterDataID packet.
// https://github.com/oracle/python-oracledb/blob/main/src/oracledb/impl/thin/messages.pyx#L850
func parseDataPacketOPIParameter(bp *packet, r io.Reader, c *oracleConn) (Packet, error) {
	var paramMap = map[string]string{}
	paramLength, err := readInt64(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for i := 0; i < int(paramLength); i++ {
		key, val, err := readKeyValue(r)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		paramMap[key] = val
	}
	c.sessionID = paramMap[AuthSessionIDKey]
	c.connParamReceived = true
	return &DataPacket{
		packet:     bp,
		DataType:   ReturnOPIParameterDataID,
		Parameters: paramMap,
	}, nil

}

func readKeyValue(r io.Reader) (string, string, error) {
	keyBuff, err := readDataLengthContent(r)
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	valBuff, err := readDataLengthContent(r)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	// skip unused bytes.
	_, err = readInt64(r)
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	return string(keyBuff), string(valBuff), nil
}
