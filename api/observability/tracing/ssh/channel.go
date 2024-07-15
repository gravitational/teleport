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

	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/observability/tracing"
)

// Channel is a wrapper around ssh.Channel that adds tracing support.
type Channel struct {
	ssh.Channel
	tracingSupported tracingCapability
	opts             []tracing.Option
}

// NewTraceChannel creates a new Channel.
func NewTraceChannel(ch ssh.Channel, opts ...tracing.Option) *Channel {
	return &Channel{
		Channel: ch,
		opts:    opts,
	}
}

// SendRequest sends a global request, and returns the
// reply. If tracing is enabled, the provided payload
// is wrapped in an Envelope to forward any tracing context.
func (c *Channel) SendRequest(ctx context.Context, name string, wantReply bool, payload []byte) (_ bool, err error) {
	config := tracing.NewConfig(c.opts)
	tracer := config.TracerProvider.Tracer(instrumentationName)

	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("ssh.ChannelRequest/%s", name),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String("ssh.Channel"),
			semconv.RPCMethodKey.String("SendRequest"),
			semconv.RPCSystemKey.String("ssh"),
		),
	)
	defer func() { tracing.EndSpan(span, err) }()

	return c.Channel.SendRequest(
		name, wantReply, wrapPayload(ctx, c.tracingSupported, config.TextMapPropagator, payload),
	)
}

// NewChannel is a wrapper around ssh.NewChannel that allows an
// Envelope to be provided to new channels.
type NewChannel struct {
	ssh.NewChannel
	Envelope Envelope
}

// NewTraceNewChannel wraps the ssh.NewChannel in a new NewChannel
//
// The provided ssh.NewChannel will have any Envelope provided
// via ExtraData extracted so that the original payload can be
// provided to callers of NewCh.ExtraData.
func NewTraceNewChannel(nch ssh.NewChannel) *NewChannel {
	ch := &NewChannel{
		NewChannel: nch,
	}

	data := nch.ExtraData()

	var envelope Envelope
	if err := json.Unmarshal(data, &envelope); err == nil {
		ch.Envelope = envelope
	} else {
		ch.Envelope.Payload = data
	}

	return ch
}

// ExtraData returns the arbitrary payload for this channel, as supplied
// by the client. This data is specific to the channel type.
func (n NewChannel) ExtraData() []byte {
	return n.Envelope.Payload
}
