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

package events

import (
	"context"
	"io"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	"github.com/gravitational/teleport/api/internalutils/stream"
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
	return nil, "", trace.NotImplemented(writerCannotRead)
}

func (w *WriterLog) SearchUnstructuredEvents(ctx context.Context, req SearchEventsRequest) (events []*auditlogpb.EventUnstructured, lastKey string, err error) {
	return nil, "", trace.NotImplemented(writerCannotRead)
}

func (w *WriterLog) ExportUnstructuredEvents(ctx context.Context, req *auditlogpb.ExportUnstructuredEventsRequest) stream.Stream[*auditlogpb.ExportEventUnstructured] {
	return stream.Fail[*auditlogpb.ExportEventUnstructured](trace.NotImplemented(writerCannotRead))
}

func (w *WriterLog) GetEventExportChunks(ctx context.Context, req *auditlogpb.GetEventExportChunksRequest) stream.Stream[*auditlogpb.EventExportChunk] {
	return stream.Fail[*auditlogpb.EventExportChunk](trace.NotImplemented(writerCannotRead))
}

// SearchSessionEvents is a flexible way to find session events.
// Only session.end and windows.desktop.session.end events are returned by this function.
// This is used to find completed sessions.
//
// Event types to filter can be specified and pagination is handled by an iterator key that allows
// a query to be resumed.
func (w *WriterLog) SearchSessionEvents(ctx context.Context, req SearchSessionEventsRequest) (events []apievents.AuditEvent, lastKey string, err error) {
	return nil, "", trace.NotImplemented(writerCannotRead)
}

const writerCannotRead = "the primary audit log does not support reading; please check the Auth Server's audit_events_uri configuration"
