/*
Copyright 2019 Gravitational, Inc.

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
	"encoding/json"
	"io"
	"time"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
)

// NewWriterLog returns a new instance of writer log
func NewWriterLog(w io.WriteCloser) *WriterLog {
	return &WriterLog{
		w:      w,
		clock:  clockwork.NewRealClock(),
		newUID: utils.NewRealUID(),
	}
}

// WriterLog is an audit log that emits all events
// to the external writer
type WriterLog struct {
	w     io.WriteCloser
	clock clockwork.Clock
	// newUID is used to generate unique IDs for events
	newUID utils.UID
}

// Close releases connection and resources associated with log if any
func (w *WriterLog) Close() error {
	return w.w.Close()
}

// EmitAuditEventLegacy emits audit event
func (w *WriterLog) EmitAuditEventLegacy(event Event, fields EventFields) error {
	err := UpdateEventFields(event, fields, w.clock, w.newUID)
	if err != nil {
		log.Error(err)
		// even in case of error, prefer to log incomplete event
		// rather than to log nothing
	}
	// line is the text to be logged
	line, err := json.Marshal(fields)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.w.Write(line)
	return trace.ConvertSystemError(err)
}

// DELETE IN: 2.7.0
// This method is no longer necessary as nodes and proxies >= 2.7.0
// use UploadSessionRecording method.
// PostSessionSlice sends chunks of recorded session to the event log
func (w *WriterLog) PostSessionSlice(SessionSlice) error {
	return trace.NotImplemented("not implemented")
}

// UploadSessionRecording uploads session recording to the audit server
func (w *WriterLog) UploadSessionRecording(r SessionRecording) error {
	return trace.NotImplemented("not implemented")
}

// GetSessionChunk returns a reader which can be used to read a byte stream
// of a recorded session starting from 'offsetBytes' (pass 0 to start from the
// beginning) up to maxBytes bytes.
//
// If maxBytes > MaxChunkBytes, it gets rounded down to MaxChunkBytes
func (w *WriterLog) GetSessionChunk(namespace string, sid session.ID, offsetBytes, maxBytes int) ([]byte, error) {
	return nil, trace.NotImplemented("not implemented")
}

// Returns all events that happen during a session sorted by time
// (oldest first).
//
// after tells to use only return events after a specified cursor Id
//
// This function is usually used in conjunction with GetSessionReader to
// replay recorded session streams.
func (w *WriterLog) GetSessionEvents(namespace string, sid session.ID, after int, includePrintEvents bool) ([]EventFields, error) {
	return nil, trace.NotImplemented("not implemented")
}

// SearchEvents is a flexible way to find events.
//
// Event types to filter can be specified and pagination is handled by an iterator key that allows
// a query to be resumed.
//
// The only mandatory requirement is a date range (UTC). Results must always
// show up sorted by date (newest first)
func (w *WriterLog) SearchEvents(fromUTC, toUTC time.Time, namespace string, eventTypes []string, limit int, order types.EventOrder, startKey string) (events []apievents.AuditEvent, lastKey string, err error) {
	return nil, "", trace.NotImplemented("not implemented")
}

// SearchSessionEvents is a flexible way to find session events.
// Only session.end events are returned by this function.
// This is used to find completed sessions.
//
// Event types to filter can be specified and pagination is handled by an iterator key that allows
// a query to be resumed.
func (w *WriterLog) SearchSessionEvents(fromUTC, toUTC time.Time, limit int, order types.EventOrder, startKey string, cond *types.WhereExpr) (events []apievents.AuditEvent, lastKey string, err error) {
	return nil, "", trace.NotImplemented("not implemented")
}

// WaitForDelivery waits for resources to be released and outstanding requests to
// complete after calling Close method
func (w *WriterLog) WaitForDelivery(context.Context) error {
	return nil
}

// StreamSessionEvents streams all events from a given session recording. An error is returned on the first
// channel if one is encountered. Otherwise it is simply closed when the stream ends.
// The event channel is not closed on error to prevent race conditions in downstream select statements.
func (w *WriterLog) StreamSessionEvents(ctx context.Context, sessionID session.ID, startIndex int64) (chan apievents.AuditEvent, chan error) {
	c, e := make(chan apievents.AuditEvent), make(chan error, 1)
	e <- trace.NotImplemented(loggerClosedMessage)
	return c, e
}
