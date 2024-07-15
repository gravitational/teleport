// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package resumption

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/multiplexer"
)

const (
	replacementInterval = 3 * time.Minute

	reconnectTimeout = time.Minute

	minBackoff = 50 * time.Millisecond
	maxBackoff = 10 * time.Second
)

var resumablePreludeLine = regexp.MustCompile(`^` +
	regexp.QuoteMeta(serverProtocolStringV1) +
	` ([0-9A-Za-z+/]{` + strconv.Itoa(base64.RawStdEncoding.EncodedLen(ecdhP256UncompressedSize)) + `}) ` + `([0-9a-z\-]+)\r\n$`)

// readServerVersionExchange returns the ECDH public key and the host ID
// extracted from a resumption v1 server version line. A triplet of (nil, "",
// nil) is returned if a server version line is peeked and is not a resumption
// v1 line.
func readServerVersionExchange(conn *multiplexer.Conn) (dhPubKey *ecdh.PublicKey, hostID string, err error) {
	const maxVersionIdentifierSize = 255
	line, err := peekLine(conn, maxVersionIdentifierSize)
	if err != nil {
		return
	}

	match := resumablePreludeLine.FindSubmatch(line)
	if match == nil {
		return nil, "", nil
	}

	var buf [ecdhP256UncompressedSize]byte
	if n, err := base64.RawStdEncoding.Decode(buf[:], match[1]); err != nil {
		return nil, "", trace.Wrap(err)
	} else if n != ecdhP256UncompressedSize {
		return nil, "", trace.Wrap(io.ErrUnexpectedEOF, "short ECDH encoding")
	}

	dhPubKey, err = ecdh.P256().NewPublicKey(buf[:])
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	hostID = string(match[2])

	// discard is guaranteed to work for the line we just peeked
	_, _ = conn.Discard(len(line))

	return
}

// redialFunc should dial the given host; the connection is allowed to die with
// the passed context (to accommodate the Teleport gRPC transport).
type redialFunc = func(ctx context.Context, hostID string) (net.Conn, error)

// WrapSSHClientConn tries to detect if the server at the other end of nc is a
// resumption v1 server, and if so it returns a [net.Conn] that will
// transparently resume itself (using the provided redial func). If the
// connection is wrapped, the context applies to the lifetime of the returned
// connection, not just the duration of the function call.
func WrapSSHClientConn(ctx context.Context, nc net.Conn, redial redialFunc) (net.Conn, error) {
	return wrapSSHClientConn(ctx, nc, redial, clockwork.NewRealClock())
}

func wrapSSHClientConn(ctx context.Context, nc net.Conn, redial redialFunc, clock clockwork.Clock) (net.Conn, error) {
	dhKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		logrus.WithError(err).Error("Failed to generate ECDH key, proceeding without resumption (this is a bug).")
		return nc, nil
	}

	// adds a read buffer with the ability to peek to nc
	conn := ensureMultiplexerConn(nc)

	// We must send the first 8 bytes of the version string to go through some
	// older versions of the multiplexer that sits in front of the Teleport SSH
	// server; thankfully, no matter which SSH client we'll end up using, it
	// must send `SSH-2.0-` as its first 8 bytes, as per RFC 4253 ("The Secure
	// Shell (SSH) Transport Layer Protocol") section 4.2. Sending only the
	// first 8 bytes rather than a full version string is noncompliant behavior,
	// but our SSH client is only intended to talk to Teleport-implemented SSH
	// servers anyway, and other clients in the ecosystem do much worse
	// (ssh-keyscan will wait for the server to send data before sending
	// anything, for example).
	if _, err := conn.Write([]byte(sshPrefix)); err != nil {
		conn.Close()
		return nil, trace.Wrap(err)
	}

	dhPub, hostID, err := readServerVersionExchange(conn)
	if err != nil {
		conn.Close()
		return nil, trace.Wrap(err)
	}

	if dhPub == nil {
		// regular SSH connection, conn is about to read the SSH- line from the
		// server but we've sent sshPrefix already, so we have to skip it from
		// the application side writes
		logrus.Debug("Server does not support resumption, proceeding without.")
		return &sshVersionSkipConn{
			Conn:           conn,
			alreadyWritten: sshPrefix,
		}, nil
	}

	dhSecret, err := dhKey.ECDH(dhPub)
	if err != nil {
		logrus.WithError(err).Warn("Failed to complete ECDH key exchange, proceeding without resumption.")
		return &sshVersionSkipConn{
			Conn:           conn,
			alreadyWritten: sshPrefix,
		}, nil
	}

	otp32 := sha256.Sum256(dhSecret)
	token := resumptionToken(otp32[:16])

	if _, err := conn.Write([]byte(clientSuffixV1)); err != nil {
		conn.Close()
		return nil, trace.Wrap(err)
	}
	if _, err := conn.Write(dhKey.PublicKey().Bytes()); err != nil {
		conn.Close()
		return nil, trace.Wrap(err)
	}
	if _, err := conn.Write([]byte{newConnClientExchangeTag}); err != nil {
		conn.Close()
		return nil, trace.Wrap(err)
	}

	resumableConn := newResumableConn(conn.LocalAddr(), conn.RemoteAddr())
	// runClientResumable expects a brand new, locked *Conn
	resumableConn.mu.Lock()
	go runClientResumableUnlocking(ctx, resumableConn, conn, token, hostID, redial, clock)

	return resumableConn, nil
}

