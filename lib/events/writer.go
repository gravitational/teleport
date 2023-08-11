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
	"io"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/utils"
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

// SearchEvents is a flexible way to find events.
//
// Event types to filter can be specified and pagination is handled by an iterator key that allows
// a query to be resumed.
//
// The only mandatory requirement is a date range (UTC). Results must always
// show up sorted by date (newest first)
func (w *WriterLog) SearchEvents(ctx context.Context, req SearchEventsRequest) (events []apievents.AuditEvent, lastKey string, err error) {
	return nil, "", trace.NotImplemented("not implemented")
}

// SearchSessionEvents is a flexible way to find session events.
// Only session.end and windows.desktop.session.end events are returned by this function.
// This is used to find completed sessions.
//
// Event types to filter can be specified and pagination is handled by an iterator key that allows
// a query to be resumed.
func (w *WriterLog) SearchSessionEvents(ctx context.Context, req SearchSessionEventsRequest) (events []apievents.AuditEvent, lastKey string, err error) {
	return nil, "", trace.NotImplemented("not implemented")
}
