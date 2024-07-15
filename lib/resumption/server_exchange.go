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
			r.log.WithError(err).Error("Error while reading resumption handshake.")
		}
		return
	}

	dhPub, err := ecdh.P256().NewPublicKey(dhBuf[:])
	if err != nil {
		r.log.WithError(err).Error("Received invalid ECDH key.")
		return
	}

	dhSecret, err := dhKey.ECDH(dhPub)
	if err != nil {
		r.log.WithError(err).Error("Failed ECDH exchange.")
		return
	}

	otp32 := sha256.Sum256(dhSecret)

	tag, err := conn.ReadByte()
	if err != nil {
		if !utils.IsOKNetworkError(err) {
			r.log.WithError(err).Error("Error while reading resumption handshake.")
		}
		return
	}

	switch tag {
	default:
		r.log.Error("Unknown tag in handshake: %x.", tag)
		return
	case newConnClientExchangeTag:
		r.log.Info("Handling new resumable SSH connection.")

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
				r.log.WithError(err).Warn("Unable to create handover listener for resumable connection, connection resumption will not work across graceful restarts.")
			}
		} else {
			r.log.Warn("Refusing to track resumable connection with an invalid remote IP address, connection resumption will not work (this is a bug).")
		}

		go func() {
			defer r.log.Info("Resumable connection completed.")
			defer resumableConn.Close()
			defer handoverCancel()
			defer func() {
				r.mu.Lock()
				defer r.mu.Unlock()
				delete(r.conns, token)
			}()
			defer entry.increaseRunning() // stop grace timeouts
			r.log.Info("Handing resumable connection to the SSH server.")
			r.sshServer(resumableConn)
		}()

		entry.increaseRunning()
		defer entry.decreaseRunning()
		const firstConn = true
		if err := runResumeV1Unlocking(resumableConn, conn, firstConn); utils.IsOKNetworkError(err) {
			r.log.Debugf("Handling new resumable connection: %v", err.Error())
		} else {
			r.log.Warnf("Handling new resumable connection: %v", err.Error())
		}
		return
	case existingConnClientExchangeTag:
	}

	var token resumptionToken
	if _, err := io.ReadFull(conn, token[:]); err != nil {
		if !utils.IsOKNetworkError(err) {
			r.log.WithError(err).Error("Error while reading resumption handshake.")
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
		r.log.Warn("Resumable connection attempted resumption from a different remote address.")
		_, _ = conn.Write([]byte{badAddressServerExchangeTag})
		return
	}

	if _, err := conn.Write([]byte{successServerExchangeTag}); err != nil {
		if !utils.IsOKNetworkError(err) {
			r.log.WithError(err).Error("Error while writing resumption handshake.")
		}
		return
	}

	entry.increaseRunning()
	defer entry.decreaseRunning()
	const notFirstConn = false
	entry.conn.mu.Lock()
	if err := runResumeV1Unlocking(entry.conn, conn, notFirstConn); utils.IsOKNetworkError(err) {
		r.log.Debugf("Handling existing resumable connection: %v", err.Error())
	} else {
		r.log.Warnf("Handling existing resumable connection: %v", err.Error())
	}
}
