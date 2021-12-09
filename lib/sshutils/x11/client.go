package x11

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"

	"github.com/gravitational/teleport/lib/sshutils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// RequestX11Forwarding sends an "x11-req" to the server to set up
// x11 forwarding for the given session.
func RequestX11Forwarding(ctx context.Context, sess *ssh.Session, clt *ssh.Client, trusted, singleConnection bool) error {
	// TODO: GetDisplayEnv and pass to functions higher in call stack
	display, err := displayFromEnv()
	if err != nil {
		return trace.Wrap(err)
	}

	screenNumber, err := parseDisplayScreenNumber(display)
	if err != nil {
		return trace.Wrap(err)
	}

	var xauthEntry *xAuthEntry
	if trusted {
		log.Debug("obtaining trusted auth token for x11 forwarding")
		// use existing real auth token
		xauthEntry, err = readXAuthEntry(ctx, "", display)
		if err != nil {
			log.Debugf("failed to obtain trusted auth token: %s", err)
			log.Debugf("creating fake auth token for trusted x11 forwarding")
			// if we can't get the real auth token, a new fake one will suffice.

			xauthEntry, err = newFakeXAuthEntry(display)
			if err != nil {
				return trace.Wrap(err)
			}
		}
	} else {
		log.Debug("generating untrusted auth token for x11 forwarding")
		// create a new untrusted auth token
		xauthEntry, err = generateUntrustedXAuthEntry(ctx, display, 0)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	// Create a fake xauth entry which will be used to authenticate incoming
	// server requests. Once the channel is created, the real auth cookie will
	// be used to authorize the XServer.
	// (somehow? Do we actually need to grab the cookie? do we just grab it to make a realistic spoof?)
	fakeXAuthEntry, err := xauthEntry.spoof()
	if err != nil {
		return trace.Wrap(err)
	}

	if err := StartX11ChannelListener(clt, fakeXAuthEntry, xauthEntry); err != nil {
		return trace.Wrap(err)
	}

	payload := ForwardRequestPayload{
		SingleConnection: singleConnection,
		AuthProtocol:     xauthEntry.proto,
		AuthCookie:       fakeXAuthEntry.cookie,
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

// StartX11ChannelListener creates an "x11" request channel to catch any
// "x11" requests to the ssh client and starts a goroutine to handle any
// requests received.
func StartX11ChannelListener(clt *ssh.Client, fakeXAuthEntry *xAuthEntry, realXAuthEntry *xAuthEntry) error {
	nchs := clt.HandleChannelOpen(sshutils.X11ChannelRequest)
	if nchs == nil {
		return trace.AlreadyExists("x11 forwarding channel already open")
	}

	go serveX11ChannelListener(nchs, fakeXAuthEntry, realXAuthEntry)
	return nil
}

// serveX11ChannelListener handles new "x11" channel requests and copies
// data between the new ssh channel and the local display.
func serveX11ChannelListener(nchs <-chan ssh.NewChannel, fakeXAuthEntry *xAuthEntry, realXAuthEntry *xAuthEntry) {
	for nch := range nchs {
		var x11ChannelReq X11ChannelRequestPayload
		if err := ssh.Unmarshal(nch.ExtraData(), &x11ChannelReq); err != nil {
			log.WithError(err).Warn("failed to unmarshal x11 channel request payload")
			if err := nch.Reject(ssh.Prohibited, "invalid payload"); err != nil {
				log.WithError(err).Error("failed to reject ssh channel")
			}
			return
		}

		log.Debugf("received x11 channel request from %s:%d", x11ChannelReq.OriginatorAddress, x11ChannelReq.OriginatorPort)

		sch, _, err := nch.Accept()
		if err != nil {
			log.WithError(err).Warn("failed to accept x11 channel request")
			return
		}

		go func(sch ssh.Channel) {
			defer sch.Close()

			conn, err := dialDisplay()
			if err != nil {
				log.WithError(err).Warn("failed to open connection to x11 unix socket")
				return
			}
			defer conn.Close()

			if err := scanAndReplaceXAuthData(sch, conn, fakeXAuthEntry, realXAuthEntry); err != nil {
				log.WithError(err).Debug("x11 channel has invalid authentication data")
				// TODO (Joerger): Improve logging
				// Print to stdout if someone tries to connect with wrong auth (same as openssh)
				fmt.Println("X11 connection rejected because of wrong authentication.")
				return
			}

			// copy data between the XClient conn and X11 channel
			linkChannelConn(conn, sch)
		}(sch)
	}
}

// scanAndReplaceXAuthData reads the initial xauth packet from the x11 channel. The xauth packet has 2 parts:
//  1. fixed size buffer (12 bytes) - holds byteOrder bit, and the sizes of the protocol string and auth data
//  2. variable size xauth packet - holds xauth protocol and data used to connect to the remote XServer.
//
// Then it compares the received auth packet with the fakeXAuthEntry sent to the server with the original "x11-req".
// If the data matches, the fakeXAuthEntry is replaced with the real (trusted/untrusted) cookie for the client's XAuthority.
func scanAndReplaceXAuthData(sch ssh.Channel, conn net.Conn, fakeXAuthEntry *xAuthEntry, realXAuthEntry *xAuthEntry) error {
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

	log.Debug("fake proto: ", string(authPacket[:protoLen]))
	log.Debug("fake data: ", hex.EncodeToString(data))

	if string(proto) != fakeXAuthEntry.proto || hex.EncodeToString(data) != fakeXAuthEntry.cookie {
		return trace.AccessDenied("x11 channel uses different authentication from what client provided")
	}

	realAuthData, err := hex.DecodeString(realXAuthEntry.cookie)
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
