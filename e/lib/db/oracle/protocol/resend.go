package protocol

// ResendPacket defines TNS oracle package
// that is used by Oracle Server to indicate a client
// to resend all  connection data.
type ResendPacket struct {
	*packet
}
