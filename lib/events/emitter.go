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
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
)

// AsyncBufferSize is a default buffer size for async emitters
const AsyncBufferSize = 1024

// AsyncEmitterConfig provides parameters for emitter
type AsyncEmitterConfig struct {
	// Inner emits events to the underlying store
	Inner apievents.Emitter
	// BufferSize is a default buffer size for emitter
	BufferSize int
}

// CheckAndSetDefaults checks and sets default values
func (c *AsyncEmitterConfig) CheckAndSetDefaults() error {
	if c.Inner == nil {
		return trace.BadParameter("missing parameter Inner")
	}
	if c.BufferSize == 0 {
		c.BufferSize = AsyncBufferSize
	}
	return nil
}

// NewAsyncEmitter returns emitter that submits events
// without blocking the caller. It will start losing events
// on buffer overflow.
func NewAsyncEmitter(cfg AsyncEmitterConfig) (*AsyncEmitter, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	a := &AsyncEmitter{
		cancel:   cancel,
		ctx:      ctx,
		eventsCh: make(chan apievents.AuditEvent, cfg.BufferSize),
		cfg:      cfg,
	}
	go a.forward()
	return a, nil
}

// AsyncEmitter accepts events to a buffered channel and emits
// events in a separate goroutine without blocking the caller.
type AsyncEmitter struct {
	cfg      AsyncEmitterConfig
	eventsCh chan apievents.AuditEvent
	cancel   context.CancelFunc
	ctx      context.Context
}

// Close closes emitter and cancels all in flight events.
func (a *AsyncEmitter) Close() error {
	a.cancel()
	return nil
}

func (a *AsyncEmitter) forward() {
	for {
		select {
		case <-a.ctx.Done():
			return
		case event := <-a.eventsCh:
			err := a.cfg.Inner.EmitAuditEvent(a.ctx, event)
			if err != nil {
				if a.ctx.Err() != nil {
					return
				}
				slog.ErrorContext(a.ctx, "Failed to emit audit event.", "error", err)
			}
		}
	}
}

// EmitAuditEvent emits audit event without blocking the caller. It will start
// losing events on buffer overflow, but it never fails.
func (a *AsyncEmitter) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	select {
	case a.eventsCh <- event:
		return nil
	case <-ctx.Done():
		return trace.ConnectionProblem(ctx.Err(), "context canceled or closed")
	default:
		slog.ErrorContext(ctx, "Failed to emit audit event. This server's connection to the auth service appears to be slow.", "event_type", event.GetType(), "event_code", event.GetCode())
		return nil
	}
}

// CheckingEmitterConfig provides parameters for emitter
type CheckingEmitterConfig struct {
	// Inner emits events to the underlying store
	Inner apievents.Emitter
	// Clock is a clock interface, used in tests
	Clock clockwork.Clock
	// UIDGenerator is unique ID generator
	UIDGenerator utils.UID
	// ClusterName specifies the name of this teleport cluster
	// as configured on the auth server
	ClusterName string
}

// NewCheckingEmitter returns emitter that checks
// that all required fields are properly set
func NewCheckingEmitter(cfg CheckingEmitterConfig) (*CheckingEmitter, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &CheckingEmitter{
		CheckingEmitterConfig: cfg,
	}, nil
}

// CheckingEmitter ensures that event fields have been set properly
// and reports statistics for every wrapper
type CheckingEmitter struct {
	CheckingEmitterConfig
}

// CheckAndSetDefaults checks and sets default values
func (w *CheckingEmitterConfig) CheckAndSetDefaults() error {
	if w.Inner == nil {
		return trace.BadParameter("missing parameter Inner")
	}
	if w.Clock == nil {
		w.Clock = clockwork.NewRealClock()
	}
	if w.UIDGenerator == nil {
		w.UIDGenerator = utils.NewRealUID()
	}
	return nil
}