// runClientResumableUnlocking expects firstConn to be ready to be passed to
// runResumeV1Unlocking, and will drive resumableConn until the connection is
// impossible to resume further or connCtx is done.
func runClientResumableUnlocking(ctx context.Context, resumableConn *Conn, firstConn net.Conn, token resumptionToken, hostID string, redial redialFunc, clock clockwork.Clock) {
	defer resumableConn.Close()

	// detached is held open by the current underlying connection
	const isFirstConn = true
	detached := goAttachResumableUnlocking(resumableConn, firstConn, isFirstConn)

	reconnectTicker := clock.NewTicker(replacementInterval)
	defer reconnectTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-reconnectTicker.Chan():
			logrus.Debug("Attempting periodic reconnection.")

			newConn, err := dialResumable(ctx, token, hostID, redial)
			if err != nil {
				logrus.Warnf("Periodic reconnection: %v.", err.Error())
				continue
			}

			if newConn == nil {
				logrus.Warn("Impossible to resume connection, giving up on periodic reconnection.")
				reconnectTicker.Stop()
				select {
				case <-ctx.Done():
				case <-detached:
				}
				return
			}

			resumableConn.mu.Lock()
			const isNotFirstConn = false
			detached = goAttachResumableUnlocking(resumableConn, newConn, isNotFirstConn)

			continue

		case <-detached:
		}

		logrus.Debug("Connection lost, starting reconnection loop.")
		reconnectDeadline := time.Now().Add(reconnectTimeout)
		backoff := minBackoff
		for {
			resumableConn.mu.Lock()
			if resumableConn.localClosed {
				resumableConn.mu.Unlock()
				return
			}
			resumableConn.mu.Unlock()

			if time.Now().After(reconnectDeadline) {
				logrus.Error("Failed to reconnect to server after timeout.")
				return
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}

			backoff = min(maxBackoff, backoff*2)

			newConn, err := dialResumable(ctx, token, hostID, redial)
			if err != nil {
				logrus.Warnf("Reconnection attempt: %v.", err.Error())
				continue
			}

			if newConn == nil {
				logrus.Error("Impossible to resume connection.")
				return
			}

			resumableConn.mu.Lock()
			const isNotFirstConn = false
			detached = goAttachResumableUnlocking(resumableConn, newConn, isNotFirstConn)

			break
		}

		reconnectTicker.Reset(replacementInterval)
		select {
		case <-reconnectTicker.Chan():
		default:
		}
	}
}

// goAttachResumableUnlocking runs the resumable protocol over nc in a
// background goroutine, with some client-friendly logging, returning a channel
// that gets closed at the end of the goroutine. resumableConn is expected to be
// locked, like runResumeV1Unlocking.
func goAttachResumableUnlocking(resumableConn *Conn, nc net.Conn, firstConn bool) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)

		if firstConn {
			logrus.Debug("Attaching new resumable connection.")
		} else {
			logrus.Debug("Attaching existing resumable connection.")
		}

		err := runResumeV1Unlocking(resumableConn, nc, firstConn)

		if firstConn {
			logrus.Debugf("Handling new resumable connection: %v", err.Error())
		} else {
			logrus.Debugf("Handling existing resumable connection: %v", err.Error())
		}
	}()
	return done
}

// dialResumable attempts to resume a connection with a given token. A return
// value of nil, nil represents an impossibility to resume due to network
// conditions (or bugs). The returned connection is allowed to not outlive the
// context.
func dialResumable(ctx context.Context, token resumptionToken, hostID string, redial redialFunc) (*multiplexer.Conn, error) {
	dhKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	logrus.Debug("Dialing server for connection resumption.")
	nc, err := redial(ctx, hostID)
	if err != nil {
		// If connections are failing because client certificates are expired
		// abandon all future connection resumption attempts.
		const expiredCertError = "remote error: tls: expired certificate"
		if strings.Contains(err.Error(), expiredCertError) {
			return nil, nil
		}

		return nil, trace.Wrap(err)
	}

	if _, err := nc.Write([]byte(clientPreludeV1)); err != nil {
		nc.Close()
		return nil, trace.Wrap(err)
	}
	if _, err := nc.Write(dhKey.PublicKey().Bytes()); err != nil {
		nc.Close()
		return nil, trace.Wrap(err)
	}
	if _, err := nc.Write([]byte{existingConnClientExchangeTag}); err != nil {
		nc.Close()
		return nil, trace.Wrap(err)
	}

	conn := ensureMultiplexerConn(nc)

	dhPub, _, err := readServerVersionExchange(conn)
	if err != nil {
		conn.Close()
		return nil, trace.Wrap(err)
	}

	if dhPub == nil {
		conn.Close()
		logrus.Error("Reached a server without resumption support, giving up.")
		return nil, nil
	}

	dhSecret, err := dhKey.ECDH(dhPub)
	if err != nil {
		conn.Close()
		return nil, trace.Wrap(err)
	}

	otp32 := sha256.Sum256(dhSecret)

	for i := 0; i < 16; i++ {
		otp32[i] ^= token[i]
	}

	if _, err := conn.Write(otp32[:16]); err != nil {
		conn.Close()
		return nil, trace.Wrap(err)
	}

	responseTag, err := conn.ReadByte()
	if err != nil {
		conn.Close()
		return nil, trace.Wrap(err)
	}

	// success case
	if responseTag == successServerExchangeTag {
		return conn, nil
	}

	// all other tags are failure cases
	_ = conn.Close()
	switch responseTag {
	case notFoundServerExchangeTag:
		logrus.Error("Server responded with 'resumable connection not found', giving up.")
	case badAddressServerExchangeTag:
		logrus.Error("Server responded with 'bad client IP address', giving up.")
	default:
		logrus.Errorf("Server responded with an unknown error tag (%v), giving up.", responseTag)
	}

	return nil, nil
}
