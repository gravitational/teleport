package protocol

import "fmt"

// PacketType defines the TNS Oracle Protocol packet types.
// https://github.com/oracle/python-oracledb/blob/main/src/oracledb/impl/thin/constants.pxi#L33
type PacketType uint8

const (
	CONNECT  PacketType = 1
	ACCEPT   PacketType = 2
	REFUSE   PacketType = 4
	REDIRECT PacketType = 5
	DATA     PacketType = 6
	RESEND   PacketType = 11
	MARKER   PacketType = 12
	CONTROL  PacketType = 14
)

func (t PacketType) String() string {
	var name string

	switch t {
	case CONNECT:
		name = "CONNECT"
	case ACCEPT:
		name = "ACCEPT"
	case REFUSE:
		name = "REFUSE"
	case REDIRECT:
		name = "REDIRECT"
	case DATA:
		name = "DATA"
	case RESEND:
		name = "RESEND"
	case MARKER:
		name = "MARKER"
	case CONTROL:
		name = "CONTROL"
	default:
		name = "UNKNOWN"
	}

	return fmt.Sprintf("%v(%v)", name, uint8(t))
}

const (
	// PacketHeaderSize is the size in bytes of Oracle header.
	PacketHeaderSize = 8
	// TNSVersionMinLargeSdu is the 12.1 Oracle Server version.
	// https://github.com/oracle/python-oracledb/blob/main/src/oracledb/impl/thin/constants.pxi#L526
	TNSVersionMinLargeSdu = 315
)
