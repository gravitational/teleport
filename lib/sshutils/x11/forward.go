package x11

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
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

	// x11BasePort is the base port used when searching
	// for an open XServer tunnel.
	x11BasePort = 6000
	// x11MaxDisplays is the number of displays which the
	// server will support concurrent x11 forwarding for.
	x11MaxDisplays = 1000
)

// ServerConfig is a server configuration for x11 forwarding
type ServerConfig struct {
	// Enabled controls whether x11 forwarding requests can be granted by the server.
	Enabled bool `yaml:"enabled"`
	// DisplayOffset tells the server what display to start searching from
	// for an open X11 Server reverse tunnel port (6000 + offset).
	DisplayOffset int `yaml:"display_offset,omitempty"`
	// UseLocalhost controls whether the server's localhost will be used
	// to create a fake X11 server when forwarding. When false, the server's
	// hostname will be used instead.
	UseLocalhost bool `yaml:"use_localhost,omitempty"`
}

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
	// OriginatorAddress is the address of the server requesting an x11 channel
	OriginatorAddress string
	// OriginatorPort is the port of the server requesting an x11 channel
	OriginatorPort uint32
}

// RequestX11Forwarding sends an "x11-req" to the server to set up x11 forwarding for the given session.
// authProto and authCookie are required to set up authentication with the Server. screenNumber is used
// by the server to determine which screen should be connected to for x11 forwarding. singleConnection is
// an optional argument to request x11 forwarding for a single connection.
func RequestX11Forwarding(sess *ssh.Session, display, authProto, authCookie string, singleConnection bool) error {
	_, _, screenNumber, err := parseDisplay(display)
	if err != nil {
		return trace.Wrap(err)
	}

	payload := ForwardRequestPayload{
		SingleConnection: singleConnection,
		AuthProtocol:     authProto,
		AuthCookie:       authCookie,
		ScreenNumber:     uint32(screenNumber),
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

// OpenNewXServerListener opens a new local XServer listener with first unused display
// number between 10 and 1010. The XServer listener's corresponding X display will be returned.
func OpenNewXServerListener(displayOffset int, useLocalhost bool, screen uint32) (l net.Listener, display string, err error) {
	host := "localhost"
	if !useLocalhost {
		var err error
		if host, err = os.Hostname(); err != nil {
			return nil, "", trace.Wrap(err)
		}
	}

	for display := displayOffset; display < displayOffset+x11MaxDisplays; display++ {
		port := strconv.Itoa(x11BasePort + display)
		l, err := net.Listen("tcp", net.JoinHostPort(host, port))
		if err == nil {
			display := fmt.Sprintf("%s:%d.%d", host, display, screen)
			return l, display, nil
		}
	}

	return nil, "", trace.LimitExceeded("No more x11 ports are available")
}

// ForwardToX11Channel begins x11 forwarding between the given
// XServer connection and a new x11 channel.
func ForwardToX11Channel(conn net.Conn, sc *ssh.ServerConn) error {
	originHost, originPort, err := net.SplitHostPort(sc.LocalAddr().String())
	if err != nil {
		return trace.Wrap(err)
	}
	originPortI, err := strconv.Atoi(originPort)
	if err != nil {
		return trace.Wrap(err)
	}
	x11ChannelReq := X11ChannelRequestPayload{
		OriginatorAddress: originHost,
		OriginatorPort:    uint32(originPortI),
	}

	sch, _, err := sc.OpenChannel(sshutils.X11ChannelRequest, ssh.Marshal(x11ChannelReq))
	if err != nil {
		return trace.Wrap(err)
	}
	defer sch.Close()

	// copy data between the X11 channel and the XServer conn
	ForwardIO(sch, conn)
	return nil
}

// ForwardToXDisplay opens up an x11 channel listener and serves any channel
// requests by beginning X11Forwarding between the channel and given display.
// the x11 channel's intial auth packet is scanned for the given fakeCookie.
// If the cookie is present, it will be replaced with the real cookie.
// Otherwise, an access denied error will be returned.
func ForwardToXDisplay(sch ssh.Channel, display, authProto, fakeCookie, realCookie string) error {
	conn, err := dialDisplay(display)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	if err := scanAndReplaceXAuthData(sch, conn, authProto, fakeCookie, realCookie); err != nil {
		return trace.Wrap(err)
	}

	// copy data between the X11 channel and the XClient conn
	ForwardIO(sch, conn)
	return nil
}

// scanAndReplaceXAuthData reads the initial xauth packet from the x11 channel. The xauth packet has 2 parts:
//  1. fixed size buffer (12 bytes) - holds byteOrder bit, and the sizes of the protocol string and auth data
//  2. variable size xauth packet - holds xauth protocol and data used to connect to the remote XServer.
//
// Then it compares the received auth packet with the auth proto and fake cookie sent to the server with the original "x11-req".
// If the data matches, the fake cookie is replaced with the real cookie to provide access to the client's X display.
func scanAndReplaceXAuthData(sch ssh.Channel, conn net.Conn, authProto, fakeCookie, realCookie string) error {
	// xauth packet starts with a fixed sized buffer of 12 bytes
	// which is used to size and decode the remaining bytes
	initBufSize := 12
	initBuf := make([]byte, initBufSize)
	if _, err := io.ReadFull(sch, initBuf); err != nil {
		return trace.Wrap(err, "x11 channel initial packet buffer missing or too short")
	}

	var protoLen, dataLen int
	switch byteOrder := initBuf[0]; byteOrder {
	///* Byte order MSB first. */
	case 0x42:
		protoLen = int(binary.BigEndian.Uint16(initBuf[6:8]))
		dataLen = int(binary.BigEndian.Uint16(initBuf[8:10]))
	///* Byte order LSB first. */
	case 0x6c:
		protoLen = int(binary.LittleEndian.Uint16(initBuf[6:8]))
		dataLen = int(binary.LittleEndian.Uint16(initBuf[8:10]))
	default:
		return trace.Errorf("x11 channel auth packet has invalid byte order: ", byteOrder)
	}

	authPacketSize := protoLen + dataLen + ((protoLen + 2*protoLen%4) % 4)
	authPacket := make([]byte, authPacketSize)
	if _, err := io.ReadFull(sch, authPacket); err != nil {
		return trace.Wrap(err, "x11 channel auth packet missing or too short")
	}

	proto := authPacket[:protoLen]
	data := authPacket[len(authPacket)-dataLen:]
	if string(proto) != authProto || hex.EncodeToString(data) != fakeCookie {
		return trace.AccessDenied("x11 channel uses different authentication from what client provided")
	}

	realAuthData, err := hex.DecodeString(realCookie)
	if err != nil {
		return trace.Wrap(err)
	}

	// Replace auth data with the real auth data and write to conn
	for i := 0; i < len(data); i++ {
		data[i] = realAuthData[i]
	}

	_, err = conn.Write(append(initBuf, authPacket...))
	return trace.Wrap(err)
}

// ForwardIO forwards io data bidirectionally between an x11 channel and the other end of
// the x11 forwarding connection (XServer, XClient, or another x11 channel)
func ForwardIO(sch ssh.Channel, conn io.ReadWriteCloser) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		io.Copy(sch, conn)
		sch.CloseWrite()
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		io.Copy(conn, sch)
		// prefer CloseWrite over Close to prevent reading from halting early.
		switch c := conn.(type) {
		case interface{ CloseWrite() error }:
			c.CloseWrite()
		default:
			c.Close()
		}
	}()
	wg.Wait()
}

