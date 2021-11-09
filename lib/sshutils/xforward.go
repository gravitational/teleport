package sshutils

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

type x11Request struct {
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

func RequestX11Channel(sess *ssh.Session) error {
	magicCookie, err := randomMITMagicCookie()
	if err != nil {
		return trace.Wrap(err)
	}

	payload := x11Request{
		SingleConnection: false,
		AuthProtocol:     MITMagicCookie,
		AuthCookie:       magicCookie,
		ScreenNumber:     0,
	}

	ok, err := sess.SendRequest(X11ForwardRequest, true, ssh.Marshal(payload))
	if err != nil {
		return trace.Wrap(err)
	} else if !ok {
		return trace.BadParameter("x11 channel request failed")
	}

	return nil
}

func HandleX11Channel(clt *ssh.Client) error {
	xchs := clt.HandleChannelOpen(X11ChannelRequest)
	if xchs == nil {
		return trace.AlreadyExists("x11 forwarding channel already open")
	}
	go func() {
		for ch := range xchs {
			go handleX11ChannelRequest(ch)
		}
	}()
	return nil
}

// handleX11ChannelRequest accepts an X11 channel and forwards it back to the client.
// Servers which support X11 forwarding request a separate channel for serving each
// inbound connection on the X11 socket of the remote session.
func handleX11ChannelRequest(xreq ssh.NewChannel) {
	// accept inbound X11 channel from server
	sch, _, err := xreq.Accept()
	if err != nil {
		log.Errorf("x11 channel fwd failed: %v", err)
		return
	}
	defer sch.Close()

	// open a unix socket for the X11 display
	conn, err := dialX11Display(os.Getenv("DISPLAY"))
	if err != nil {
		log.Errorf("x11 channel fwd failed: %v", err)
		return
	}
	defer conn.Close()

	// setup wait group for io forwarding goroutines
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		// forward data from client to X11 unix socket
		io.Copy(conn, sch)
		// inform unix socket that no more data is coming
		conn.(*net.UnixConn).CloseWrite()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		// forward data from x11 unix socket to client
		io.Copy(sch, conn)
		// inform server that no more data is coming
		sch.CloseWrite()
	}()
	wg.Wait()
}

// dialX11Display dials the x11 socket for the given display.
func dialX11Display(display string) (net.Conn, error) {
	if !strings.HasPrefix(display, "/") {
		display = "/tmp/.X11-unix/X" + parseDisplayNumber(display)
	}
	return net.Dial("unix", display)
}

// Parse unix DISPLAY value e.g. [hostname]:[display_number].[screen_number]
func parseDisplayNumber(display string) string {
	displayNumIdx := strings.LastIndex(display, ":") + 1
	if displayNumIdx == 0 {
		return "0"
	}

	// If there is a "." after the display_number, slice up to the "."
	if dotIdx := strings.LastIndex(display, "."); dotIdx > displayNumIdx {
		return display[displayNumIdx:dotIdx]
	}
	return display[displayNumIdx:]
}

// MITMagicCookie is an xauth protocol
const MITMagicCookie = "MIT-MAGIC-COOKIE-1"

// Generate a random 128-bit MIT-MAGIC-COOKIE-1
func randomMITMagicCookie() (string, error) {
	cookieBytes := make([]byte, 16)
	if _, err := rand.Read(cookieBytes); err != nil {
		return "", trace.Wrap(err)
	}
	return hex.EncodeToString(cookieBytes), nil
}
