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
	"log/slog"
	"sync/atomic"

	"github.com/gravitational/trace"

	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	"github.com/gravitational/teleport/api/internalutils/stream"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// DiscardAuditLog is do-nothing, discard-everything implementation
// of IAuditLog interface used for cases when audit is turned off
type DiscardAuditLog struct{}

// NewDiscardAuditLog returns a no-op audit log instance
func NewDiscardAuditLog() *DiscardAuditLog {
	return &DiscardAuditLog{}
}

func (d *DiscardAuditLog) Close() error {
	return nil
}

func (d *DiscardAuditLog) SearchEvents(ctx context.Context, req SearchEventsRequest) ([]apievents.AuditEvent, string, error) {
	return make([]apievents.AuditEvent, 0), "", nil
}

func (d *DiscardAuditLog) SearchSessionEvents(ctx context.Context, req SearchSessionEventsRequest) ([]apievents.AuditEvent, string, error) {
	return make([]apievents.AuditEvent, 0), "", nil
}

func (d *DiscardAuditLog) SearchUnstructuredEvents(ctx context.Context, req SearchEventsRequest) ([]*auditlogpb.EventUnstructured, string, error) {
	return make([]*auditlogpb.EventUnstructured, 0), "", nil
}

func (d *DiscardAuditLog) ExportUnstructuredEvents(ctx context.Context, req *auditlogpb.ExportUnstructuredEventsRequest) stream.Stream[*auditlogpb.ExportEventUnstructured] {
	return stream.Empty[*auditlogpb.ExportEventUnstructured]()
}

func (d *DiscardAuditLog) GetEventExportChunks(ctx context.Context, req *auditlogpb.GetEventExportChunksRequest) stream.Stream[*auditlogpb.EventExportChunk] {
	return stream.Empty[*auditlogpb.EventExportChunk]()
}

func (d *DiscardAuditLog) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	return nil
}

func (d *DiscardAuditLog) StreamSessionEvents(ctx context.Context, sessionID session.ID, startIndex int64) (chan apievents.AuditEvent, chan error) {
	c, e := make(chan apievents.AuditEvent), make(chan error, 1)
	close(c)
	return c, e
}

// NewDiscardRecorder returns a [SessionRecorderChecker] that discards events.
func NewDiscardRecorder() *DiscardRecorder {
	return &DiscardRecorder{
		done: make(chan struct{}),
	}
}

// DiscardRecorder returns a stream that discards all events
type DiscardRecorder struct {
	completed atomic.Bool
	done      chan struct{}
}

// Write discards data
func (d *DiscardRecorder) Write(p []byte) (n int, err error) {
	if d.completed.Load() {
		return 0, trace.BadParameter("stream is closed")
	}

	return len(p), nil
}

// Status returns a channel that always blocks
func (*DiscardRecorder) Status() <-chan apievents.StreamStatus {
	return nil
}

// Done returns channel closed when streamer is closed
// should be used to detect sending errors
func (d *DiscardRecorder) Done() <-chan struct{} {
	return d.done
}

// Close flushes non-uploaded flight stream data without marking
// the stream completed and closes the stream instance
func (d *DiscardRecorder) Close(ctx context.Context) error {
	return d.Complete(ctx)
}

// Complete marks the stream as closed
func (d *DiscardRecorder) Complete(ctx context.Context) error {
	if !d.completed.CompareAndSwap(false, true) {
		close(d.done)
	}
	return nil
}

// RecordEvent discards session event
func (d *DiscardRecorder) RecordEvent(ctx context.Context, pe apievents.PreparedSessionEvent) error {
	if d.completed.Load() {
		return trace.BadParameter("stream is closed")
	}
	event := pe.GetAuditEvent()

	slog.Log(ctx, logutils.TraceLevel, "Discarding stream event",
		"event_id", event.GetID(),
		"event_type", event.GetType(),
		"event_time", event.GetTime(),
		"event_index", event.GetIndex(),
	)
	return nil
}

// NewDiscardEmitter returns a no-op discard emitter
func NewDiscardEmitter() *DiscardEmitter {
	return &DiscardEmitter{}
}

// DiscardEmitter discards all events
type DiscardEmitter struct{}

// EmitAuditEvent discards audit event
func (*DiscardEmitter) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	slog.DebugContext(ctx, "Discarding event",
		"event_id", event.GetID(),
		"event_type", event.GetType(),
		"event_time", event.GetTime(),
		"event_index", event.GetIndex(),
	)

	return nil
}

// NewDiscardStreamer returns a streamer that creates streams that
// discard events
func NewDiscardStreamer() *DiscardStreamer {
	return &DiscardStreamer{}
}

// DiscardStreamer creates DiscardRecorders
type DiscardStreamer struct{}

// CreateAuditStream creates a stream that discards all events
func (*DiscardStreamer) CreateAuditStream(ctx context.Context, sid session.ID) (apievents.Stream, error) {
	return NewDiscardRecorder(), nil
}

// ResumeAuditStream resumes a stream that discards all events
func (*DiscardStreamer) ResumeAuditStream(ctx context.Context, sid session.ID, uploadID string) (apievents.Stream, error) {
	return NewDiscardRecorder(), nil
}

// NoOpPreparer is a SessionEventPreparer that doesn't change events
type NoOpPreparer struct{}

// PrepareSessionEvent returns the event unchanged
func (m *NoOpPreparer) PrepareSessionEvent(event apievents.AuditEvent) (apievents.PreparedSessionEvent, error) {
	return preparedSessionEvent{
		event: event,
	}, nil
}

// WithNoOpPreparer wraps rec with a SessionEventPreparer that will leave
// events unchanged
func WithNoOpPreparer(rec SessionRecorder) SessionPreparerRecorder {
	return NewSessionPreparerRecorder(&NoOpPreparer{}, rec)
}
