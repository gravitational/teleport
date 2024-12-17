/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package srv

import (
	"context"
	"log/slog"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
)

// RequestSender is an interface that implements SendRequest. It is used so
// server and client connections can be passed to functions to send requests.
type RequestSender interface {
	// SendRequest is used to send a out-of-band request.
	SendRequest(name string, wantReply bool, payload []byte) (bool, []byte, error)
}

// KeepAliveParams configures the keep-alive loop.
type KeepAliveParams struct {
	// Conns is the list of connections to send keep-alive connections to. All
	// connections must respond to the keep-alive to not be considered missed.
	Conns []RequestSender

	// Interval is the interval to send keep-alive messsages at.
	Interval time.Duration

	// MaxCount is the number of keep-alive messages that can be missed before
	// the connection is disconnected.
	MaxCount int64

	// CloseContext is used by the server to notify the keep-alive loop to stop.
	CloseContext context.Context

	// CloseCancel is used by the keep-alive loop to notify the server to stop.
	CloseCancel context.CancelFunc
}

// StartKeepAliveLoop starts the keep-alive loop.
func StartKeepAliveLoop(p KeepAliveParams) {
	var missedCount int64

	log := slog.With(teleport.ComponentKey, teleport.ComponentKeepAlive)
	log.DebugContext(p.CloseContext, "Starting keep-alive loop", "interval", p.Interval, "max_count", p.MaxCount)

	tickerCh := time.NewTicker(p.Interval)
	defer tickerCh.Stop()

	for {
		select {
		case <-tickerCh.C:
			var sentCount int

			// Send a keep alive message on all connections and make sure a response
			// was received on all.
			for _, conn := range p.Conns {
				ok := sendKeepAliveWithTimeout(p.CloseContext, conn, defaults.ReadHeadersTimeout)
				if ok {
					sentCount++
				}
			}
			if sentCount == len(p.Conns) {
				missedCount = 0
				continue
			}

			// If enough keep-alives are missed, the connection is dead, call cancel
			// and notify the server to disconnect and cleanup.
			missedCount = missedCount + 1
			if missedCount > p.MaxCount {
				log.InfoContext(p.CloseContext, "Missed too keep-alive messages, closing connection", "missed_count", missedCount)
				p.CloseCancel()
				return
			}
		// If an external caller closed the context (connection is done) then no
		// more need to wait around for keep-alives.
		case <-p.CloseContext.Done():
			return
		}
	}
}

// sendKeepAliveWithTimeout sends a keepalive@openssh.com message to the remote
// client. A manual timeout is needed here because SendRequest will wait for a
// response forever.
func sendKeepAliveWithTimeout(closeContext context.Context, conn RequestSender, timeout time.Duration) bool {
	errorCh := make(chan error, 1)

	go func() {
		// SendRequest will unblock when connection or channel is closed.
		_, _, err := conn.SendRequest(teleport.KeepAliveReqType, true, nil)
		errorCh <- err
	}()

	select {
	case err := <-errorCh:
		if err != nil {
			return false
		}
		return true
	case <-time.After(timeout):
		return false
	case <-closeContext.Done():
		return false
	}
}
