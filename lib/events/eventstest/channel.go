/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package eventstest

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
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
		log:    logrus.WithField(teleport.ComponentKey, "channel_emitter"),
		events: make(chan apievents.AuditEvent, capacity),
	}
}

func (e *ChannelEmitter) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	e.log.Infof("EmitAuditEvent(%v)", event)
	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case e.events <- event:
			return nil
		case <-time.After(5 * time.Second):
			e.log.WithFields(logrus.Fields{
				"event":   event,
				"elapsed": time.Since(start),
			}).Info("EmitAuditEvent has been blocked sending to a full ChannelEmitter for a long time")
		}
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
		log:    logrus.WithField(teleport.ComponentKey, "channel_recorder"),
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
	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case e.events <- event.GetAuditEvent():
			return nil
		case <-time.After(5 * time.Second):
			e.log.WithFields(logrus.Fields{
				"event":   event.GetAuditEvent(),
				"elapsed": time.Since(start),
			}).Info("RecordEvent has been blocked sending to a full ChannelRecorder for a long time")
		}
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
