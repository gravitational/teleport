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
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"

	"gopkg.in/check.v1"
)

type KeepAliveSuite struct{}

var _ = check.Suite(&KeepAliveSuite{})

func TestSrv(t *testing.T) { check.TestingT(t) }

func (s *KeepAliveSuite) TestServerClose(c *check.C) {
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
	c.Assert(err, check.IsNil)

	// Close the context (server), should cause the loop to stop as well.
	closeCancel()

	// Wait 1 second for the keep-alive loop to stop, or return an error.
	select {
	case <-time.After(1 * time.Second):
		c.Fatalf("Timeout waiting for keep-alive loop to stop.")
	case <-doneCh:
	}
}

func (s *KeepAliveSuite) TestLoopClose(c *check.C) {
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
	c.Assert(err, check.IsNil)

	// Wait 1 second for the keep-alive loop to stop, or return an error.
	select {
	case <-time.After(1 * time.Second):
		c.Fatalf("Timeout waiting for keep-alive loop to stop.")
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
	reply bool
	count int64
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