// EmitAuditEvent emits audit event
func (r *CheckingEmitter) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	ctx = context.WithoutCancel(ctx)
	auditEmitEvent.Inc()
	auditEmitEventSizes.Observe(float64(event.Size()))
	if err := checkAndSetEventFields(event, r.Clock, r.UIDGenerator, r.ClusterName); err != nil {
		slog.ErrorContext(ctx, "Failed to emit audit event.", "error", err)
		AuditFailedEmit.Inc()
		return trace.Wrap(err)
	}
	if err := r.Inner.EmitAuditEvent(ctx, event); err != nil {
		AuditFailedEmit.Inc()
		slog.ErrorContext(ctx, "Failed to emit audit event of type", "event_type", event.GetType(), "error", err)
		return trace.Wrap(err)
	}
	return nil
}

// checkAndSetEventFields updates passed event fields with additional information
// common for all event types such as unique IDs, timestamps, codes, etc.
//
// This method is a "final stop" for various audit log implementations for
// updating event fields before it gets persisted in the backend.
func checkAndSetEventFields(event apievents.AuditEvent, clock clockwork.Clock, uid utils.UID, clusterName string) error {
	if err := checkBasicEventFields(event); err != nil {
		return trace.Wrap(err)
	}
	if event.GetID() == "" && event.GetType() != SessionPrintEvent && event.GetType() != DesktopRecordingEvent {
		event.SetID(uid.New())
	}
	if event.GetTime().IsZero() {
		event.SetTime(clock.Now().UTC().Round(time.Millisecond))
	}
	if event.GetClusterName() == "" {
		event.SetClusterName(clusterName)
	}
	return nil
}

func checkBasicEventFields(event apievents.AuditEvent) error {
	if event.GetType() == "" {
		return trace.BadParameter("missing mandatory event type field")
	}
	if event.GetCode() == "" && event.GetType() != SessionPrintEvent && event.GetType() != DesktopRecordingEvent {
		return trace.BadParameter("missing mandatory event code field for %v event", event.GetType())
	}
	return nil
}

// NewWriterEmitter returns a new instance of emitter writing to writer
func NewWriterEmitter(w io.WriteCloser) *WriterEmitter {
	return &WriterEmitter{
		w:         w,
		WriterLog: NewWriterLog(w),
	}
}

// WriterEmitter is an emitter that emits all events
// to the external writer
type WriterEmitter struct {
	w io.WriteCloser
	*WriterLog
}

// Close closes the underlying io.WriteCloser passed in NewWriterEmitter
func (w *WriterEmitter) Close() error {
	return trace.NewAggregate(
		w.w.Close(),
		w.WriterLog.Close())
}

// EmitAuditEvent writes the event to the writer
func (w *WriterEmitter) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	// line is the text to be logged
	line, err := utils.FastMarshal(event)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = fmt.Fprintln(w.w, string(line))
	return trace.ConvertSystemError(err)
}

// NewLoggingEmitter returns an emitter that logs all events to the console
// with the info level. Events are only logged for self-hosted installations,
// Teleport Cloud treats this as a no-op.
func NewLoggingEmitter(cloud bool) *LoggingEmitter {
	return &LoggingEmitter{
		emit: !(modules.GetModules().Features().Cloud || cloud),
	}
}

// LoggingEmitter logs all events with info level
type LoggingEmitter struct {
	emit bool
}

// EmitAuditEvent logs audit event, skips session print events, session
// disk events and app session request events, because they are very verbose.
func (l *LoggingEmitter) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	if !l.emit {
		return nil
	}

	switch event.GetType() {
	case ResizeEvent, SessionDiskEvent, SessionPrintEvent, AppSessionRequestEvent, "":
		return nil
	}

	data, err := utils.FastMarshal(event)
	if err != nil {
		return trace.Wrap(err)
	}

	var fields map[string]any
	if err := utils.FastUnmarshal(data, &fields); err != nil {
		return trace.Wrap(err)
	}
	fields[teleport.ComponentKey] = teleport.ComponentAuditLog

	slog.InfoContext(ctx, "emitting audit event", "event_type", event.GetType(), "fields", fields)
	return nil
}

