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
	"errors"
	"fmt"
	"net"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/observability/tracing"
)

// Client is a wrapper around ssh.Client that adds tracing support.
type Client struct {
	*ssh.Client
	opts             []tracing.Option
	tracingSupported bool
	rejectedError    error
}

// NewClient creates a new Client.
//
// The server being connected to is probed to determine if it supports
// ssh tracing. This is done by attempting to open a TracingChannel channel.
// If the channel is successfully opened then all payloads delivered to the
// server will be wrapped in an Envelope with tracing context. All Session
// and Channel created from the returned Client will honor the clients view
// of whether they should provide tracing context.
//
// Note: a channel is used instead of a global request in order prevent blocking
// forever in the event that the connection is rejected. In that case, the server
// doesn't service any global requests and writes the error to the first opened
// channel.
func NewClient(c ssh.Conn, chans <-chan ssh.NewChannel, reqs <-chan *ssh.Request, opts ...tracing.Option) *Client {
	clt := &Client{
		Client: ssh.NewClient(c, chans, reqs),
		opts:   opts,
	}

	// Check if the server supports tracing
	ch, _, err := clt.Client.OpenChannel(TracingChannel, nil)
	if err != nil {
		var openError *ssh.OpenChannelError
		if errors.As(err, &openError) {
			switch openError.Reason {
			case ssh.Prohibited:
				// prohibited errors due to locks and session control are expected by callers of NewSession
				clt.rejectedError = err
			default:
			}

			return clt
		}

		return clt
	}

	_ = ch.Close()
	clt.tracingSupported = true

	return clt
}

// DialContext initiates a connection to the addr from the remote host.
// The resulting connection has a zero LocalAddr() and RemoteAddr().
func (c *Client) DialContext(ctx context.Context, n, addr string) (net.Conn, error) {
	tracer := tracing.NewConfig(c.opts).TracerProvider.Tracer(instrumentationName)

	_, span := tracer.Start(
		ctx,
		"ssh.DialContext",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			append(
				peerAttr(c.Conn.RemoteAddr()),
				attribute.String("network", n),
				attribute.String("address", addr),
				semconv.RPCServiceKey.String("ssh.Client"),
				semconv.RPCMethodKey.String("Dial"),
				semconv.RPCSystemKey.String("ssh"),
			)...,
		),
	)
	defer span.End()

	conn, err := c.Client.Dial(n, addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conn, nil
}

// SendRequest sends a global request, and returns the
// reply. If tracing is enabled, the provided payload
// is wrapped in an Envelope to forward any tracing context.
func (c *Client) SendRequest(ctx context.Context, name string, wantReply bool, payload []byte) (bool, []byte, error) {
	config := tracing.NewConfig(c.opts)
	tracer := config.TracerProvider.Tracer(instrumentationName)

	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("ssh.GlobalRequest/%s", name),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			append(
				peerAttr(c.Conn.RemoteAddr()),
				attribute.Bool("want_reply", wantReply),
				semconv.RPCServiceKey.String("ssh.Client"),
				semconv.RPCMethodKey.String("SendRequest"),
				semconv.RPCSystemKey.String("ssh"),
			)...,
		),
	)
	defer span.End()

	ok, resp, err := c.Client.SendRequest(name, wantReply, wrapPayload(ctx, c.tracingSupported, config.TextMapPropagator, payload))
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}

	return ok, resp, err
}

// OpenChannel tries to open a channel. If tracing is enabled,
// the provided payload is wrapped in an Envelope to forward
// any tracing context.
func (c *Client) OpenChannel(ctx context.Context, name string, data []byte) (*Channel, <-chan *ssh.Request, error) {
	config := tracing.NewConfig(c.opts)
	tracer := config.TracerProvider.Tracer(instrumentationName)
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("ssh.OpenChannel/%s", name),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			append(
				peerAttr(c.Conn.RemoteAddr()),
				semconv.RPCServiceKey.String("ssh.Client"),
				semconv.RPCMethodKey.String("OpenChannel"),
				semconv.RPCSystemKey.String("ssh"),
			)...,
		),
	)
	defer span.End()

	ch, reqs, err := c.Client.OpenChannel(name, wrapPayload(ctx, c.tracingSupported, config.TextMapPropagator, data))
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}

	return &Channel{
		Channel: ch,
		opts:    c.opts,
	}, reqs, err
}

// NewSession creates a new SSH session that is passed tracing context
// so that spans may be correlated properly over the ssh connection.
func (c *Client) NewSession(ctx context.Context) (*ssh.Session, error) {
	tracer := tracing.NewConfig(c.opts).TracerProvider.Tracer(instrumentationName)

	_, span := tracer.Start(
		ctx,
		"ssh.NewSession",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			append(
				peerAttr(c.Conn.RemoteAddr()),
				semconv.RPCServiceKey.String("ssh.Client"),
				semconv.RPCMethodKey.String("NewSession"),
				semconv.RPCSystemKey.String("ssh"),
			)...,
		),
	)
	defer span.End()

	// If the TracingChannel was rejected when the client was created,
	// the connection was prohibited due to a lock or session control.
	// Callers to NewSession are expecting to receive the reason the session
	// was rejected, so we need to propagate the rejectedError here.
	if c.rejectedError != nil {
		return nil, trace.Wrap(c.rejectedError)
	}

	session, err := c.Client.NewSession()

	return session, trace.Wrap(err)
}
