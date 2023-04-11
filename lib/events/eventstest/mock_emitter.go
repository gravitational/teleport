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

	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"
)

// MockEmitter is an emitter that stores all emitted events.
type MockEmitter struct {
	mu     sync.RWMutex
	events []events.AuditEvent
}

// EmitAuditEvent emits audit event
func (e *MockEmitter) EmitAuditEvent(ctx context.Context, event events.AuditEvent) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, event)
	return nil
}

// LastEvent returns the last emitted event.
func (e *MockEmitter) LastEvent() events.AuditEvent {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if len(e.events) == 0 {
		return nil
	}
	return e.events[len(e.events)-1]
}

// Events returns all the emitted events.
func (e *MockEmitter) Events() []events.AuditEvent {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]events.AuditEvent, len(e.events))
	copy(result, e.events)
	return result
}

// Reset clears the emitted events.
func (e *MockEmitter) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = nil
}

func (e *MockEmitter) CreateAuditStream(ctx context.Context, sid session.ID) (events.Stream, error) {
	return e, nil
}

func (e *MockEmitter) ResumeAuditStream(ctx context.Context, sid session.ID, uploadID string) (events.Stream, error) {
	return e, nil
}

func (e *MockEmitter) Status() <-chan events.StreamStatus {
	return nil
}

func (e *MockEmitter) Done() <-chan struct{} {
	return nil
}

func (e *MockEmitter) Close(ctx context.Context) error {
	return nil
}

func (e *MockEmitter) Complete(ctx context.Context) error {
	return nil
}
