package protocol

import (
	"bytes"
	"encoding/binary"
	"math"

	"github.com/gravitational/trace"
)

const snsMarker = uint32(0xDEADBEEF)

// VerifySNSPacket reads the header of incoming packet to check for expected marker and possible error flags.
func VerifySNSPacket(incomingPacket *DataPacket) error {
	buf, err := incomingPacket.DataPayload()
	if err != nil {
		return trace.Wrap(err)
	}

	payload := bytes.NewReader(buf)

	_, err = readSNSHeader(payload)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// readSNSHeader reads the header of SNS packet. The header should contain SNS marker 0xDEADBEEF,
// a service count number and an error flag set to zero. If the flag is not zero, an error will be raised.
// The service count number is a number of services (auth, encryption, checksum, ...) that can be found
// in the body of the packet.
func readSNSHeader(incomingPacket *bytes.Reader) (int, error) {
	buf, err := readNBytes(incomingPacket, 4)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	marker := binary.BigEndian.Uint32(buf)
	if marker != snsMarker {
		return 0, trace.BadParameter("header mismatch: expected %x, got %x", snsMarker, marker)
	}

	// skip following 6 bytes
	_, err = readNBytes(incomingPacket, 6)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	buf, err = readNBytes(incomingPacket, 2)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	serviceCount := binary.BigEndian.Uint16(buf)

	errorFlag, err := incomingPacket.ReadByte()
	if err != nil {
		return 0, trace.Wrap(err)
	}
	if errorFlag != 0 {
		return 0, trace.BadParameter("SNS header: server returned error flag %v", errorFlag)
	}

	return int(serviceCount), nil
}

func readFieldHeader(incomingPacket *bytes.Reader) (uint16, error) {
	buf, err := readNBytes(incomingPacket, 2)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	length := binary.BigEndian.Uint16(buf)

	// optional validation
	// buf, err = readNBytes(incomingPacket, 2)
	// marker := binary.BigEndian.Uint16(buf)
	// if marker != 0 {
	// 	return 0, trace.BadParameter("unexpected field header marker: %v", marker)
	// }

	_, err = readNBytes(incomingPacket, 2)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	return length, nil
}

func snsReadString(incomingPacket *bytes.Reader) (string, error) {
	stringLen, err := readFieldHeader(incomingPacket)
	if err != nil {
		return "", err
	}
	resultBytes, err := readNBytes(incomingPacket, int(stringLen))
	if err != nil {
		return "", err
	}
	return string(resultBytes), nil
}

// KerberosAuthParams define SPN for which the Kerberos ticket must be fetched.
type KerberosAuthParams struct {
	// ServiceClass is a part of SPN. Usually it would be "HTTP" or "LDAP" or "MSSQLSvc".
	// Not so for Oracle: this will be a database name, e.g. "db-lx5d5muz5cztmw6mq".
	ServiceClass string
	// ServerInstance is part of SPN. Oracle will put a domain name here, e.g. "db.oraad.com".
	ServerInstance string
}

// ParseKerberosAuthParams parses the data packet with Kerberos auth params.
func ParseKerberosAuthParams(incomingPacket *DataPacket) (*KerberosAuthParams, error) {
	buf, err := incomingPacket.DataPayload()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	payload := bytes.NewReader(buf)

	serviceCount, err := readSNSHeader(payload)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// discard service headers
	const serviceHeaderSize = 8
	_, err = readNBytes(payload, serviceHeaderSize*serviceCount)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	serviceClass, err := snsReadString(payload)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(serviceClass) == 0 {
		return nil, trace.BadParameter("kerberos negotiation error: received empty service class")
	}

	serverInstance, err := snsReadString(payload)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(serverInstance) == 0 {
		return nil, trace.BadParameter("kerberos negotiation error: received empty server instance")
	}

	return &KerberosAuthParams{
		ServiceClass:   serviceClass,
		ServerInstance: serverInstance,
	}, nil
}

// BuildKerberosTokenPayload builds a payload of a data packet containing given Kerberos token.
func BuildKerberosTokenPayload(token []byte) ([]byte, error) {
	if len(token) >= math.MaxUint16 {
		return nil, trace.BadParameter("kerberos token length too large")
	}

	var payload []byte

	payload = binary.BigEndian.AppendUint32(payload, snsMarker)
	payload = binary.BigEndian.AppendUint16(payload, 0xFFFF) // placeholder for total packet length

	// fixed bytes that never change.
	const fixed = `
00000000  0b 20 02 00 00 01 00 00  01 00 04 00 00 00 00 00  |. ..............|
00000010  02 00 03 00 02 00 04 00  04 00 00 00 04 00 04 00  |................|
00000020  01 7f 00 00 01                                    |.....|`
	fixedBytes, err := DecodeHexDump(fixed)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	payload = append(payload, fixedBytes...)

	// Kerberos token goes here.
	payload = binary.BigEndian.AppendUint16(payload, uint16(len(token)))
	payload = binary.BigEndian.AppendUint16(payload, 1) // type tag for "token length" field
	payload = append(payload, token...)

	// Replace the placeholder total packet length with the real value.
	binary.BigEndian.PutUint16(payload[4:], uint16(len(payload)))

	return payload, nil
}
