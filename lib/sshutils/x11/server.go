package x11

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"syscall"

	"github.com/gravitational/teleport/lib/sshutils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// StartXServerListener creates a new XServer listener and starts a goroutine
// that handles any server requests received by copying data between the server
// connection and the ssh server connection. The XServer's X Display will be
// returned - "localhost:display_number.screen_number".
func StartXServerListener(ctx context.Context, sc *ssh.ServerConn, x11Req ForwardRequestPayload) (xaddr string, err error) {
	l, display, err := openXServerListener()
	if err != nil {
		return "", nil
	}

	// Close the listener once the server connection has closed
	go func() {
		sc.Wait()
		l.Close()
	}()

	originHost, originPort, err := net.SplitHostPort(sc.LocalAddr().String())
	if err != nil {
		return "", trace.Wrap(err)
	}

	originPortI, err := strconv.Atoi(originPort)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Store the original x11Req data to authenticate x11 channel request with the client
	x11ChannelReq := X11ChannelRequestPayload{
		OriginatorAddress: originHost,
		OriginatorPort:    uint32(originPortI),
	}
	xauthFile, err := xauthHomePath()
	if err != nil {
		return "", trace.Wrap(err)
	}
	displayString := fmt.Sprintf("%s:%d.%d", x11Host, display, x11Req.ScreenNumber)
	if err := removeXAuthEntry(ctx, xauthFile, displayString); err != nil {
		return "", trace.Wrap(err)
	}
	if err := addXAuthEntry(ctx, xauthFile, displayString, x11Req.AuthProtocol, x11Req.AuthCookie); err != nil {
		return "", trace.Wrap(err)
	}
	go serveXServerListener(l, sc, ssh.Marshal(x11ChannelReq))
	return displayString, nil
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
func serveXServerListener(l net.Listener, sc *ssh.ServerConn, payload []byte) {
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
		defer conn.Close()

		sch, _, err := sc.OpenChannel(sshutils.X11ChannelRequest, payload)
		if err != nil {
			log.WithError(err).Debug("Failed to request x11 channel")
			return
		}
		defer sch.Close()

		// copy data between the XServer conn and X11 channel
		linkChannelConn(conn, sch)
	}
}
