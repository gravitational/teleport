/*
Copyright 2017 Gravitational, Inc.

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

package eventstest

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"
)

// ChannelEmitter emits audit events by writing them to a channel.
type ChannelEmitter struct {
	events chan events.AuditEvent
	log    logrus.FieldLogger
}

// NewChannelEmitter returns a new instance of test emitter.
func NewChannelEmitter(capacity int) *ChannelEmitter {
	return &ChannelEmitter{
		log:    logrus.WithField(trace.Component, "channel_emitter"),
		events: make(chan events.AuditEvent, capacity),
	}
}

func (e *ChannelEmitter) EmitAuditEvent(ctx context.Context, event events.AuditEvent) error {
	e.log.Infof("EmitAuditEvent(%v)", event)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case e.events <- event:
		return nil
	}
}

func (e *ChannelEmitter) C() <-chan events.AuditEvent {
	return e.events
}

func (e *ChannelEmitter) CreateAuditStream(ctx context.Context, sid session.ID) (events.Stream, error) {
	return e, nil
}

func (e *ChannelEmitter) ResumeAuditStream(ctx context.Context, sid session.ID, uploadID string) (events.Stream, error) {
	return e, nil
}

func (e *ChannelEmitter) Status() <-chan events.StreamStatus {
	return nil
}

func (e *ChannelEmitter) Done() <-chan struct{} {
	return nil
}

func (e *ChannelEmitter) Close(ctx context.Context) error {
	return nil
}

func (e *ChannelEmitter) Complete(ctx context.Context) error {
	return nil
}
