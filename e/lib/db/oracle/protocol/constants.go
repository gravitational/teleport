package protocol

import "fmt"

// Type defines the TNS Oracle Protocol types.
// https://github.com/oracle/python-oracledb/blob/main/src/oracledb/impl/thin/constants.pxi#L33
type Type uint8

const (
	CONNECT  Type = 1
	ACCEPT   Type = 2
	REFUSE   Type = 4
	REDIRECT Type = 5
	DATA     Type = 6
	RESEND   Type = 11
	MARKER   Type = 12
	CONTROL  Type = 14
)

func (t Type) String() string {
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
