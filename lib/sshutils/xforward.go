package sshutils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

const (
	// DisplayEnv is an environment variable used to determine
	// the currently connected display.
	DisplayEnv = "DISPLAY"
	// mitMagicCookie is an xauth protocol.
	mitMagicCookie = "MIT-MAGIC-COOKIE-1"

	// x11Host is the host name for local XServers.
	x11Host = "localhost"
	// x11BasePort is the base port used for opening display ports.
	x11BasePort = 6000
	// x11MinDisplayNumber is the first display number allowed.
	x11MinDisplayNumber = 10
	// x11MaxDisplays is the number of displays which the
	// server will support concurrent x11 forwarding for.
	x11MaxDisplays = 1000
)

// X11ForwardRequestPayload according to http://www.ietf.org/rfc/rfc4254.txt
type X11ForwardRequestPayload struct {
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

// RequestX11Forward sends an "x11-req" to the server to set up
// x11 forwarding for the given session.
func RequestX11Forward(sess *ssh.Session) error {
	magicCookie, err := randomMITMagicCookie()
	if err != nil {
		return trace.Wrap(err)
	}

	payload := X11ForwardRequestPayload{
		SingleConnection: false,
		AuthProtocol:     mitMagicCookie,
		AuthCookie:       magicCookie,
		ScreenNumber:     0,
	}

	ok, err := sess.SendRequest(X11ForwardRequest, true, ssh.Marshal(payload))
	if err != nil {
		return trace.Wrap(err)
	} else if !ok {
		return trace.Errorf("x11 forward request failed")
	}

	return nil
}

// Generate a random 128-bit MIT-MAGIC-COOKIE-1
func randomMITMagicCookie() (string, error) {
	cookieBytes := make([]byte, 16)
	if _, err := rand.Read(cookieBytes); err != nil {
		return "", trace.Wrap(err)
	}
	return hex.EncodeToString(cookieBytes), nil
}

// StartX11ChannelListener creates an "x11" request channel to catch any
// "x11" requests to the ssh client and starts a goroutine to handle any
// requests received.
func StartX11ChannelListener(clt *ssh.Client) error {
	nchs := clt.HandleChannelOpen(X11ChannelRequest)
	if nchs == nil {
		return trace.AlreadyExists("x11 forwarding channel already open")
	}

	go serveX11ChannelListener(nchs)
	return nil
}

// serveX11ChannelListener handles new "x11" channel requests and copies
// data between the new ssh channel and the local display.
func serveX11ChannelListener(nchs <-chan ssh.NewChannel) {
	for nch := range nchs {
		// accept downstream X11 channel from server
		sch, _, err := nch.Accept()
		if err != nil {
			log.WithError(err).Warn("failed to accept x11 channel request")
			return
		}

		conn, err := openXClientConnection()
		if err != nil {
			log.WithError(err).Warn("failed to open connection to x11 unix socket")
			return
		}

		// copy data between the XClient conn and X11 channel
		go copyAndCloseWriter(conn, sch)
		go copyAndCloseWriter(sch, conn)
	}
}

// openXClientConnection opens a unix socket connection to the set
// X Display ($DISPLAY). If the Display doesn't look like a unix socket,
// the default "/tmp/.X11-unix/X[display_number]" will be used.
func openXClientConnection() (net.Conn, error) {
	display := os.Getenv(DisplayEnv)
	if display == "" {
		return nil, trace.BadParameter("display env variable is not set")
	}

	host, dNum, _ := parseDisplay(display)
	if !strings.HasPrefix(host, "/") {
		display = "/tmp/.X11-unix/X" + dNum
	}

	conn, err := net.Dial("unix", display)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return conn, nil
}

// parseDisplay parses the set X Display value - "hostname:display_number.screen_number"
func parseDisplay(display string) (host, dNum, sNum string) {
	hostEndIdx := strings.LastIndex(display, ":")
	if hostEndIdx == -1 || hostEndIdx == len(display) {
		return display, "0", ""
	}
	host = display[:hostEndIdx]

	dNumStartIdx := hostEndIdx + 1
	dNumEndIdx := strings.LastIndex(display, ".")
	if dNumEndIdx == -1 || dNumEndIdx == len(display) {
		return host, display[dNumStartIdx:], ""
	}

	sNumStartIdx := dNumEndIdx + 1
	return host, display[dNumStartIdx:dNumEndIdx], display[sNumStartIdx:]
}

// StartXServerListener creates a new XServer listener and starts a goroutine
// that handles any server requests received by copying data between the server
// connection and the ssh server connection. The XServer's X Display will be
// returned - "localhost:display_number.screen_number".
func StartXServerListener(sc *ssh.ServerConn, x11Req X11ForwardRequestPayload) (xaddr string, err error) {
	l, display, err := openXServerListener()
	if err != nil {
		return "", nil
	}

	// Close the listener once the server connection has closed
	go func() {
		sc.Wait()
		l.Close()
	}()

	go serveXServerListener(l, sc)
	return fmt.Sprintf("%s:%d.%d", x11Host, display, x11Req.ScreenNumber), nil
}

// openXServerListener opens a new local XServer listener on the first display available
// between 10 and 1010. The XServer address will be "localhost:[6000+display_number]".
func openXServerListener() (l net.Listener, display int, err error) {
	for display := x11MinDisplayNumber; display < x11MinDisplayNumber+x11MaxDisplays; display++ {
		port := strconv.Itoa(x11BasePort + display)
		l, err := net.Listen("tcp", net.JoinHostPort(x11Host, port))
		if err == nil {
			return l, display, nil
		}
	}
	return nil, 0, trace.LimitExceeded("No more x11 ports are available")
}

// serveXServerListener handles new XServer connections and copies
// data between the ssh server connection and XServer connection.
func serveXServerListener(l net.Listener, sc *ssh.ServerConn) {
	for {
		conn, err := l.Accept()
		if err != nil {
			// listener and server connection are closed
			if err == syscall.EINVAL {
				return
			}
			log.WithError(err).Debug("Failed to accept x11 server request")
			return
		}

		// Make an upstream request to open a new X11 channel.
		sch, _, err := sc.OpenChannel(X11ChannelRequest, nil)
		if err != nil {
			log.WithError(err).Debug("Failed to request x11 channel")
			return
		}

		// copy data between the XServer conn and X11 channel
		go copyAndCloseWriter(conn, sch)
		go copyAndCloseWriter(sch, conn)
	}
}

// copyAllAndClose copies all data from reader to writer and
// closes the writer once the reader's EOF is reached.
func copyAndCloseWriter(w io.WriteCloser, r io.ReadCloser) {
	defer w.Close()
	io.Copy(w, r)
}
