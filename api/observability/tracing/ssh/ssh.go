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
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/gravitational/teleport/api"
	"github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/utils/sshutils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"
)

const (
	// TracingChannel is a SSH channel used to indicate that servers support tracing.
	TracingChannel = "tracing"

	// instrumentationName is the name of this instrumentation package.
	instrumentationName = "otelssh"
)

// ContextFromRequest extracts any tracing data provided via an Envelope
// in the ssh.Request payload. If the payload contains an Envelope, then
// the context returned will have tracing data populated from the remote
// tracing context and the ssh.Request payload will be replaced with the
// original payload from the client.
func ContextFromRequest(req *ssh.Request) context.Context {
	ctx := context.Background()
	var envelope Envelope
	if err := json.Unmarshal(req.Payload, &envelope); err == nil {
		ctx = tracing.WithPropagationContext(ctx, envelope.PropagationContext)
		req.Payload = envelope.Payload
	}

	return ctx
}

// ContextFromNewChannel extracts any tracing data provided via an Envelope
// in the ssh.NewChannel ExtraData. If the ExtraData contains an Envelope, then
// the context returned will have tracing data populated from the remote
// tracing context and the ssh.NewChannel wrapped in a TraceCh so that the
// original ExtraData from the client is exposed instead of the Envelope
// payload.
func ContextFromNewChannel(nch ssh.NewChannel) (context.Context, ssh.NewChannel) {
	ch := NewTraceCh(nch)
	ctx := tracing.WithPropagationContext(context.Background(), ch.Envelope.PropagationContext)

	return ctx, ch
}

// Client is a wrapper around ssh.Client that adds tracing support.
type Client struct {
	*ssh.Client
	tracer           oteltrace.Tracer
	cfg              *tracing.Config
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
	cfg := tracing.NewConfig(opts)
	clt := &Client{
		Client: ssh.NewClient(c, chans, reqs),
		tracer: cfg.TracerProvider.Tracer(
			instrumentationName,
			oteltrace.WithInstrumentationVersion(api.Version),
		),
		cfg: cfg,
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
	_, span := c.tracer.Start(
		ctx,
		"ssh.Dial",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			append(
				peerAttr(c.Conn.RemoteAddr()),
				attribute.String("network", n),
				attribute.String("address", addr),
				semconv.RPCServiceKey.String("ssh.Client"),
				semconv.RPCMethodKey.String("SendRequest"),
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
	ctx, span := c.tracer.Start(
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

	ok, resp, err := c.Client.SendRequest(name, wantReply, wrapPayload(ctx, c.tracingSupported, c.cfg.TextMapPropagator, payload))
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
	ctx, span := c.tracer.Start(
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

	ch, reqs, err := c.Client.OpenChannel(name, wrapPayload(ctx, c.tracingSupported, c.cfg.TextMapPropagator, data))
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}

	return &Channel{
		Channel:          ch,
		tracingSupported: c.tracingSupported,
		tracer:           c.tracer,
		cfg:              c.cfg,
	}, reqs, err
}

// NewSession creates a new Session.
func (c *Client) NewSession(ctx context.Context) (*Session, error) {
	ctx, span := c.tracer.Start(
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

	ch, in, err := c.OpenChannel(ctx, "session", nil)
	if err != nil {
		// An EOF here means that the server closed our connection. In
		// the event that the rejectedError was populated when opening the
		// TracingChannel channel, the connection was prohibited due to a lock or
		// session control. Callers to NewSession are expecting to receive
		// the reason the session was rejected, so we need to propagate the
		// rejectedError here instead of returning the EOF.
		if errors.Is(err, io.EOF) && c.rejectedError != nil {
			return nil, trace.Wrap(c.rejectedError)
		}

		return nil, trace.Wrap(err)
	}

	return newSession(ch, in, c.tracingSupported, c.tracer)
}

// Channel is a wrapper around ssh.Channel that adds tracing support.
type Channel struct {
	ssh.Channel
	tracingSupported bool
	cfg              *tracing.Config
	tracer           oteltrace.Tracer
}

// NewChannel creates a new Channel.
func NewChannel(ch ssh.Channel, opts ...tracing.Option) *Channel {
	cfg := tracing.NewConfig(opts)

	return &Channel{
		Channel: ch,
		cfg:     cfg,
		tracer: cfg.TracerProvider.Tracer(
			instrumentationName,
			oteltrace.WithInstrumentationVersion(api.Version),
		),
	}
}

// SendRequest sends a global request, and returns the
// reply. If tracing is enabled, the provided payload
// is wrapped in an Envelope to forward any tracing context.
func (c *Channel) SendRequest(ctx context.Context, name string, wantReply bool, payload []byte) (bool, error) {
	ctx, span := c.tracer.Start(
		ctx,
		fmt.Sprintf("ssh.ChannelRequest/%s", name),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String("ssh.Channel"),
			semconv.RPCMethodKey.String("SendRequest"),
			semconv.RPCSystemKey.String("ssh"),
		),
	)
	defer span.End()

	ok, err := c.Channel.SendRequest(name, wantReply, wrapPayload(ctx, c.tracingSupported, c.cfg.TextMapPropagator, payload))
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}

	return ok, err
}

// ServerConn is a wrapper around ssh.ServerConn
// that adds tracing support.
type ServerConn struct {
	Options []tracing.Option
	*ssh.ServerConn
}

// NewServerConn creates a new ServerConn.
func NewServerConn(conn net.Conn, config *ssh.ServerConfig, opts ...tracing.Option) (*ServerConn, <-chan ssh.NewChannel, <-chan *ssh.Request, error) {
	sconn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	serverConn := &ServerConn{
		Options:    opts,
		ServerConn: sconn,
	}

	return serverConn, chans, reqs, nil
}

// SendRequest sends a global request, and returns the
// reply. If tracing is enabled, the provided payload
// is wrapped in an Envelope to forward any tracing context.
func (s *ServerConn) SendRequest(ctx context.Context, name string, wantReply bool, payload []byte) (bool, []byte, error) {
	cfg := tracing.NewConfig(s.Options)
	tracer := cfg.TracerProvider.Tracer(
		instrumentationName,
		oteltrace.WithInstrumentationVersion(api.Version),
	)

	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("ssh.SeverRequest/%s", name),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			append(
				peerAttr(s.Conn.RemoteAddr()),
				semconv.RPCServiceKey.String("ssh.ServerConn"),
				semconv.RPCMethodKey.String("SendRequest"),
				semconv.RPCSystemKey.String("ssh"),
			)...,
		),
	)
	defer span.End()

	ok, resp, err := s.Conn.SendRequest(name, wantReply, wrapPayload(ctx, true, cfg.TextMapPropagator, payload))
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}

	return ok, resp, err
}

