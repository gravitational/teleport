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
	"io"
	"net"
	"net/netip"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/utils"
)

func (r *SSHServerWrapper) attemptHandover(conn *multiplexer.Conn, token resumptionToken) {
	handoverConn, err := r.dialHandover(token)
	if err != nil {
		if trace.IsNotFound(err) {
			r.log.Debug("Resumable connection not found or already deleted.")
			_, _ = conn.Write([]byte{notFoundServerExchangeTag})
			return
		}
		r.log.WithError(err).Error("Error while connecting to handover socket.")
		return
	}
	defer handoverConn.Close()

	var remoteIP netip.Addr
	if t, _ := conn.RemoteAddr().(*net.TCPAddr); t != nil {
		remoteIP, _ = netip.AddrFromSlice(t.IP)
	}
	remoteIP16 := remoteIP.As16()

	if _, err := handoverConn.Write(remoteIP16[:]); err != nil {
		if !utils.IsOKNetworkError(err) {
			r.log.WithError(err).Error("Error while forwarding remote address to handover socket.")
		}
		return
	}

	r.log.Debug("Forwarding resuming connection to handover socket.")
	_ = utils.ProxyConn(context.Background(), conn, handoverConn)
}

func (r *SSHServerWrapper) startHandoverListener(ctx context.Context, token resumptionToken, entry *connEntry) error {
	l, err := r.createHandoverListener(token)
	if err != nil {
		return trace.Wrap(err)
	}

	go r.runHandoverListener(l, entry)
	context.AfterFunc(ctx, func() { _ = l.Close() })

	return nil
}

func (r *SSHServerWrapper) runHandoverListener(l net.Listener, entry *connEntry) {
	defer l.Close()

	var tempDelay time.Duration
	for {
		// the logic for this Accept loop is copied from [net/http.Server]
		c, err := l.Accept()
		if err == nil {
			tempDelay = 0
			go r.handleHandoverConnection(c, entry)
			continue
		}

		if tempErr, ok := err.(interface{ Temporary() bool }); !ok || !tempErr.Temporary() {
			if !utils.IsOKNetworkError(err) {
				r.log.WithError(err).Warn("Accept error in handover listener.")
			}
			return
		}

		tempDelay = max(5*time.Millisecond, min(2*tempDelay, time.Second))
		r.log.WithError(err).WithField("delay", tempDelay).Warn("Temporary accept error in handover listener, continuing after delay.")
		time.Sleep(tempDelay)
	}
}

func (r *SSHServerWrapper) handleHandoverConnection(conn net.Conn, entry *connEntry) {
	defer conn.Close()

	var remoteIP16 [16]byte
	if _, err := io.ReadFull(conn, remoteIP16[:]); err != nil {
		if !utils.IsOKNetworkError(err) {
			r.log.WithError(err).Error("Error while reading remote address from handover socket.")
		}
		return
	}
	remoteIP := netip.AddrFrom16(remoteIP16).Unmap()

	r.resumeConnection(entry, conn, remoteIP)
}
