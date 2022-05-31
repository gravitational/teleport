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

package ssh

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/utils/sshutils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"
)

const (
	// TracingRequest is sent by clients to server to pass along tracing context.
	TracingRequest = "tracing@goteleport.com"
)

// Client is a wrapper around ssh.Client that adds tracing support.
type Client struct {
	*ssh.Client
}

// NewClient creates a new Client.
func NewClient(c ssh.Conn, chans <-chan ssh.NewChannel, reqs <-chan *ssh.Request) *Client {
	return &Client{Client: ssh.NewClient(c, chans, reqs)}
}

// NewSession creates a new SSH session that is passed tracing context so that spans may be correlated
// properly over the ssh connection.
func (c *Client) NewSession(ctx context.Context) (*ssh.Session, error) {
	session, err := c.Client.NewSession()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	span := oteltrace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return session, nil
	}

	traceCtx := tracing.PropagationContextFromContext(ctx)
	if len(traceCtx) == 0 {
		return session, nil
	}

	payload, err := json.Marshal(traceCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if _, err := session.SendRequest(TracingRequest, false, payload); err != nil {
		return nil, trace.Wrap(err)
	}

	return session, nil
}

// NewClientConn creates a new SSH client connection that is passed tracing context so that spans may be correlated
// properly over the ssh connection.
func NewClientConn(ctx context.Context, conn net.Conn, addr string, config *ssh.ClientConfig) (ssh.Conn, <-chan ssh.NewChannel, <-chan *ssh.Request, error) {
	hp := &sshutils.HandshakePayload{
		TracingContext: tracing.PropagationContextFromContext(ctx),
	}

	if len(hp.TracingContext) > 0 {
		payloadJSON, err := json.Marshal(hp)
		if err == nil {
			payload := fmt.Sprintf("%s%s\x00", sshutils.ProxyHelloSignature, payloadJSON)
			_, err = conn.Write([]byte(payload))
			if err != nil {
				log.WithError(err).Warnf("Failed to pass along tracing context to proxy %v", addr)
			}
		}
	}

	c, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	return c, chans, reqs, nil
}

// NewClientConnWithDeadline establishes new client connection with specified deadline
func NewClientConnWithDeadline(ctx context.Context, conn net.Conn, addr string, config *ssh.ClientConfig) (*Client, error) {
	if config.Timeout > 0 {
		if err := conn.SetReadDeadline(time.Now().Add(config.Timeout)); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	c, chans, reqs, err := NewClientConn(ctx, conn, addr, config)
	if err != nil {
		return nil, err
	}
	if config.Timeout > 0 {
		if err := conn.SetReadDeadline(time.Time{}); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return NewClient(c, chans, reqs), nil
}
