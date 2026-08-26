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
	"crypto/sha256"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"time"

	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	newConnClientExchangeTag      byte = 0
	existingConnClientExchangeTag byte = 1

	successServerExchangeTag  byte = 0
	notFoundServerExchangeTag byte = 1
	// badAddressServerExchangeTag signifies that the resumption attempt was
	// rejected because the client address was different from the expected one.
	// A sophisticated client could decide to start periodically reconnecting
	// with a long interval, or potentially only attempt to reconnect after the
	// local network conditions have changed (as maybe the client address will
	// be different then), but it's also reasonable to give up on resumption
	// if this is received.
	badAddressServerExchangeTag byte = 2
)

// handleResumptionExchangeV1 takes over after having sent the magic server
// version with the ECDH key in dhKey and right after receiving the v1 client
// prelude (but no other data).
func (r *SSHServerWrapper) handleResumptionExchangeV1(conn *multiplexer.Conn, dhKey *ecdh.PrivateKey) {
	defer conn.Close()

	const ecdhP256UncompressedSize = 65

	var dhBuf [ecdhP256UncompressedSize]byte
	if _, err := io.ReadFull(conn, dhBuf[:]); err != nil {
		if !utils.IsOKNetworkError(err) {
			slog.ErrorContext(context.TODO(), "error while reading resumption handshake", "error", err)
		}
		return
	}

	dhPub, err := ecdh.P256().NewPublicKey(dhBuf[:])
	if err != nil {
		slog.ErrorContext(context.TODO(), "received invalid ECDH key", "error", err)
		return
	}

	dhSecret, err := dhKey.ECDH(dhPub)
	if err != nil {
		slog.ErrorContext(context.TODO(), "failed ECDH exchange", "error", err)
		return
	}

	otp32 := sha256.Sum256(dhSecret)

	tag, err := conn.ReadByte()
	if err != nil {
		if !utils.IsOKNetworkError(err) {
			slog.ErrorContext(context.TODO(), "error while reading resumption handshake", "error", err)
		}
		return
	}

	switch tag {
	default:
		slog.ErrorContext(context.TODO(), "unknown tag in handshake", "tag", tag)
		return
	case newConnClientExchangeTag:
		slog.InfoContext(context.TODO(), "handling new resumable SSH connection")

		resumableConn := newResumableConn(conn.LocalAddr(), conn.RemoteAddr())
		// nothing must use the resumable conn until the firstConn handler is
		// marked as attached
		resumableConn.mu.Lock()

		var remoteIP netip.Addr
		if t, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
			remoteIP, _ = netip.AddrFromSlice(t.IP)
			remoteIP = remoteIP.Unmap()
		}

		token := resumptionToken(otp32[:16])
		entry := &connEntry{
			conn:     resumableConn,
			remoteIP: remoteIP,
			timeout:  time.AfterFunc(detachedTimeout, func() { resumableConn.Close() }),
		}

		// this context is only used for the convenience of [context.AfterFunc]
		handoverContext, handoverCancel := context.WithCancel(context.Background())
		if remoteIP.IsValid() {
			r.mu.Lock()
			r.conns[token] = entry
			r.mu.Unlock()

			if err := r.startHandoverListener(handoverContext, token, entry); err != nil {
				slog.WarnContext(context.TODO(), "unable to create handover listener for resumable connection, connection resumption will not work across graceful restarts", "error", err)
			}
		} else {
			slog.WarnContext(context.TODO(), "refusing to track resumable connection with an invalid remote IP address, connection resumption will not work (this is a bug)")
		}

		go func() {
			defer slog.InfoContext(context.TODO(), "resumable connection completed")
			defer resumableConn.Close()
			defer handoverCancel()
			defer func() {
				r.mu.Lock()
				defer r.mu.Unlock()
				delete(r.conns, token)
			}()
			defer entry.increaseRunning() // stop grace timeouts
			slog.InfoContext(context.TODO(), "handing resumable connection to the SSH server")
			r.sshServer(resumableConn)
		}()

		entry.increaseRunning()
		defer entry.decreaseRunning()
		const firstConn = true
		if err := runResumeV1Unlocking(resumableConn, conn, firstConn); utils.IsOKNetworkError(err) {
			slog.DebugContext(context.TODO(), "handling new resumable connection", "error", err)
		} else {
			slog.WarnContext(context.TODO(), "handling new resumable connection", "error", err)
		}
		return
	case existingConnClientExchangeTag:
	}

	var token resumptionToken
	if _, err := io.ReadFull(conn, token[:]); err != nil {
		if !utils.IsOKNetworkError(err) {
			slog.ErrorContext(context.TODO(), "error while reading resumption handshake", "error", err)
		}
		return
	}

	for i := range token {
		token[i] ^= otp32[i]
	}

	r.mu.Lock()
	entry := r.conns[token]
	r.mu.Unlock()

	if entry == nil {
		r.attemptHandover(conn, token)
		return
	}

	var remoteIP netip.Addr
	if t, _ := conn.RemoteAddr().(*net.TCPAddr); t != nil {
		remoteIP, _ = netip.AddrFromSlice(t.IP)
		remoteIP = remoteIP.Unmap()
	}

	r.resumeConnection(entry, conn, remoteIP)
}

func (r *SSHServerWrapper) resumeConnection(entry *connEntry, conn net.Conn, remoteIP netip.Addr) {
	if entry.remoteIP != remoteIP {
		slog.WarnContext(context.TODO(), "resumable connection attempted resumption from a different remote address")
		_, _ = conn.Write([]byte{badAddressServerExchangeTag})
		return
	}

	if _, err := conn.Write([]byte{successServerExchangeTag}); err != nil {
		if !utils.IsOKNetworkError(err) {
			slog.ErrorContext(context.TODO(), "error while writing resumption handshake", "error", err)
		}
		return
	}

	entry.increaseRunning()
	defer entry.decreaseRunning()
	const notFirstConn = false
	entry.conn.mu.Lock()
	if err := runResumeV1Unlocking(entry.conn, conn, notFirstConn); utils.IsOKNetworkError(err) {
		slog.DebugContext(context.TODO(), "handling existing resumable connection", "error", err)
	} else {
		slog.WarnContext(context.TODO(), "handling existing resumable connection", "error", err)
	}
}