// NewMultiEmitter returns emitter that writes
// events to all emitters
func NewMultiEmitter(emitters ...apievents.Emitter) *MultiEmitter {
	return &MultiEmitter{
		emitters: emitters,
	}
}

// MultiEmitter writes audit events to multiple emitters
type MultiEmitter struct {
	emitters []apievents.Emitter
}

// EmitAuditEvent emits audit event to all emitters
func (m *MultiEmitter) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	ctx = context.WithoutCancel(ctx)
	var errors []error
	for i := range m.emitters {
		err := m.emitters[i].EmitAuditEvent(ctx, event)
		if err != nil {
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}

// StreamerAndEmitter combines streamer and emitter to create stream emitter
type StreamerAndEmitter struct {
	Streamer
	apievents.Emitter
}

// NewCallbackEmitter returns an emitter that invokes a callback on every
// action, is used in tests to inject failures
func NewCallbackEmitter(cfg CallbackEmitterConfig) (*CallbackEmitter, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &CallbackEmitter{
		CallbackEmitterConfig: cfg,
	}, nil
}

// CallbackEmitterConfig provides parameters for emitter
type CallbackEmitterConfig struct {
	// OnEmitAuditEvent is called on emit audit event on a stream
	OnEmitAuditEvent func(ctx context.Context, event apievents.AuditEvent) error
}

// CheckAndSetDefaults checks and sets default values
func (c *CallbackEmitterConfig) CheckAndSetDefaults() error {
	if c.OnEmitAuditEvent == nil {
		return trace.BadParameter("missing parameter OnEmitAuditEvent")
	}
	return nil
}

// CallbackEmitter invokes a callback on every action, is used in tests to inject failures
type CallbackEmitter struct {
	CallbackEmitterConfig
}

func (c *CallbackEmitter) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	ctx = context.WithoutCancel(ctx)
	return c.OnEmitAuditEvent(ctx, event)
}

// NewCallbackStreamer returns streamer that invokes callback on every
// action, is used in tests to inject failures
func NewCallbackStreamer(cfg CallbackStreamerConfig) (*CallbackStreamer, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &CallbackStreamer{
		CallbackStreamerConfig: cfg,
	}, nil
}

// CallbackStreamerConfig provides parameters for streamer
type CallbackStreamerConfig struct {
	// Inner emits events to the underlying store
	Inner Streamer
	// OnCreateAuditStream is called on create audit stream
	OnCreateAuditStream func(ctx context.Context, sid session.ID, inner Streamer) (apievents.Stream, error)
	// OnResumeAuditStream is called on resuming audit stream
	OnResumeAuditStream func(ctx context.Context, sid session.ID, uploadID string, inner Streamer) (apievents.Stream, error)
	// OnRecordEvent is called on emit audit event on a stream
	OnRecordEvent func(ctx context.Context, sid session.ID, event apievents.PreparedSessionEvent) error
}

// CheckAndSetDefaults checks and sets default values
func (c *CallbackStreamerConfig) CheckAndSetDefaults() error {
	if c.Inner == nil {
		return trace.BadParameter("missing parameter Inner")
	}
	return nil
}

// CallbackStreamer ensures that event fields have been set properly
// and reports statistics for every wrapper
type CallbackStreamer struct {
	CallbackStreamerConfig
}

