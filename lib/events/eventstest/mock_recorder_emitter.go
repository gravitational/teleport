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
	"sync"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"
)

// MockRecorderEmitter is a recorder and emitter that stores all events.
type MockRecorderEmitter struct {
	mu             sync.RWMutex
	events         []apievents.AuditEvent
	recordedEvents []apievents.PreparedSessionEvent
}

func (e *MockRecorderEmitter) Write(_ []byte) (int, error) {
	return 0, nil
}

// EmitAuditEvent emits audit event
func (e *MockRecorderEmitter) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, event)
	return nil
}

// RecordEvent records a session event
func (e *MockRecorderEmitter) RecordEvent(ctx context.Context, event apievents.PreparedSessionEvent) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.recordedEvents = append(e.recordedEvents, event)
	e.events = append(e.events, event.GetAuditEvent())
	return nil
}

// LastEvent returns the last emitted event.
func (e *MockRecorderEmitter) LastEvent() apievents.AuditEvent {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if len(e.events) == 0 {
		return nil
	}
	return e.events[len(e.events)-1]
}

// Events returns all the emitted events.
func (e *MockRecorderEmitter) Events() []apievents.AuditEvent {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]apievents.AuditEvent, len(e.events))
	copy(result, e.events)
	return result
}

// RecordedEvents returns all the emitted events.
func (e *MockRecorderEmitter) RecordedEvents() []apievents.PreparedSessionEvent {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]apievents.PreparedSessionEvent, len(e.recordedEvents))
	copy(result, e.recordedEvents)
	return result
}

// Reset clears the emitted events.
func (e *MockRecorderEmitter) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = nil
}

func (e *MockRecorderEmitter) CreateAuditStream(ctx context.Context, sid session.ID) (apievents.Stream, error) {
	return e, nil
}

func (e *MockRecorderEmitter) ResumeAuditStream(ctx context.Context, sid session.ID, uploadID string) (apievents.Stream, error) {
	return e, nil
}

func (e *MockRecorderEmitter) Status() <-chan apievents.StreamStatus {
	return nil
}

func (e *MockRecorderEmitter) Done() <-chan struct{} {
	return nil
}

func (e *MockRecorderEmitter) Close(ctx context.Context) error {
	return nil
}

func (e *MockRecorderEmitter) Complete(ctx context.Context) error {
	return nil
}
