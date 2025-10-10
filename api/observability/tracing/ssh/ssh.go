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
	"cmp"
	"context"
	"encoding/json"
	"net"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/observability/tracing"
)

const (
	// EnvsRequest sets multiple environment variables that will be applied to any
	// command executed by Shell or Run.
	// See [EnvsReq] for the corresponding payload.
	EnvsRequest = "envs@goteleport.com"

	// instrumentationName is the name of this instrumentation package.
	instrumentationName = "otelssh"
)

// EnvsReq contains json marshaled key:value pairs sent as the
// payload for an [EnvsRequest].
type EnvsReq struct {
	// EnvsJSON is a json marshaled map[string]string containing
	// environment variables.
	EnvsJSON []byte `json:"envs"`
}

// FileTransferReq contains parameters used to create a file transfer
// request to be stored in the SSH server
type FileTransferReq struct {
	// Download is true if the file transfer requests a download, false if upload
	Download bool
	// Location is the location of the file to be downloaded, or directory to upload a file
	Location string
	// Filename is the name of the file to be uploaded
	Filename string
}

// FileTransferDecisionReq contains parameters used to approve or deny an active
// file transfer request on the SSH server
type FileTransferDecisionReq struct {
	// RequestID is the ID of the file transfer request being responded to
	RequestID string
	// Approved is true if approved, false if denied.
	Approved bool
}

// ContextFromRequest extracts any tracing data provided via an Envelope
// in the ssh.Request payload. If the payload contains an Envelope, then
// the context returned will have tracing data populated from the remote
// tracing context and the ssh.Request payload will be replaced with the
// original payload from the client.
func ContextFromRequest(req *ssh.Request, opts ...tracing.Option) context.Context {
	ctx := context.Background()

	var envelope Envelope
	if err := json.Unmarshal(req.Payload, &envelope); err != nil {
		return ctx
	}

	ctx = tracing.WithPropagationContext(ctx, envelope.PropagationContext, opts...)
	req.Payload = envelope.Payload

	return ctx
}

// ContextFromNewChannel extracts any tracing data provided via an Envelope
// in the ssh.NewChannel ExtraData. If the ExtraData contains an Envelope, then
// the context returned will have tracing data populated from the remote
// tracing context and the ssh.NewChannel wrapped in a TraceCh so that the
// original ExtraData from the client is exposed instead of the Envelope
// payload.
func ContextFromNewChannel(nch ssh.NewChannel, opts ...tracing.Option) (context.Context, ssh.NewChannel) {
	ch := NewTraceNewChannel(nch)
	ctx := tracing.WithPropagationContext(context.Background(), ch.Envelope.PropagationContext, opts...)

	return ctx, ch
}

// Dial starts a client connection to the given SSH server. It is a
// convenience function that connects to the given network address,
// initiates the SSH handshake, and then sets up a Client.  For access
// to incoming channels and requests, use net.Dial with NewClientConn
// instead.
func Dial(ctx context.Context, network, addr string, config *ssh.ClientConfig, opts ...tracing.Option) (*Client, error) {
	tracer := tracing.NewConfig(opts).TracerProvider.Tracer(instrumentationName)
	ctx, span := tracer.Start(
		ctx,
		"ssh/Dial",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("network", network),
			attribute.String("address", addr),
			semconv.RPCServiceKey.String("ssh"),
			semconv.RPCMethodKey.String("Dial"),
			semconv.RPCSystemKey.String("ssh"),
		),
	)
	defer span.End()

	dialer := net.Dialer{Timeout: config.Timeout}
	conn, err := dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}
	c, err := NewClientWithTimeout(ctx, conn, addr, config, opts...)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// NewClientConnWithTimeout creates a new SSH client connection that includes tracing context,
// allowing spans to be properly correlated across the SSH connection.
//
// The connection respects the earliest of the following:
// - The context's deadline or cancellation
// - The timeout specified in the config
// - A default timeout of 30 seconds if config doesn't specify a timeout
//
// Behavior based on config.Timeout:
// - If > 0: the timeout is applied in addition to any context deadline.
// - If == 0: a default timeout of 30 seconds is used to avoid hanging connections.
// - If < 0: only the context’s deadline or cancellation is used.
func NewClientConnWithTimeout(ctx context.Context, conn net.Conn, addr string, config *ssh.ClientConfig, opts ...tracing.Option) (ssh.Conn, <-chan ssh.NewChannel, <-chan *ssh.Request, error) {
	tracer := tracing.NewConfig(opts).TracerProvider.Tracer(instrumentationName)
	ctx, span := tracer.Start( //nolint:staticcheck,ineffassign // keeping shadowed ctx to avoid accidental missing in the future
		ctx,
		"ssh/NewClientConnWithTimeout",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			append(
				peerAttr(conn.RemoteAddr()),
				attribute.String("address", addr),
				semconv.RPCServiceKey.String("ssh"),
				semconv.RPCMethodKey.String("NewClientConnWithTimeout"),
				semconv.RPCSystemKey.String("ssh"),
			)...,
		),
	)
	defer span.End()

	// ssh.ClientConfig.Timeout is not the total timeout for the connection
	// establishment, including DNS resolution, TCP connection, but it doesn't
	// include the SSH estalbishment.
	// From the crypto/ssh docs:
	// > Timeout is the maximum amount of time for the TCP connection to establish.
	//
	// Since we pass the connection here, the timeout will never be enforced by the
	// ssh package. `NewClientConnWithDeadline` tries to enforced it by setting
	// the read deadline on the connection, but might not be sufficient because
	// we have some net.Conn implementations that don't support setting read deadlines
	// and will block forever.
	// To be sure that we don't block forever, we set up a timer that will close
	// the connection when the timeout is reached.
	// If the context has a deadline, we use that instead and take the minimum
	// between the two.
	// If neither is set, we default to 30 seconds.

	// We aim to close the connection to avoid clients to hang forever
	// if the server is not responding.

	// If config.Timeout is negative, we don't set a timeout and restrict
	// ourselves to the context deadline if any.
	if config.Timeout >= 0 {
		newCtx, cancel := context.WithTimeout(
			ctx,
			cmp.Or(config.Timeout, defaults.DefaultIOTimeout),
		)
		defer cancel()
		ctx = newCtx
	}

	stopFn := context.AfterFunc(ctx, func() {
		_ = conn.Close()
	})
	defer stopFn()

	c, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		// if the context was canceled or timed out, return that an aggregated error instead
		// of the original error returned from NewClientConn. The returned error would be something like
		// "ssh: handshake failed: read tcp {ip}:{port} -> {ip}:{port} use of closed network connection"
		// which doesn't indicate the real error was a timeout or cancellation.
		// If the context was not canceled and the function failed, it returns the original error as
		// ctx.Err() would be nil.
		return nil, nil, nil, trace.NewAggregate(ctx.Err(), err)
	}

	if !stopFn() {
		// we failed to stop the AfterFunc so conn will be closed and
		// c will soon become invalid no matter what we do, so we
		// drain it and close it
		_ = conn.Close()
		go func() {
			for newCh := range chans {
				_ = newCh.Reject(0, "")
			}
		}()
		go ssh.DiscardRequests(reqs)
		_ = c.Close()
		return nil, nil, nil, trace.Wrap(ctx.Err())
	}

	return c, chans, reqs, nil
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
// tracing context. Any servers that support tracing propagation
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
func wrapPayload(ctx context.Context, supported tracingCapability, propagator propagation.TextMapPropagator, payload []byte) []byte {
	if supported != tracingSupported {
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
