/*
Copyright 2017-2020 Gravitational, Inc.

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
	"sync"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"
)

// MockRecorderEmitter is a recorder and emitter that stores all events.
type MockRecorderEmitter struct {
	mu     sync.RWMutex
	events []apievents.AuditEvent
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
