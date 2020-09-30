/*
Copyright 2018 Gravitational, Inc.

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
	"time"

	"github.com/gravitational/teleport/lib/session"

	"github.com/gravitational/trace"
)

// NewMultiLog returns a new instance of a multi logger
func NewMultiLog(loggers ...IAuditLog) *MultiLog {
	return &MultiLog{
		loggers: loggers,
	}
}

// MultiLog is a logger that fan outs write operations
// to all loggers, and performs all read and search operations
// on the first logger that implements the operation
type MultiLog struct {
	loggers []IAuditLog
}

// WaitForDelivery waits for resources to be released and outstanding requests to
// complete after calling Close method
func (m *MultiLog) WaitForDelivery(ctx context.Context) error {
	return nil
}

// Closer releases connections and resources associated with logs if any
func (m *MultiLog) Close() error {
	var errors []error
	for _, log := range m.loggers {
		errors = append(errors, log.Close())
	}
	return trace.NewAggregate(errors...)
}

// EmitAuditEventLegacy emits audit event
func (m *MultiLog) EmitAuditEventLegacy(event Event, fields EventFields) error {
	var errors []error
	for _, log := range m.loggers {
		errors = append(errors, log.EmitAuditEventLegacy(event, fields))
	}
	return trace.NewAggregate(errors...)
}

// UploadSessionRecording uploads session recording to the audit server
func (m *MultiLog) UploadSessionRecording(rec SessionRecording) error {
	var errors []error
	for _, log := range m.loggers {
		errors = append(errors, log.UploadSessionRecording(rec))
	}
	return trace.NewAggregate(errors...)
}

// DELETE IN: 2.7.0
// This method is no longer necessary as nodes and proxies >= 2.7.0
// use UploadSessionRecording method.
// PostSessionSlice sends chunks of recorded session to the event log
func (m *MultiLog) PostSessionSlice(slice SessionSlice) error {
	var errors []error
	for _, log := range m.loggers {
		errors = append(errors, log.PostSessionSlice(slice))
	}
	return trace.NewAggregate(errors...)
}

// GetSessionChunk returns a reader which can be used to read a byte stream
// of a recorded session starting from 'offsetBytes' (pass 0 to start from the
// beginning) up to maxBytes bytes.
//
// If maxBytes > MaxChunkBytes, it gets rounded down to MaxChunkBytes
func (m *MultiLog) GetSessionChunk(namespace string, sid session.ID, offsetBytes, maxBytes int) (data []byte, err error) {
	for _, log := range m.loggers {
		data, err = log.GetSessionChunk(namespace, sid, offsetBytes, maxBytes)
		if !trace.IsNotImplemented(err) {
			return data, err
		}
	}
	return data, err
}

// Returns all events that happen during a session sorted by time
// (oldest first).
//
// after tells to use only return events after a specified cursor Id
//
// This function is usually used in conjunction with GetSessionReader to
// replay recorded session streams.
func (m *MultiLog) GetSessionEvents(namespace string, sid session.ID, after int, fetchPrintEvents bool) (events []EventFields, err error) {
	for _, log := range m.loggers {
		events, err = log.GetSessionEvents(namespace, sid, after, fetchPrintEvents)
		if !trace.IsNotImplemented(err) {
			return events, err
		}
	}
	return events, err
}

// SearchEvents is a flexible way to find events. The format of a query string
// depends on the implementing backend. A recommended format is urlencoded
// (good enough for Lucene/Solr)
//
// Pagination is also defined via backend-specific query format.
//
// The only mandatory requirement is a date range (UTC). Results must always
// show up sorted by date (newest first)
func (m *MultiLog) SearchEvents(fromUTC, toUTC time.Time, query string, limit int) (events []EventFields, err error) {
	for _, log := range m.loggers {
		events, err = log.SearchEvents(fromUTC, toUTC, query, limit)
		if !trace.IsNotImplemented(err) {
			return events, err
		}
	}
	return events, err
}

// SearchSessionEvents returns session related events only. This is used to
// find completed session.
func (m *MultiLog) SearchSessionEvents(fromUTC, toUTC time.Time, limit int) (events []EventFields, err error) {
	for _, log := range m.loggers {
		events, err = log.SearchSessionEvents(fromUTC, toUTC, limit)
		if !trace.IsNotImplemented(err) {
			return events, err
		}
	}
	return events, err
}
