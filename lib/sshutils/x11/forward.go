package x11

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

const (
	// DefaultDisplayOffset is the default display offset when
	// searchinf for an X11 Server reverse tunnel port.
	DefaultDisplayOffset = 10
	// DisplayEnv is an environment variable used to determine what
	// local display should be connected to during x11 forwarding.
	DisplayEnv = "DISPLAY"

	// x11BasePort is the base port used for xserver tcp addresses.
	// e.g. DISPLAY=localhost:10 -> net.Dial("tcp", "localhost:6010")
	// Used by some XServer clients, such as openSSH and MobaXTerm.
	x11BasePort = 6000
	// x11MaxDisplays is the number of displays which the
	// server will support concurrent x11 forwarding for.
	x11MaxDisplays = 1000
	// x11SocketDir is the name of the directory where x11 unix sockets are kept.
	x11SocketDir = ".X11-unix"
)

// ServerConfig is a server configuration for x11 forwarding
type ServerConfig struct {
	// Enabled controls whether x11 forwarding requests can be granted by the server.
	Enabled bool `yaml:"enabled"`
	// DisplayOffset tells the server what x11 display number to start from.
	DisplayOffset int `yaml:"display_offset,omitempty"`
}

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

// X11ChannelRequestPayload according to http://www.ietf.org/rfc/rfc4254.txt
type X11ChannelRequestPayload struct {
	// OriginatorAddress is the address of the server requesting an x11 channel
	OriginatorAddress string
	// OriginatorPort is the port of the server requesting an x11 channel
	OriginatorPort uint32
}

// RequestX11Forwarding sends an "x11-req" to the server to set up x11 forwarding for the given session.
// authProto and authCookie are required to set up authentication with the Server. screenNumber is used
// by the server to determine which screen should be connected to for x11 forwarding. singleConnection is
// an optional argument to request x11 forwarding for a single connection.
func RequestX11Forwarding(sess *ssh.Session, display, authProto, authCookie string) error {
	_, _, screenNumber, err := parseDisplay(display)
	if err != nil {
		return trace.Wrap(err)
	}

	payload := ForwardRequestPayload{
		AuthProtocol: authProto,
		AuthCookie:   authCookie,
		ScreenNumber: uint32(screenNumber),
	}

	ok, err := sess.SendRequest(sshutils.X11ForwardRequest, true, ssh.Marshal(payload))
	if err != nil {
		return trace.Wrap(err)
	} else if !ok {
		return trace.Errorf("x11 forward request failed")
	}

	return nil
}

type x11ChannelHandler func(ctx context.Context, nch ssh.NewChannel)

// ServeX11ChannelRequests opens an x11 channel handler and starts a
// goroutine to serve any channels received with the handler provided.
func ServeX11ChannelRequests(ctx context.Context, clt *ssh.Client, handler x11ChannelHandler) error {
	channels := clt.HandleChannelOpen(sshutils.X11ChannelRequest)
	if channels == nil {
		return trace.AlreadyExists("x11 forwarding channel already open")
	}

	go func() {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		for {
			select {
			case nch := <-channels:
				if nch == nil {
					return
				}
				go handler(ctx, nch)
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

// OpenNewXServerListener opens a new XServer unix socket with the first unused display
// number. The unix socket's corresponding XServer display value will also be returned.
func OpenNewXServerListener(displayOffset int, screen uint32) (net.Listener, string, error) {
	for displayNumber := displayOffset; displayNumber < displayOffset+x11MaxDisplays; displayNumber++ {
		if l, err := net.Listen("unix", xserverUnixSocket(displayNumber)); err == nil {
			return l, unixXDisplay(displayNumber), nil
		}
	}

	return nil, "", trace.LimitExceeded("No more x11 sockets are available")
}

// Forward begins x11 forwarding between an xserver connection and an x11 channel.
// The xserver connection may be a direct xserver connection, or another x11 channel.
func Forward(xconn io.ReadWriteCloser, x11Chan ssh.Channel) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		io.Copy(x11Chan, xconn)
		x11Chan.CloseWrite()
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		io.Copy(xconn, x11Chan)
		// prefer CloseWrite over Close to prevent reading from halting pre-maturely.
		switch c := xconn.(type) {
		case interface{ CloseWrite() error }:
			c.CloseWrite()
		default:
			c.Close()
		}
	}()
	wg.Wait()
}

// DialXDisplay connects to the local xserver set in the $DISPLAY variable.
func DialXDisplay(display string) (net.Conn, error) {
	hostname, displayNumber, _, err := parseDisplay(display)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If display is a unix socket, dial the address as an x11 unix socket
	if hostname == "unix" || hostname == "" {
		return net.Dial("unix", xserverUnixSocket(displayNumber))
	}

	// If hostname can be parsed as an IP address, dial the address as an x11 tcp socket
	if ip := net.ParseIP(hostname); ip != nil {
		conn, err := net.Dial("tcp", xserverTCPSocket(hostname, displayNumber))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return conn, nil
	}

	// dial display as generic socket address
	return net.Dial("unix", display)
}

// parsesDisplay parses the given display value and returns the host,
// display number, and screen number, or a parsing error. display
// should be in the format "hostname:displayNumber.screenNumber".
func parseDisplay(display string) (string, int, int, error) {
	splitHost := strings.Split(display, ":")
	host := splitHost[0]
	if len(splitHost) < 2 {
		return host, 0, 0, nil
	}

	splitDisplayNumber := strings.Split(splitHost[1], ".")
	displayNumber, err := strconv.Atoi(splitDisplayNumber[0])
	if err != nil {
		return "", 0, 0, trace.Wrap(err)
	}
	if len(splitDisplayNumber) < 2 {
		return host, displayNumber, 0, nil
	}

	screenNumber, err := strconv.Atoi(splitDisplayNumber[1])
	if err != nil {
		return "", 0, 0, trace.Wrap(err)
	}

	return host, displayNumber, screenNumber, nil
}

// xserverUnixSocket returns the display's associated
// unix socket - "/tmp/.X11-unix/X<display_number>"
func xserverUnixSocket(display int) string {
	return filepath.Join(os.TempDir(), x11SocketDir, fmt.Sprintf("X%d", display))
}

// xserverTCPSocket returns the display's associated tcp socket
// with the given hostname - "hostname:<6000+display_number>"
func xserverTCPSocket(hostname string, display int) string {
	return fmt.Sprintf("%s:%d", hostname, x11BasePort+display)
}

// unixXDisplay returns the xserver display value for an xserver
// unix socket with the given display number - "unix:<display_number"
func unixXDisplay(displayNumber int) string {
	return fmt.Sprintf("unix:%d", displayNumber)
}
