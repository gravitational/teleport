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
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestServerClose(t *testing.T) {
	t.Parallel()

	doneCh := make(chan bool, 1)
	closeContext, closeCancel := context.WithCancel(context.Background())

	// Create a request sender that always replies to keep-alive requests.
	requestSender := &testRequestSender{
		reply: true,
	}

	go func() {
		StartKeepAliveLoop(KeepAliveParams{
			Conns: []RequestSender{
				requestSender,
			},
			Interval:     10 * time.Millisecond,
			MaxCount:     2,
			CloseContext: closeContext,
			CloseCancel:  closeCancel,
		})
		doneCh <- true
	}()

	// Wait for a keep-alive to be sent.
	err := waitForRequests(requestSender, 1)
	require.NoError(t, err)

	// Close the context (server), should cause the loop to stop as well.
	closeCancel()

	// Wait 1 second for the keep-alive loop to stop, or return an error.
	select {
	case <-time.After(1 * time.Second):
		t.Fatalf("Timeout waiting for keep-alive loop to stop.")
	case <-doneCh:
	}
}

func TestLoopClose(t *testing.T) {
	t.Parallel()

	doneCh := make(chan bool, 1)
	closeContext, closeCancel := context.WithCancel(context.Background())

	// Create a request sender that never replies to keep-alive requests.
	requestSender := &testRequestSender{
		reply: false,
	}

	go func() {
		StartKeepAliveLoop(KeepAliveParams{
			Conns: []RequestSender{
				requestSender,
			},
			Interval:     10 * time.Millisecond,
			MaxCount:     2,
			CloseContext: closeContext,
			CloseCancel:  closeCancel,
		})
		doneCh <- true
	}()

	// Wait for a keep-alive to be sent.
	err := waitForRequests(requestSender, 1)
	require.NoError(t, err)

	// Wait 1 second for the keep-alive loop to stop, or return an error.
	select {
	case <-time.After(1 * time.Second):
		t.Fatalf("Timeout waiting for keep-alive loop to stop.")
	case <-doneCh:
	}
}

func waitForRequests(requestSender *testRequestSender, count int) error {
	tickerCh := time.NewTicker(50 * time.Millisecond)
	defer tickerCh.Stop()
	timeoutCh := time.NewTimer(1 * time.Second)
	defer timeoutCh.Stop()

	for {
		select {
		case <-tickerCh.C:
			if requestSender.Count() > count {
				return nil
			}
		case <-timeoutCh.C:
			return trace.BadParameter("timeout waiting for requests")
		}
	}
}

type testRequestSender struct {
	count int64 // intentionally placed first to ensure 64-bit alignment
	reply bool
}

func (n *testRequestSender) SendRequest(name string, wantReply bool, payload []byte) (bool, []byte, error) {
	atomic.AddInt64(&n.count, 1)
	if n.reply == false {
		return false, nil, trace.BadParameter("no reply")
	}
	return false, nil, nil
}

func (n *testRequestSender) Count() int {
	return int(atomic.LoadInt64(&n.count))

}
