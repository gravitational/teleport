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
	"time"

	"github.com/gravitational/teleport/lib/session"

	"github.com/gravitational/trace"
)

// NewMockAuditLog returns an instance of MockAuditLog.
func NewMockAuditLog(capacity int) *MockAuditLog {
	return &MockAuditLog{
		SlicesC:         make(chan *SessionSlice, capacity),
		FailedAttemptsC: make(chan *SessionSlice, capacity),
	}
}

// EmittedEvent holds the event type and event fields.
type EmittedEvent struct {
	EventType Event
	Fields    EventFields
}

// MockAuditLog is audit log used for tests.
type MockAuditLog struct {
	sync.Mutex
	returnError     error
	FailedAttemptsC chan *SessionSlice
	SlicesC         chan *SessionSlice
	EmittedEvent    *EmittedEvent
}

func (d *MockAuditLog) SetError(e error) {
	d.Lock()
	d.returnError = e
	d.Unlock()
}

func (d *MockAuditLog) GetError() error {
	d.Lock()
	defer d.Unlock()
	return d.returnError
}

func (d *MockAuditLog) WaitForDelivery(context.Context) error {
	return nil
}

func (d *MockAuditLog) Close() error {
	return nil
}

// EmitAuditEventLegacy is a mock that records event and fields inside a struct.
func (d *MockAuditLog) EmitAuditEventLegacy(ev Event, fields EventFields) error {
	d.EmittedEvent = &EmittedEvent{ev, fields}
	return nil
}

func (d *MockAuditLog) UploadSessionRecording(SessionRecording) error {
	return nil
}

func (d *MockAuditLog) PostSessionSlice(slice SessionSlice) error {
	if err := d.GetError(); err != nil {
		d.FailedAttemptsC <- &slice
		return trace.Wrap(err)
	}
	d.SlicesC <- &slice
	return nil
}

func (d *MockAuditLog) GetSessionChunk(namespace string, sid session.ID, offsetBytes, maxBytes int) ([]byte, error) {
	return make([]byte, 0), nil
}

func (d *MockAuditLog) GetSessionEvents(namespace string, sid session.ID, after int, fetchPrintEvents bool) ([]EventFields, error) {
	return make([]EventFields, 0), nil
}

func (d *MockAuditLog) SearchEvents(fromUTC, toUTC time.Time, query string, limit int) ([]EventFields, error) {
	return make([]EventFields, 0), nil
}

func (d *MockAuditLog) SearchSessionEvents(fromUTC, toUTC time.Time, limit int) ([]EventFields, error) {
	return make([]EventFields, 0), nil
}

// Reset resets state to zero values.
func (d *MockAuditLog) Reset() {
	d.EmittedEvent = nil
}

// MockEmitter is emitter that stores last audit event
type MockEmitter struct {
	mtx       sync.RWMutex
	lastEvent AuditEvent
}

// CreateAuditStream creates a stream that discards all events
func (e *MockEmitter) CreateAuditStream(ctx context.Context, sid session.ID) (Stream, error) {
	return e, nil
}

// ResumeAuditStream resumes a stream that discards all events
func (e *MockEmitter) ResumeAuditStream(ctx context.Context, sid session.ID, uploadID string) (Stream, error) {
	return e, nil
}

// EmitAuditEvent emits audit event
func (e *MockEmitter) EmitAuditEvent(ctx context.Context, event AuditEvent) error {
	e.mtx.Lock()
	defer e.mtx.Unlock()
	e.lastEvent = event
	return nil
}

// LastEvent returns last emitted event
func (e *MockEmitter) LastEvent() AuditEvent {
	e.mtx.RLock()
	defer e.mtx.RUnlock()
	return e.lastEvent
}

// Reset resets state to zero values.
func (e *MockEmitter) Reset() {
	e.mtx.Lock()
	defer e.mtx.Unlock()
	e.lastEvent = nil
}

// Status returns a channel that always blocks
func (e *MockEmitter) Status() <-chan StreamStatus {
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
