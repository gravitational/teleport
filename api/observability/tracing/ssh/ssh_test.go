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
	"crypto/ed25519"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/observability/tracing"
)

const testPayload = "test"

type server struct {
	listener net.Listener
	config   *ssh.ServerConfig
	handler  func(*ssh.ServerConn, <-chan ssh.NewChannel, <-chan *ssh.Request)

	cSigner ssh.Signer
	hSigner ssh.Signer
}

func (s *server) Run(errC chan error) {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				errC <- err
			}
			return
		}

		go func() {
			sconn, chans, reqs, err := ssh.NewServerConn(conn, s.config)
			if err != nil {
				errC <- err
				return
			}
			s.handler(sconn, chans, reqs)
		}()
	}
}

func (s *server) Stop() error {
	return s.listener.Close()
}

func generateSigner(t *testing.T) ssh.Signer {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	sshSigner, err := ssh.NewSignerFromSigner(private)
	require.NoError(t, err)
	return sshSigner
}

func (s *server) GetClient(t *testing.T) (ssh.Conn, <-chan ssh.NewChannel, <-chan *ssh.Request) {
	conn, err := net.Dial("tcp", s.listener.Addr().String())
	require.NoError(t, err)

	sconn, nc, r, err := ssh.NewClientConn(conn, "", &ssh.ClientConfig{
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(s.cSigner)},
		HostKeyCallback: ssh.FixedHostKey(s.hSigner.PublicKey()),
	})
	require.NoError(t, err)

	return sconn, nc, r
}

func newServer(t *testing.T, tracingCap tracingCapability, handler func(*ssh.ServerConn, <-chan ssh.NewChannel, <-chan *ssh.Request)) *server {
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	cSigner := generateSigner(t)
	hSigner := generateSigner(t)

	version := "SSH-2.0-Teleport"
	if tracingCap != tracingSupported {
		version = "SSH-2.0"
	}

	config := &ssh.ServerConfig{
		NoClientAuth:  true,
		ServerVersion: version,
	}
	config.AddHostKey(hSigner)

	srv := &server{
		listener: listener,
		config:   config,
		handler:  handler,
		cSigner:  cSigner,
		hSigner:  hSigner,
	}

	t.Cleanup(func() { require.NoError(t, srv.Stop()) })

	return srv
}

type handler struct {
	tracingSupported tracingCapability
	errChan          chan error
	ctx              context.Context
}

func (h handler) handle(sconn *ssh.ServerConn, chans <-chan ssh.NewChannel, reqs <-chan *ssh.Request) {
	for {
		select {
		case <-h.ctx.Done():
			return
		case req := <-reqs:
			if req == nil {
				return
			}

			h.requestHandler(req)

		case ch := <-chans:
			if ch == nil {
				return
			}

			h.channelHandler(ch)
		}
	}
}

func (h handler) requestHandler(req *ssh.Request) {
	switch {
	case req.Type == "test":
		defer func() {
			if req.WantReply {
				if err := req.Reply(true, nil); err != nil {
					h.errChan <- err
				}
			}
		}()

	default:
		if err := req.Reply(false, nil); err != nil {
			h.errChan <- err
		}
	}
}

func (h handler) channelHandler(ch ssh.NewChannel) {
	switch ch.ChannelType() {
	case "session":
		switch h.tracingSupported {
		case tracingUnsupported:
			if subtle.ConstantTimeCompare(ch.ExtraData(), []byte(testPayload)) == 1 {
				h.errChan <- errors.New("payload mismatch")
			}
		case tracingSupported:
			var envelope Envelope
			if err := json.Unmarshal(ch.ExtraData(), &envelope); err != nil {
				h.errChan <- trace.Wrap(err, "failed to unmarshal envelope")
				ch.Accept()
				return
			}
			if len(envelope.PropagationContext) <= 0 {
				h.errChan <- errors.New("empty propagation context")
				ch.Accept()
				return
			}
			if len(envelope.Payload) > 0 {
				h.errChan <- errors.New("payload mismatch")
				ch.Accept()
				return
			}
		}

		_, chReqs, err := ch.Accept()
		if err != nil {
			h.errChan <- trace.Wrap(err, "failed to accept channel")
			return
		}

		go func() {
			for {
				select {
				case <-h.ctx.Done():
					return
				case req := <-chReqs:
					switch req.Type {
					case "subsystem":
						h.subsystemHandler(req)
					}
				}
			}
		}()
	default:
		if err := ch.Reject(ssh.UnknownChannelType, "unknown channel type"); err != nil {
			h.errChan <- trace.Wrap(err, "failed to reject channel")
		}
	}
}

type subsystemRequestMsg struct {
	Subsystem string
}

