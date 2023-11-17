package protocol

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
)

const (
	// PacketHeaderSize is the size in bytes of Oracle header.
	PacketHeaderSize = 8
	// TNSVersionMinLargeSdu is the 12.1 Oracle Server version.
	// https://github.com/oracle/python-oracledb/blob/main/src/oracledb/impl/thin/constants.pxi#L526
	TNSVersionMinLargeSdu = 315
)

const (
	defaultReaderCapacity = 32 * 1024
)
