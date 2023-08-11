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

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"
)

// ChannelEmitter emits audit events by writing them to a channel.
type ChannelEmitter struct {
	events chan apievents.AuditEvent
	log    logrus.FieldLogger
}

// NewChannelEmitter returns a new instance of test emitter.
func NewChannelEmitter(capacity int) *ChannelEmitter {
	return &ChannelEmitter{
		log:    logrus.WithField(trace.Component, "channel_emitter"),
		events: make(chan apievents.AuditEvent, capacity),
	}
}

func (e *ChannelEmitter) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	e.log.Infof("EmitAuditEvent(%v)", event)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case e.events <- event:
		return nil
	}
}

func (e *ChannelEmitter) C() <-chan apievents.AuditEvent {
	return e.events
}

// ChannelRecorder records session events by writing them to a channel.
type ChannelRecorder struct {
	events chan apievents.AuditEvent
	log    logrus.FieldLogger
}

// NewChannelRecorder returns a new instance of test recorder.
func NewChannelRecorder(capacity int) *ChannelRecorder {
	return &ChannelRecorder{
		log:    logrus.WithField(trace.Component, "channel_recorder"),
		events: make(chan apievents.AuditEvent, capacity),
	}
}

func (e *ChannelRecorder) C() <-chan apievents.AuditEvent {
	return e.events
}

func (e *ChannelRecorder) CreateAuditStream(ctx context.Context, sid session.ID) (apievents.Stream, error) {
	return e, nil
}

func (e *ChannelRecorder) ResumeAuditStream(ctx context.Context, sid session.ID, uploadID string) (apievents.Stream, error) {
	return e, nil
}

func (*ChannelRecorder) Write(b []byte) (int, error) {
	return len(b), nil
}

func (e *ChannelRecorder) RecordEvent(ctx context.Context, event apievents.PreparedSessionEvent) error {
	e.log.Infof("RecordEvent(%v)", event.GetAuditEvent())
	select {
	case <-ctx.Done():
		return ctx.Err()
	case e.events <- event.GetAuditEvent():
		return nil
	}
}

func (e *ChannelRecorder) Status() <-chan apievents.StreamStatus {
	return nil
}

func (e *ChannelRecorder) Done() <-chan struct{} {
	return nil
}

func (e *ChannelRecorder) Close(ctx context.Context) error {
	return nil
}

func (e *ChannelRecorder) Complete(ctx context.Context) error {
	return nil
}