// CreateAuditStream creates audit event stream
func (s *CallbackStreamer) CreateAuditStream(ctx context.Context, sid session.ID) (apievents.Stream, error) {
	var stream apievents.Stream
	var err error
	if s.OnCreateAuditStream != nil {
		stream, err = s.OnCreateAuditStream(ctx, sid, s.Inner)
	} else {
		stream, err = s.Inner.CreateAuditStream(ctx, sid)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &CallbackStream{
		stream:   stream,
		streamer: s,
	}, nil
}

// ResumeAuditStream resumes audit event stream
func (s *CallbackStreamer) ResumeAuditStream(ctx context.Context, sid session.ID, uploadID string) (apievents.Stream, error) {
	var stream apievents.Stream
	var err error
	if s.OnResumeAuditStream != nil {
		stream, err = s.OnResumeAuditStream(ctx, sid, uploadID, s.Inner)
	} else {
		stream, err = s.Inner.ResumeAuditStream(ctx, sid, uploadID)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &CallbackStream{
		stream:    stream,
		sessionID: sid,
		streamer:  s,
	}, nil
}

// CallbackStream call
type CallbackStream struct {
	stream    apievents.Stream
	sessionID session.ID
	streamer  *CallbackStreamer
}

// Close flushes non-uploaded flight stream data without marking
// the stream completed and closes the stream instance
func (s *CallbackStream) Close(ctx context.Context) error {
	return s.stream.Close(ctx)
}

// Done returns channel closed when streamer is closed
// should be used to detect sending errors
func (s *CallbackStream) Done() <-chan struct{} {
	return s.stream.Done()
}

// Status returns channel receiving updates about stream status
// last event index that was uploaded and upload ID
func (s *CallbackStream) Status() <-chan apievents.StreamStatus {
	return s.stream.Status()
}

// Complete closes the stream and marks it finalized
func (s *CallbackStream) Complete(ctx context.Context) error {
	return s.stream.Complete(ctx)
}

// RecordEvent records a session event
func (s *CallbackStream) RecordEvent(ctx context.Context, event apievents.PreparedSessionEvent) error {
	if s.streamer.OnRecordEvent != nil {
		if err := s.streamer.OnRecordEvent(ctx, s.sessionID, event); err != nil {
			return trace.Wrap(err)
		}
	}
	return s.stream.RecordEvent(ctx, event)
}

// NewReportingStreamer reports upload events
// to the eventsC channel, if the channel is not nil.
func NewReportingStreamer(streamer Streamer, eventsC chan UploadEvent) *ReportingStreamer {
	return &ReportingStreamer{
		streamer: streamer,
		eventsC:  eventsC,
	}
}

// ReportingStreamer  reports upload events
// to the eventsC channel, if the channel is not nil.
type ReportingStreamer struct {
	streamer Streamer
	eventsC  chan UploadEvent
}

// CreateAuditStream creates audit event stream
func (s *ReportingStreamer) CreateAuditStream(ctx context.Context, sid session.ID) (apievents.Stream, error) {
	stream, err := s.streamer.CreateAuditStream(ctx, sid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ReportingStream{
		Stream:    stream,
		eventsC:   s.eventsC,
		sessionID: sid,
	}, nil
}

// ResumeAuditStream resumes audit event stream
func (s *ReportingStreamer) ResumeAuditStream(ctx context.Context, sid session.ID, uploadID string) (apievents.Stream, error) {
	stream, err := s.streamer.ResumeAuditStream(ctx, sid, uploadID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ReportingStream{
		Stream:    stream,
		sessionID: sid,
		eventsC:   s.eventsC,
	}, nil
}

// ReportingStream reports status of uploads to the events channel
type ReportingStream struct {
	apievents.Stream
	sessionID session.ID
	eventsC   chan UploadEvent
}

// Complete closes the stream and marks it finalized
func (s *ReportingStream) Complete(ctx context.Context) error {
	err := s.Stream.Complete(ctx)
	if s.eventsC == nil {
		return trace.Wrap(err)
	}
	select {
	case s.eventsC <- UploadEvent{
		SessionID: string(s.sessionID),
		Error:     err,
	}:
	default:
		slog.WarnContext(ctx, "Skip send event on a blocked channel.")
	}
	return trace.Wrap(err)
}
