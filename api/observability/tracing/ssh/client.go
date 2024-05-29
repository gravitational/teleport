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
	"bytes"
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/observability/tracing"
)

// Client is a wrapper around ssh.Client that adds tracing support.
type Client struct {
	*ssh.Client
	opts       []tracing.Option
	capability tracingCapability
}

type tracingCapability int

const (
	tracingUnknown tracingCapability = iota
	tracingUnsupported
	tracingSupported
)

// NewClient creates a new Client.
//
// The server being connected to is probed to determine if it supports
// ssh tracing. This is done by inspecting the version the server provides
// during the handshake, if it comes from a Teleport ssh server, then all
// payloads will be wrapped in an Envelope with tracing context. All Session
// and Channel created from the returned Client will honor the clients view
// of whether they should provide tracing context.
func NewClient(c ssh.Conn, chans <-chan ssh.NewChannel, reqs <-chan *ssh.Request, opts ...tracing.Option) *Client {
	clt := &Client{
		Client:     ssh.NewClient(c, chans, reqs),
		opts:       opts,
		capability: tracingUnsupported,
	}

	if bytes.HasPrefix(clt.ServerVersion(), []byte("SSH-2.0-Teleport")) {
		clt.capability = tracingSupported
	}

	return clt
}

// DialContext initiates a connection to the addr from the remote host.
// The resulting connection has a zero LocalAddr() and RemoteAddr().
func (c *Client) DialContext(ctx context.Context, n, addr string) (net.Conn, error) {
	tracer := tracing.NewConfig(c.opts).TracerProvider.Tracer(instrumentationName)
	ctx, span := tracer.Start(
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

	// create the wrapper while the lock is held
	wrapper := &clientWrapper{
		capability: c.capability,
		Conn:       c.Client.Conn,
		opts:       c.opts,
		ctx:        ctx,
		contexts:   make(map[string][]context.Context),
	}

	conn, err := wrapper.Dial(n, addr)
	return conn, trace.Wrap(err)
}

// SendRequest sends a global request, and returns the
// reply. If tracing is enabled, the provided payload
// is wrapped in an Envelope to forward any tracing context.
func (c *Client) SendRequest(
	ctx context.Context, name string, wantReply bool, payload []byte,
) (_ bool, _ []byte, err error) {
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
	defer func() { tracing.EndSpan(span, err) }()

	return c.Client.SendRequest(
		name, wantReply, wrapPayload(ctx, c.capability, config.TextMapPropagator, payload),
	)
}

// OpenChannel tries to open a channel. If tracing is enabled,
// the provided payload is wrapped in an Envelope to forward
// any tracing context.
func (c *Client) OpenChannel(
	ctx context.Context, name string, data []byte,
) (_ *Channel, _ <-chan *ssh.Request, err error) {
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
	defer func() { tracing.EndSpan(span, err) }()

	ch, reqs, err := c.Client.OpenChannel(name, wrapPayload(ctx, c.capability, config.TextMapPropagator, data))
	return &Channel{
		Channel: ch,
		opts:    c.opts,
	}, reqs, err
}

// NewSession creates a new SSH session that is passed tracing context
// so that spans may be correlated properly over the ssh connection.
func (c *Client) NewSession(ctx context.Context) (*Session, error) {
	return c.newSession(ctx, nil)
}

// NewSessionWithRequestCallback creates a new SSH session that is passed
// tracing context so that spans may be correlated properly over the ssh
// connection. The handling of channel requests from the underlying SSH
// session can be controlled with chanReqCallback.
func (c *Client) NewSessionWithRequestCallback(ctx context.Context, chanReqCallback ChannelRequestCallback) (*Session, error) {
	return c.newSession(ctx, chanReqCallback)
}

func (c *Client) newSession(ctx context.Context, chanReqCallback ChannelRequestCallback) (*Session, error) {
	tracer := tracing.NewConfig(c.opts).TracerProvider.Tracer(instrumentationName)

	ctx, span := tracer.Start(
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

	// create the wrapper while the lock is still held
	wrapper := &clientWrapper{
		capability: c.capability,
		Conn:       c.Client.Conn,
		opts:       c.opts,
		ctx:        ctx,
		contexts:   make(map[string][]context.Context),
	}

	// get a session from the wrapper
	session, err := wrapper.NewSession(chanReqCallback)
	return session, trace.Wrap(err)
}

// clientWrapper wraps the ssh.Conn for individual ssh.Client
// operations to intercept internal calls by the ssh.Client to
// OpenChannel. This allows for internal operations within the
// ssh.Client to have their payload wrapped in an Envelope to
// forward tracing context when tracing is enabled.
type clientWrapper struct {
	// Conn is the ssh.Conn that requests will be forwarded to
	ssh.Conn
	// capability the tracingCapability of the ssh server
	capability tracingCapability
	// ctx the context which should be used to create spans from
	ctx context.Context
	// opts the tracing options to use for creating spans with
	opts []tracing.Option

	// lock protects the context queue
	lock sync.Mutex
	// contexts a LIFO queue of context.Context per channel name.
	contexts map[string][]context.Context
}

// ChannelRequestCallback allows the handling of channel requests
// to be customized. nil can be returned if you don't want
// golang/x/crypto/ssh to handle the request.
type ChannelRequestCallback func(req *ssh.Request) *ssh.Request

// NewSession opens a new Session for this client.
func (c *clientWrapper) NewSession(callback ChannelRequestCallback) (*Session, error) {
	// create a client that will defer to us when
	// opening the "session" channel so that we
	// can add an Envelope to the request
	client := &ssh.Client{
		Conn: c,
	}

	var session *ssh.Session
	var err error
	if callback != nil {
		// open a session manually so we can take ownership of the
		// requests chan
		ch, originalReqs, openChannelErr := client.OpenChannel("session", nil)
		if openChannelErr != nil {
			return nil, trace.Wrap(openChannelErr)
		}

		// pass the channel requests to the provided callback and
		// forward them to another chan so golang.org/x/crypto/ssh
		// can handle Session exiting correctly
		reqs := make(chan *ssh.Request, cap(originalReqs))
		go func() {
			defer close(reqs)

			for req := range originalReqs {
				if req := callback(req); req != nil {
					reqs <- req
				}
			}
		}()

		session, err = newCryptoSSHSession(ch, reqs)
		if err != nil {
			_ = ch.Close()
			return nil, trace.Wrap(err)
		}
	} else {
		session, err = client.NewSession()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// wrap the session so all session requests on the channel
	// can be traced
	return &Session{
		Session: session,
		wrapper: c,
	}, nil
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
		panic("wrappedSSHConn OpenChannel called more than once")
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

// Dial initiates a connection to the addr from the remote host.
func (c *clientWrapper) Dial(n, addr string) (net.Conn, error) {
	// create a client that will defer to us when
	// opening the "direct-tcpip" channel so that we
	// can add an Envelope to the request
	client := &ssh.Client{
		Conn: c,
	}

	return client.Dial(n, addr)
}

// addContext adds the provided context.Context to the end of
// the list for the provided channel name
func (c *clientWrapper) addContext(ctx context.Context, name string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.contexts[name] = append(c.contexts[name], ctx)
}

// nextContext returns the first context.Context for the provided
// channel name
func (c *clientWrapper) nextContext(name string) context.Context {
	c.lock.Lock()
	defer c.lock.Unlock()

	contexts, ok := c.contexts[name]
	switch {
	case !ok, len(contexts) <= 0:
		return context.Background()
	case len(contexts) == 1:
		delete(c.contexts, name)
		return contexts[0]
	default:
		c.contexts[name] = contexts[1:]
		return contexts[0]
	}
}

// OpenChannel tries to open a channel. If tracing is enabled,
// the provided payload is wrapped in an Envelope to forward
// any tracing context.
func (c *clientWrapper) OpenChannel(name string, data []byte) (_ ssh.Channel, _ <-chan *ssh.Request, err error) {
	config := tracing.NewConfig(c.opts)
	tracer := config.TracerProvider.Tracer(instrumentationName)
	ctx, span := tracer.Start(
		c.ctx,
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
	defer func() { tracing.EndSpan(span, err) }()

	ch, reqs, err := c.Conn.OpenChannel(name, wrapPayload(ctx, c.capability, config.TextMapPropagator, data))
	return channelWrapper{
		Channel: ch,
		manager: c,
	}, reqs, err
}

// channelWrapper wraps an ssh.Channel to allow for requests to
// contain tracing context.
type channelWrapper struct {
	ssh.Channel
	manager *clientWrapper
}

// SendRequest sends a channel request. If tracing is enabled,
// the provided payload is wrapped in an Envelope to forward
// any tracing context.
//
// It is the callers' responsibility to ensure that addContext is
// called with the appropriate context.Context prior to any
// requests being sent along the channel.
func (c channelWrapper) SendRequest(name string, wantReply bool, payload []byte) (_ bool, err error) {
	config := tracing.NewConfig(c.manager.opts)
	ctx, span := config.TracerProvider.Tracer(instrumentationName).Start(
		c.manager.nextContext(name),
		fmt.Sprintf("ssh.ChannelRequest/%s", name),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.Bool("want_reply", wantReply),
			semconv.RPCServiceKey.String("ssh.Channel"),
			semconv.RPCMethodKey.String("SendRequest"),
			semconv.RPCSystemKey.String("ssh"),
		),
	)
	defer func() { tracing.EndSpan(span, err) }()

	return c.Channel.SendRequest(name, wantReply, wrapPayload(ctx, c.manager.capability, config.TextMapPropagator, payload))
}