// sendRequest represents a resource capable of sending an ssh request such as
// an ssh.Channel or ssh.Session.
type sendRequest interface {
	SendRequest(name string, wantReply bool, payload []byte) (bool, error)
}

// ForwardRequest is a helper for forwarding a request across a session or channel.
func ForwardRequest(sender sendRequest, req *ssh.Request) (bool, error) {
	reply, err := sender.SendRequest(req.Type, req.WantReply, req.Payload)
	if err != nil || !req.WantReply {
		return reply, trace.Wrap(err)
	}
	return reply, trace.Wrap(req.Reply(reply, nil))
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

// dialDisplay connects to the set $DISPLAY variable. If it looks like a
// unix socket, we dial to the default /temp/.X11-unix/X* socket.
// Otherwise, we dial a tcp connection to it.
func dialDisplay(display string) (net.Conn, error) {
	hostname, displayNumber, _, err := parseDisplay(display)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If display is a unix socket, dial the default x11 unix socket
	// "/tmp/.X11-unix/X[display_number]" for the display number found in $DISPLAY.
	if hostname == "unix" || hostname == "" {
		sock := fmt.Sprintf("%s/.X11-unix/X%d", os.TempDir(), displayNumber)
		return net.Dial("unix", sock)
	}

	// If hostname can be parsed as an IP address, dial tcp with port 6000+display.
	// MobaXTerm expects x11 forwarding to dial "localhost:6000+display".
	if ip := net.ParseIP(hostname); ip != nil {
		conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", hostname, x11BasePort+displayNumber))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return conn, nil
	}

	// dial display as generic socket address
	return net.Dial("unix", display)
}
