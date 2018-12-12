/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package srv

import (
	"context"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"

	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
)

// RequestSender is an interface that impliments SendRequest. It is used so
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

	log := logrus.WithFields(logrus.Fields{
		trace.Component: teleport.ComponentKeepAlive,
	})
	log.Debugf("Starting keep-alive loop with with interval %v and max count %v.", p.Interval, p.MaxCount)

	tickerCh := time.NewTicker(p.Interval)
	defer tickerCh.Stop()

	for {
		select {
		case <-tickerCh.C:
			var sentCount int

			// Send a keep alive message on all connections and make sure a response
			// was received on all.
			for _, conn := range p.Conns {
				ok := sendKeepAliveWithTimeout(conn, defaults.ReadHeadersTimeout, p.CloseContext)
				if ok {
					sentCount += 1
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
				log.Infof("Missed %v keep-alive messages, closing connection.", missedCount)
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
func sendKeepAliveWithTimeout(conn RequestSender, timeout time.Duration, closeContext context.Context) bool {
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
