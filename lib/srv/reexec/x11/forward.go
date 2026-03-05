package x11

const (
	// ForwardRequest is a request to initiate X11 forwarding.
	ForwardRequest = "x11-req"

	// ChannelRequest is the type of an X11 forwarding channel.
	ChannelRequest = "x11"
)

// ForwardRequestPayload according to http://www.ietf.org/rfc/rfc4254.txt
type ForwardRequestPayload struct {
	// SingleConnection determines whether any connections will be forwarded
	// after the first connection, or after the session is closed. In OpenSSH
	// and Teleport SSH clients, SingleConnection is always set to false.
	SingleConnection bool
	// AuthProtocol is the name of the X11 authentication protocol being used.
	AuthProtocol string
	// AuthCookie is a hexadecimal encoded X11 authentication cookie. This should
	// be a fake, random cookie, which will be checked and replaced by the real
	// cookie once the connection request is received.
	AuthCookie string
	// ScreenNumber determines which screen will be.
	ScreenNumber uint32
}

// ChannelRequestPayload according to http://www.ietf.org/rfc/rfc4254.txt
type ChannelRequestPayload struct {
	// OriginatorAddress is the address of the server requesting an X11 channel
	OriginatorAddress string
	// OriginatorPort is the port of the server requesting an X11 channel
	OriginatorPort uint32
}

// ServerConfig is a server configuration for X11 forwarding
type ServerConfig struct {
	// Enabled controls whether X11 forwarding requests can be granted by the server.
	Enabled bool
	// DisplayOffset tells the server what X11 display number to start from when
	// searching for an open X11 unix socket for XServer proxies.
	DisplayOffset int
	// MaxDisplay tells the server what X11 display number to stop at when
	// searching for an open X11 unix socket for XServer proxies.
	MaxDisplay int
}
