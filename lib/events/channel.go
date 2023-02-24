/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package events

import (
	"context"

	"github.com/gravitational/trace"

	apievents "github.com/gravitational/teleport/api/types/events"
)

var _ apievents.Emitter = &ChannelEmitter{}

// ChannelEmitterConfig is a configuration for channel emitter. This is primarily
// of interest for testing.
type ChannelEmitterConfig struct {
	Channel chan apievents.AuditEvent
}

// SetDefaults sets config defaults.
func (c *ChannelEmitterConfig) SetDefaults() {
	if c.Channel == nil {
		c.Channel = make(chan apievents.AuditEvent)
	}
}

// NewChannelEmitter returns an emitter where events are sent to a configured channel.
func NewChannelEmitter(cfg ChannelEmitterConfig) *ChannelEmitter {
	cfg.SetDefaults()

	return &ChannelEmitter{
		channel: cfg.Channel,
	}
}

// ChannelEmitter sends all events to a configured channel.
type ChannelEmitter struct {
	channel chan apievents.AuditEvent
}

// EmitAuditEvent sends the given event to a channel.
func (c *ChannelEmitter) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	select {
	case c.channel <- event:
	case <-ctx.Done():
		return trace.BadParameter("context canceled")
	}
	return nil
}