func (h handler) subsystemHandler(req *ssh.Request) {
	defer func() {
		if req.WantReply {
			if err := req.Reply(true, nil); err != nil {
				h.errChan <- err
			}
		}
	}()

	switch h.tracingSupported {
	case tracingUnsupported:
		var msg subsystemRequestMsg
		if err := ssh.Unmarshal(req.Payload, &msg); err != nil {
			h.errChan <- trace.Wrap(err, "failed to unmarshal payload")
			return
		}

		if msg.Subsystem != "test" {
			h.errChan <- errors.New("received wrong subsystem")
		}
	case tracingSupported:
		var envelope Envelope
		if err := json.Unmarshal(req.Payload, &envelope); err != nil {
			h.errChan <- trace.Wrap(err, "failed to unmarshal envelope")
			return
		}
		if len(envelope.PropagationContext) <= 0 {
			h.errChan <- errors.New("empty propagation context")
			return
		}

		var msg subsystemRequestMsg
		if err := ssh.Unmarshal(envelope.Payload, &msg); err != nil {
			h.errChan <- trace.Wrap(err, "failed to unmarshal payload")
			return
		}
		if msg.Subsystem != "test" {
			h.errChan <- errors.New("received wrong subsystem")
			return
		}
	default:
		if err := req.Reply(false, nil); err != nil {
			h.errChan <- err
		}
	}
}

func TestClient(t *testing.T) {
	cases := []struct {
		name             string
		tracingSupported tracingCapability
	}{
		{
			name:             "server supports tracing",
			tracingSupported: tracingSupported,
		},
		{
			name:             "server does not support tracing",
			tracingSupported: tracingSupported,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			errChan := make(chan error, 5)

			handler := handler{
				tracingSupported: tt.tracingSupported,
				errChan:          errChan,
				ctx:              ctx,
			}

			srv := newServer(t, tt.tracingSupported, handler.handle)
			go srv.Run(errChan)

			tp := sdktrace.NewTracerProvider()
			conn, chans, reqs := srv.GetClient(t)
			client := NewClient(
				conn,
				chans,
				reqs,
				tracing.WithTracerProvider(tp),
				tracing.WithTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})),
			)
			require.Equal(t, tt.tracingSupported, client.capability)

			ctx, span := tp.Tracer("test").Start(context.Background(), "test")
			ok, resp, err := client.SendRequest(ctx, "test", true, []byte("test"))
			span.End()
			require.True(t, ok)
			require.Empty(t, resp)
			require.NoError(t, err)

			select {
			case err := <-errChan:
				require.NoError(t, err)
			default:
			}

			session, err := client.NewSession(ctx)
			require.NoError(t, err)
			require.NotNil(t, session)

			select {
			case err := <-errChan:
				require.NoError(t, err)
			default:
			}

			require.NoError(t, session.RequestSubsystem(ctx, "test"))

			select {
			case err := <-errChan:
				require.NoError(t, err)
			default:
			}
		})
	}
}

func TestWrapPayload(t *testing.T) {
	testPayload := []byte("test")

	nonRecordingCtx, nonRecordingSpan := otel.GetTracerProvider().Tracer("non-recording").Start(context.Background(), "test")
	nonRecordingSpan.End()

	emptyCtx, emptySpan := sdktrace.NewTracerProvider().Tracer("empty-trace-context").Start(context.Background(), "test")
	t.Cleanup(func() { emptySpan.End() })

	recordingCtx, recordingSpan := sdktrace.NewTracerProvider().Tracer("recording").Start(context.Background(), "test")
	t.Cleanup(func() { recordingSpan.End() })
	cases := []struct {
		name             string
		ctx              context.Context
		supported        tracingCapability
		propagator       propagation.TextMapPropagator
		payloadAssertion require.ComparisonAssertionFunc
	}{
		{
			name:             "unsupported returns provided payload",
			ctx:              recordingCtx,
			supported:        tracingUnsupported,
			payloadAssertion: require.Equal,
		},
		{

			name:             "non-recording spans aren't propagated",
			supported:        tracingSupported,
			ctx:              nonRecordingCtx,
			payloadAssertion: require.Equal,
		},
		{
			name:             "empty trace context is not propagated",
			supported:        tracingSupported,
			ctx:              emptyCtx,
			payloadAssertion: require.Equal,
		},
		{
			name:       "recording spans are propagated",
			supported:  tracingSupported,
			ctx:        recordingCtx,
			propagator: propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}),
			payloadAssertion: func(t require.TestingT, i interface{}, i2 interface{}, i3 ...interface{}) {
				payload, ok := i2.([]byte)
				require.True(t, ok)

				require.NotEqual(t, testPayload, payload)

				var envelope Envelope
				require.NoError(t, json.Unmarshal(payload, &envelope))
				require.Equal(t, testPayload, envelope.Payload)
				require.NotEmpty(t, envelope.PropagationContext)
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if tt.propagator == nil {
				tt.propagator = otel.GetTextMapPropagator()
			}
			payload := wrapPayload(tt.ctx, tt.supported, tt.propagator, testPayload)
			tt.payloadAssertion(t, testPayload, payload)
		})
	}
}
