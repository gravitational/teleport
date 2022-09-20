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

package events

import (
	"context"
	"sync"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"
)

// MockEmitter is emitter that stores last audit event
type MockEmitter struct {
	mtx    sync.RWMutex
	events []apievents.AuditEvent
}

// CreateAuditStream creates a stream that discards all events
func (e *MockEmitter) CreateAuditStream(ctx context.Context, sid session.ID) (apievents.Stream, error) {
	return e, nil
}

// ResumeAuditStream resumes a stream that discards all events
func (e *MockEmitter) ResumeAuditStream(ctx context.Context, sid session.ID, uploadID string) (apievents.Stream, error) {
	return e, nil
}

// EmitAuditEvent emits audit event
func (e *MockEmitter) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	e.mtx.Lock()
	defer e.mtx.Unlock()
	e.events = append(e.events, event)
	return nil
}

// LastEvent returns last emitted event
func (e *MockEmitter) LastEvent() apievents.AuditEvent {
	e.mtx.RLock()
	defer e.mtx.RUnlock()
	if len(e.events) == 0 {
		return nil
	}
	return e.events[len(e.events)-1]
}

// Events returns all the emitted events
func (e *MockEmitter) Events() []apievents.AuditEvent {
	e.mtx.RLock()
	defer e.mtx.RUnlock()
	return e.events
}

// Reset resets state to zero values.
func (e *MockEmitter) Reset() {
	e.mtx.Lock()
	defer e.mtx.Unlock()
	e.events = nil
}

// Status returns a channel that always blocks
func (e *MockEmitter) Status() <-chan apievents.StreamStatus {
	return nil
}

// Done returns channel closed when streamer is closed
// should be used to detect sending errors
func (e *MockEmitter) Done() <-chan struct{} {
	return nil
}

// Close flushes non-uploaded flight stream data without marking
// the stream completed and closes the stream instance
func (e *MockEmitter) Close(ctx context.Context) error {
	return nil
}

// Complete does nothing
func (e *MockEmitter) Complete(ctx context.Context) error {
	return nil
}
