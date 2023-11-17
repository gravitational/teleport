package protocol

// RedirectPacket defines TNS oracle package
// that is used by Oracle Server to indicate a client
// to redirect connection.
type RedirectPacket struct {
	*packet
}
