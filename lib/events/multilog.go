/*
Copyright 2018-2020 Gravitational, Inc.

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

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"

	"github.com/gravitational/trace"
)

// NewMultiLog returns a new instance of a multi logger
func NewMultiLog(loggers ...IAuditLog) (*MultiLog, error) {
	emitters := make([]apievents.Emitter, 0, len(loggers))
	for _, logger := range loggers {
		emitter, ok := logger.(apievents.Emitter)
		if !ok {
			return nil, trace.BadParameter("expected emitter, got %T", logger)
		}
		emitters = append(emitters, emitter)
	}
	return &MultiLog{
		MultiEmitter: NewMultiEmitter(emitters...),
		loggers:      loggers,
	}, nil
}

// MultiLog is a logger that fan outs write operations
// to all loggers, and performs all read and search operations
// on the first logger that implements the operation
type MultiLog struct {
	loggers []IAuditLog
	*MultiEmitter
}

// WaitForDelivery waits for resources to be released and outstanding requests to
// complete after calling Close method
func (m *MultiLog) WaitForDelivery(ctx context.Context) error {
	return nil
}

// Close releases connections and resources associated with logs if any
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

// SearchEvents is a flexible way to find events.
//
// Event types to filter can be specified and pagination is handled by an iterator key that allows
// a query to be resumed.
//
// The only mandatory requirement is a date range (UTC).
//
// This function may never return more than 1 MiB of event data.
func (m *MultiLog) SearchEvents(fromUTC, toUTC time.Time, namespace string, eventTypes []string, limit int, order types.EventOrder, startKey string) (events []apievents.AuditEvent, lastKey string, err error) {
	for _, log := range m.loggers {
		events, lastKey, err := log.SearchEvents(fromUTC, toUTC, namespace, eventTypes, limit, order, startKey)
		if !trace.IsNotImplemented(err) {
			return events, lastKey, err
		}
	}
	return events, lastKey, err
}

// SearchSessionEvents is a flexible way to find session events.
// Only session.end events are returned by this function.
// This is used to find completed sessions.
//
// Event types to filter can be specified and pagination is handled by an iterator key that allows
// a query to be resumed.
func (m *MultiLog) SearchSessionEvents(fromUTC, toUTC time.Time, limit int, order types.EventOrder, startKey string, cond *types.WhereExpr) (events []apievents.AuditEvent, lastKey string, err error) {
	for _, log := range m.loggers {
		events, lastKey, err = log.SearchSessionEvents(fromUTC, toUTC, limit, order, startKey, cond)
		if !trace.IsNotImplemented(err) {
			return events, lastKey, err
		}
	}
	return events, lastKey, err
}

// StreamSessionEvents streams all events from a given session recording. An error is returned on the first
// channel if one is encountered. Otherwise it is simply closed when the stream ends.
// The event channel is not closed on error to prevent race conditions in downstream select statements.
func (m *MultiLog) StreamSessionEvents(ctx context.Context, sessionID session.ID, startIndex int64) (chan apievents.AuditEvent, chan error) {
	c, e := make(chan apievents.AuditEvent), make(chan error, 1)

	go func() {
	loggers:
		for _, log := range m.loggers {
			subCh, subErrCh := log.StreamSessionEvents(ctx, sessionID, startIndex)

			for {
				select {
				case event, more := <-subCh:
					if !more {
						close(c)
						return
					}

					c <- event
				case err := <-subErrCh:
					if !trace.IsNotImplemented(err) {
						e <- trace.Wrap(err)
						return
					}

					continue loggers
				}
			}
		}
	}()

	return c, e
}
