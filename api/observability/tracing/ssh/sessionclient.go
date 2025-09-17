// Copyright 2025 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ssh

import (
	"sync"
	"sync/atomic"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// SessionClient is an alternative [ssh.Client] used to create new sessions
// beyond the limitations of [ssh.Client.NewSession]. Specifically, this custom
// client enables the caller to:
//   - Handle incoming session level requests (e.g. "current-session-id@goteleport.com")
//     which are wholly ignored by the default client.
//   - Provide custom extra data to the "session" channel request - TODO(Joerger): follow up PR adding this
//
// If the upstream golang.org/x/crypto/ssh is updated to provide this functionality directly,
// custom client can be fully removed. This custom client is unrelated to tracing functionality.
type SessionClient struct {
	mu              sync.Mutex
	requestHandlers map[string]chan *ssh.Request
}

// NewSessionClient returns a new SessionClient.
func NewSessionClient() *SessionClient {
	return &SessionClient{
		requestHandlers: map[string]chan *ssh.Request{},
	}
}

// NewSession opens a new Session for this client.
func (c *SessionClient) NewSession(conn ssh.Conn) (*ssh.Session, error) {
	// open a session manually so we can take ownership of the
	// requests chan
	ch, reqs, err := conn.OpenChannel("session", nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Capture requests not handled by registered request handlers and
	// pass them to the crypto [ssh.Session].
	unhandledReqs := make(chan *ssh.Request, cap(reqs))
	go func() {
		c.handleRequests(reqs, unhandledReqs)
		close(unhandledReqs)
	}()

	session, err := newCryptoSSHSession(ch, unhandledReqs)
	if err != nil {
		_ = ch.Close()
		return nil, trace.Wrap(err)
	}

	return session, nil
}

// HandleRequest returns a channel on which ssh Requests for the given
// type are sent. If the type already is being handled, nil is returned.
// The channel is closed when the connection is closed.
//
// This should be called before NewSession to ensure requests of this type are
// not processed before the handler is registered.
//
// This method was adapted from golang/x/crypto/ssh.Client.HandleChannelOpen.
// golang/x/crypto/ssh does not currently provide a similar method for session
// requests out of the box.
func (c *SessionClient) HandleRequests(requestType string) <-chan *ssh.Request {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.requestHandlers == nil {
		// The SSH channel has been closed.
		c := make(chan *ssh.Request)
		close(c)
		return c
	}

	ch := c.requestHandlers[requestType]
	if ch != nil {
		return nil
	}

	// This is the same buffer size used in golang/x/crypto/ssh for the channel requests
	// channel serviced by registered handlers from [ssh.Client.HandleChannelOpen].
	const bufferSize = 16

	ch = make(chan *ssh.Request, bufferSize)
	c.requestHandlers[requestType] = ch
	return ch
}

// handleRequests from the remote side.
func (c *SessionClient) handleRequests(in <-chan *ssh.Request, unhandledReqs chan<- *ssh.Request) {
	for req := range in {
		c.mu.Lock()
		handler := c.requestHandlers[req.Type]
		c.mu.Unlock()

		if handler != nil {
			handler <- req
		} else {
			// Pass on requests without a registered handler. These will be
			// handled by the default x/crypto/ssh request handler.
			unhandledReqs <- req
		}
	}

	c.mu.Lock()
	for _, ch := range c.requestHandlers {
		close(ch)
	}
	c.requestHandlers = nil
	c.mu.Unlock()
}

// sshSession allows an SSH session to be created while also allowing
// callers to take ownership of the SSH channel requests chan.
type sshSession struct {
	ssh.Conn

	channelOpened atomic.Bool

	ch   ssh.Channel
	reqs <-chan *ssh.Request
}

func (f *sshSession) OpenChannel(_ string, _ []byte) (ssh.Channel, <-chan *ssh.Request, error) {
	if !f.channelOpened.CompareAndSwap(false, true) {
		panic("WrappedSSHConn.OpenChannel called more than once")
	}

	return f.ch, f.reqs, nil
}

// newCryptoSSHSession allows callers to take ownership of the SSH
// channel requests chan and allow callers to handle SSH channel requests.
// golang.org/x/crypto/ssh.(Client).NewSession takes ownership of all
// SSH channel requests and doesn't allow the caller to view or reply
// to them, so this workaround is needed.
func newCryptoSSHSession(ch ssh.Channel, reqs <-chan *ssh.Request) (*ssh.Session, error) {
	return (&ssh.Client{
		Conn: &sshSession{
			ch:   ch,
			reqs: reqs,
		},
	}).NewSession()
}