// OpenChannel tries to open a channel. If tracing is enabled,
// the provided payload is wrapped in an Envelope to forward
// any tracing context.
func (s *ServerConn) OpenChannel(ctx context.Context, name string, data []byte) (*Channel, <-chan *ssh.Request, error) {
	cfg := tracing.NewConfig(s.Options)
	tracer := cfg.TracerProvider.Tracer(
		instrumentationName,
		oteltrace.WithInstrumentationVersion(api.Version),
	)

	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("ssh.OpenChannel/%s", name),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			append(
				peerAttr(s.Conn.RemoteAddr()),
				semconv.RPCServiceKey.String("ssh.ServerConn"),
				semconv.RPCMethodKey.String("OpenChannel"),
				semconv.RPCSystemKey.String("ssh"),
			)...,
		),
	)
	defer span.End()

	ch, reqs, err := s.Conn.OpenChannel(name, wrapPayload(ctx, true, cfg.TextMapPropagator, data))
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}

	return NewChannel(ch), reqs, err
}

// Dial starts a client connection to the given SSH server. It is a
// convenience function that connects to the given network address,
// initiates the SSH handshake, and then sets up a Client.  For access
// to incoming channels and requests, use net.Dial with NewClientConn
// instead.
func Dial(ctx context.Context, network, addr string, config *ssh.ClientConfig) (*Client, error) {
	conn, err := net.DialTimeout(network, addr, config.Timeout)
	if err != nil {
		return nil, err
	}
	c, chans, reqs, err := NewClientConn(ctx, conn, addr, config)
	if err != nil {
		return nil, err
	}
	return NewClient(c, chans, reqs), nil
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

// peerAttr returns attributes about the peer address.
func peerAttr(addr net.Addr) []attribute.KeyValue {
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return nil
	}

	if host == "" {
		host = "127.0.0.1"
	}

	return []attribute.KeyValue{
		semconv.NetPeerIPKey.String(host),
		semconv.NetPeerPortKey.String(port),
	}
}

// Envelope wraps the payload of all ssh messages with
// tracing context. Any servers that reply to a TracingChannel
// will attempt to parse the Envelope for all received requests and
// ensure that the original payload is provided to the handlers.
type Envelope struct {
	PropagationContext tracing.PropagationContext
	Payload            []byte
}

// createEnvelope wraps the provided payload with a tracing envelope
// that is used to propagate trace context .
func createEnvelope(ctx context.Context, propagator propagation.TextMapPropagator, payload []byte) Envelope {
	envelope := Envelope{
		Payload: payload,
	}

	span := oteltrace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return envelope
	}

	traceCtx := tracing.PropagationContextFromContext(ctx, tracing.WithTextMapPropagator(propagator))
	if len(traceCtx) == 0 {
		return envelope
	}

	envelope.PropagationContext = traceCtx

	return envelope
}

// wrapPayload wraps the provided payload within an envelope if tracing is
// enabled and there is any tracing information to propagate. Otherwise, the
// original payload is returned
func wrapPayload(ctx context.Context, supported bool, propagator propagation.TextMapPropagator, payload []byte) []byte {
	if !supported {
		return payload
	}

	envelope := createEnvelope(ctx, propagator, payload)
	if len(envelope.PropagationContext) == 0 {
		return payload
	}

	wrappedPayload, err := json.Marshal(envelope)
	if err == nil {
		return wrappedPayload
	}

	return payload
}

// NewCh is a wrapper around ssh.NewChannel that allows an
// Envelope to be provided to new channels.
type NewCh struct {
	ssh.NewChannel
	Envelope Envelope
}

// NewTraceCh creates a new NewCh
//
// The provided ssh.NewChannel will have any Envelope provided
// via ExtraData extracted so that the original payload can be
// provided to callers of NewCh.ExtraData.
func NewTraceCh(nch ssh.NewChannel) *NewCh {
	ch := &NewCh{
		NewChannel: nch,
	}

	var envelope Envelope
	if err := json.Unmarshal(nch.ExtraData(), &envelope); err == nil {
		ch.Envelope = envelope
	} else {
		ch.Envelope.Payload = nch.ExtraData()
	}

	return ch
}

// ExtraData returns the arbitrary payload for this channel, as supplied
// by the client. This data is specific to the channel type.
func (n NewCh) ExtraData() []byte {
	return n.Envelope.Payload
}
