// Copyright 2022 Gravitational, Inc
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

package sshutils

import (
	"context"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// RequestForwarder represents a resource capable of sending
// an ssh request such as an ssh.Channel or ssh.Session.
type RequestForwarder interface {
	SendRequest(name string, wantReply bool, payload []byte) (bool, error)
}

// ForwardRequest is a helper for forwarding a request across a session or channel.
func ForwardRequest(sender RequestForwarder, req *ssh.Request) (bool, error) {
	reply, err := sender.SendRequest(req.Type, req.WantReply, req.Payload)
	if err != nil || !req.WantReply {
		return reply, trace.Wrap(err)
	}
	return reply, trace.Wrap(req.Reply(reply, nil))
}

// ForwardRequests forwards all ssh requests received from the
// given channel until the channel or context is closed.
func ForwardRequests(ctx context.Context, sin <-chan *ssh.Request, sender RequestForwarder) error {
	for {
		select {
		case sreq, ok := <-sin:
			if !ok {
				// channel closed, stop processing
				return nil
			}
			switch sreq.Type {
			case WindowChangeRequest:
				if _, err := ForwardRequest(sender, sreq); err != nil {
					return trace.Wrap(err)
				}
			default:
				if sreq.WantReply {
					sreq.Reply(false, nil)
				}
				continue
			}
		case <-ctx.Done():
			if ctx.Err() != context.Canceled {
				return trace.Wrap(ctx.Err())
			}
			return nil
		}
	}
}
