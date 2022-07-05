// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package agent

import (
	"context"
	"errors"

	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
)

// RequestAgentForwarding sets up agent forwarding for the session.
// ForwardToAgent or ForwardToRemote should be called to route
// the authentication requests.
//
// This is a forked version of golang.org/x/crypto/ssh/agent
// that wraps payloads sent across the underlying session in an
// Envelope, which allows us to provide tracing context to
// the server processing forwarding requests.
func RequestAgentForwarding(ctx context.Context, session *tracessh.Session) error {
	ok, err := session.SendRequest(ctx, "auth-agent-req@openssh.com", true, nil)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("forwarding request denied")
	}
	return nil
}
