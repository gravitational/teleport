package x11

import (
	"io"
	"net"
	"sync"

	"golang.org/x/crypto/ssh"
)

const (
	// DisplayEnv is an environment variable used to determine
	// the currently connected display.
	DisplayEnv = "DISPLAY"

	// x11Host is the host name for local XServers.
	x11Host = "localhost"
	// x11BasePort is the base port used for opening display ports.
	x11BasePort = 6000
	// x11MinDisplayNumber is the first display number allowed.
	x11MinDisplayNumber = 10
	// x11MaxDisplays is the number of displays which the
	// server will support concurrent x11 forwarding for.
	x11MaxDisplays = 1000
	// x11UnixSocket is the unix socket used for x11 forwarding
	x11UnixSocket = ".X11-unix"
)

// ForwardRequestPayload according to http://www.ietf.org/rfc/rfc4254.txt
type ForwardRequestPayload struct {
	// SingleConnection determines whether any connections will be forwarded
	// after the first connection, or after the session is closed.
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

// X11ChannelRequestPayload according to http://www.ietf.org/rfc/rfc4254.txt
type X11ChannelRequestPayload struct {
	OriginatorAddress string
	OriginatorPort    uint32
}

func linkChannelConn(conn net.Conn, sch ssh.Channel) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		io.Copy(conn, sch)
		switch c := conn.(type) {
		case *net.UnixConn:
			c.CloseWrite()
		case *net.TCPConn:
			c.CloseWrite()
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		io.Copy(sch, conn)
		sch.CloseWrite()
	}()
	wg.Wait()
}
