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

package events

import (
	"context"
	"sync"
	"time"

	"github.com/gravitational/teleport/lib/session"

	"github.com/gravitational/trace"
)

func NewMockAuditLog(capacity int) *MockAuditLog {
	return &MockAuditLog{
		SlicesC:         make(chan *SessionSlice, capacity),
		FailedAttemptsC: make(chan *SessionSlice, capacity),
	}
}

// MockAuditLog is audit log used for tests
type MockAuditLog struct {
	sync.Mutex
	returnError     error
	FailedAttemptsC chan *SessionSlice
	SlicesC         chan *SessionSlice
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

func (d *MockAuditLog) EmitAuditEvent(event Event, fields EventFields) error {
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
