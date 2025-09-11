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
	"sync/atomic"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// ChannelRequestCallback allows the handling of channel requests
// to be customized. nil can be returned if you don't want
// golang/x/crypto/ssh to handle the request.
type ChannelRequestCallback func(req *ssh.Request) *ssh.Request

// NewSession opens a new Session for this client.
func NewSession(client *ssh.Client, callback ChannelRequestCallback) (*ssh.Session, error) {
	// No custom request handling needed. We can use the basic golang/x/crypto/ssh implementation.
	if callback == nil {
		session, err := client.NewSession()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return session, nil
	}

	// open a session manually so we can take ownership of the
	// requests chan
	ch, originalReqs, openChannelErr := client.OpenChannel("session", nil)
	if openChannelErr != nil {
		return nil, trace.Wrap(openChannelErr)
	}

	handleReqs := originalReqs
	if callback != nil {
		reqs := make(chan *ssh.Request, cap(originalReqs))
		handleReqs = reqs

		// pass the channel requests to the provided callback and
		// forward them to another chan so golang.org/x/crypto/ssh
		// can handle Session exiting correctly
		go func() {
			defer close(reqs)

			for req := range originalReqs {
				if req := callback(req); req != nil {
					reqs <- req
				}
			}
		}()
	}

	session, err := newCryptoSSHSession(ch, handleReqs)
	if err != nil {
		_ = ch.Close()
		return nil, trace.Wrap(err)
	}

	return session, nil
}

// wrappedSSHConn allows an SSH session to be created while also allowing
// callers to take ownership of the SSH channel requests chan.
type wrappedSSHConn struct {
	ssh.Conn

	channelOpened atomic.Bool

	ch   ssh.Channel
	reqs <-chan *ssh.Request
}

func (f *wrappedSSHConn) OpenChannel(_ string, _ []byte) (ssh.Channel, <-chan *ssh.Request, error) {
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
		Conn: &wrappedSSHConn{
			ch:   ch,
			reqs: reqs,
		},
	}).NewSession()
}
