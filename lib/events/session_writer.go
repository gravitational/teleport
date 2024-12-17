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
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/session"
)

// NewSessionWriter returns a new instance of session writer
func NewSessionWriter(cfg SessionWriterConfig) (*SessionWriter, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	stream, err := cfg.Streamer.CreateAuditStream(cfg.Context, cfg.SessionID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(cfg.Context)
	writer := &SessionWriter{
		cfg:      cfg,
		stream:   stream,
		log:      slog.With(teleport.ComponentKey, cfg.Component),
		cancel:   cancel,
		closeCtx: ctx,
		eventsCh: make(chan apievents.PreparedSessionEvent),
		doneCh:   make(chan struct{}),
	}
	go func() {
		writer.processEvents()
		close(writer.doneCh)
	}()

	return writer, nil
}

// SessionWriterConfig configures session writer
type SessionWriterConfig struct {
	// SessionID defines the session to record.
	SessionID session.ID

	// Component is a component used for logging
	Component string

	// MakeEvents converts bytes written via the io.Writer interface
	// into AuditEvents that are written to the stream.
	// For backwards compatibility, SessionWriter will convert bytes to
	// SessionPrint events when MakeEvents is not provided.
	MakeEvents func([]byte) []apievents.AuditEvent

	// Preparer will set necessary fields of events created by Write.
	Preparer SessionEventPreparer

	// Streamer is used to create and resume audit streams
	Streamer Streamer

	// Context is a context to cancel the writes
	// or any other operations
	Context context.Context

	// Clock is used to override time in tests
	Clock clockwork.Clock

	// BackoffTimeout is a backoff timeout
	// if set, failed audit write events will be lost
	// if session writer fails to write events after this timeout
	BackoffTimeout time.Duration

	// BackoffDuration is a duration of the backoff before the next try
	BackoffDuration time.Duration
}

// CheckAndSetDefaults checks and sets defaults
func (cfg *SessionWriterConfig) CheckAndSetDefaults() error {
	if cfg.SessionID.IsZero() {
		return trace.BadParameter("session writer config: missing parameter SessionID")
	}
	if cfg.Streamer == nil {
		return trace.BadParameter("session writer config: missing parameter Streamer")
	}
	if cfg.Preparer == nil {
		return trace.BadParameter("session writer config: missing parameter Preparer")
	}
	if cfg.Context == nil {
		return trace.BadParameter("session writer config: missing parameter Context")
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.BackoffTimeout == 0 {
		cfg.BackoffTimeout = AuditBackoffTimeout
	}
	if cfg.BackoffDuration == 0 {
		cfg.BackoffDuration = NetworkBackoffDuration
	}
	if cfg.MakeEvents == nil {
		cfg.MakeEvents = bytesToSessionPrintEvents
	}
	return nil
}

func bytesToSessionPrintEvents(b []byte) []apievents.AuditEvent {
	start := time.Now().UTC().Round(time.Millisecond)
	var result []apievents.AuditEvent
	for len(b) != 0 {
		printEvent := &apievents.SessionPrint{
			Metadata: apievents.Metadata{
				Type: SessionPrintEvent,
				Time: start,
			},
			Data: b,
		}
		if printEvent.Size() > MaxProtoMessageSizeBytes {
			extraBytes := printEvent.Size() - MaxProtoMessageSizeBytes
			printEvent.Data = b[:extraBytes]
			printEvent.Bytes = int64(len(printEvent.Data))
			b = b[extraBytes:]
		} else {
			printEvent.Bytes = int64(len(printEvent.Data))
			b = nil
		}
		result = append(result, printEvent)
	}
	return result
}

// SessionWriter wraps session stream and writes session events to it
type SessionWriter struct {
	mtx        sync.Mutex
	cfg        SessionWriterConfig
	log        *slog.Logger
	buffer     []apievents.PreparedSessionEvent
	eventsCh   chan apievents.PreparedSessionEvent
	lastStatus *apievents.StreamStatus
	stream     apievents.Stream
	cancel     context.CancelFunc
	closeCtx   context.Context
	// doneCh is closed when all internal processes have exited
	doneCh chan struct{}

	backoffUntil   time.Time
	lostEvents     atomic.Int64
	acceptedEvents atomic.Int64
	slowWrites     atomic.Int64
}

// SessionWriterStats provides stats about lost events and slow writes
type SessionWriterStats struct {
	// AcceptedEvents is a total amount of events accepted for writes
	AcceptedEvents int64
	// LostEvents provides stats about lost events due to timeouts
	LostEvents int64
	// SlowWrites is a stat about how many times
	// events could not be written right away. It is a noisy
	// metric, so only used in debug modes.
	SlowWrites int64
}

// Status returns channel receiving updates about stream status
// last event index that was uploaded and upload ID
func (a *SessionWriter) Status() <-chan apievents.StreamStatus {
	return nil
}

// Done returns channel closed when streamer is closed
// should be used to detect sending errors
func (a *SessionWriter) Done() <-chan struct{} {
	return a.closeCtx.Done()
}

// PrepareSessionEvent will set necessary event fields for session-related
// events and must be called before the event is recorded, regardless
// of whether the event will be recorded, emitted, or both.
func (a *SessionWriter) PrepareSessionEvent(event apievents.AuditEvent) (apievents.PreparedSessionEvent, error) {
	return a.cfg.Preparer.PrepareSessionEvent(event)
}

// Write takes a chunk and writes it into the audit log
func (a *SessionWriter) Write(data []byte) (int, error) {
	// buffer is copied here to prevent data corruption:
	// io.Copy allocates single buffer and calls multiple writes in a loop
	// Write is async, this can lead to cases when the buffer is re-used
	// and data is corrupted unless we copy the data buffer in the first place
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	events := a.cfg.MakeEvents(dataCopy)
	for _, event := range events {
		event, err := a.cfg.Preparer.PrepareSessionEvent(event)
		if err != nil {
			a.log.ErrorContext(a.closeCtx, "failed to setup event", "error", err, "event", event.GetAuditEvent().GetType())
		}
		if err := a.RecordEvent(a.cfg.Context, event); err != nil {
			a.log.ErrorContext(a.closeCtx, "failed to emit event", "error", err, "event", event.GetAuditEvent().GetType())
			return 0, trace.Wrap(err)
		}
	}

	return len(data), nil
}

// checkAndResetBackoff checks whether the backoff is in place,
// also resets it if the time has passed. If the state is backoff,
// returns true
func (a *SessionWriter) checkAndResetBackoff(now time.Time) bool {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	switch {
	case a.backoffUntil.IsZero():
		// backoff is not set
		return false
	case a.backoffUntil.After(now):
		// backoff has not expired yet
		return true
	default:
		// backoff has expired
		a.backoffUntil = time.Time{}
		return false
	}
}

// maybeSetBackoff sets backoff if it's not already set.
// Does not overwrite backoff time to avoid concurrent calls
// overriding the backoff timer.
//
// Returns true if this call sets the backoff.
func (a *SessionWriter) maybeSetBackoff(backoffUntil time.Time) bool {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	switch {
	case !a.backoffUntil.IsZero():
		return false
	default:
		a.backoffUntil = backoffUntil
		return true
	}
}

// RecordEvent emits audit event
func (a *SessionWriter) RecordEvent(ctx context.Context, pe apievents.PreparedSessionEvent) error {
	event := pe.GetAuditEvent()
	if err := checkBasicEventFields(event); err != nil {
		return trace.Wrap(err)
	}

	// the index starts at 0, so we'll only know if the index is invalid
	// if we've seen at least one event already
	if event.GetIndex() == 0 && a.acceptedEvents.Load() > 0 {
		return trace.BadParameter("missing mandatory event index field")
	}

	a.acceptedEvents.Add(1)

	// Without serialization, RecordEvent will call grpc's method directly.
	// When BPF callback is emitting events concurrently with session data to the grpc stream,
	// it becomes deadlocked (not just blocked temporarily, but permanently)
	// in flowcontrol.go, trying to get quota:
	// https://github.com/grpc/grpc-go/blob/a906ca0441ceb1f7cd4f5c7de30b8e81ce2ff5e8/internal/transport/flowcontrol.go#L60

	// If backoff is in effect, lose event, return right away
	if isBackoff := a.checkAndResetBackoff(a.cfg.Clock.Now()); isBackoff {
		a.lostEvents.Add(1)
		return nil
	}

	// This fast path will be used all the time during normal operation.
	select {
	case a.eventsCh <- pe:
		return nil
	case <-ctx.Done():
		return trace.ConnectionProblem(ctx.Err(), "context canceled or timed out")
	case <-a.closeCtx.Done():
		return trace.ConnectionProblem(a.closeCtx.Err(), "session writer is closed")
	default:
		a.slowWrites.Add(1)
	}

	// Channel is blocked.
	//
	// Try slower write with the timeout, and initiate backoff
	// if unsuccessful.
	//
	// Code borrows logic from this commit by rsc:
	//
	// https://github.com/rsc/kubernetes/commit/6a19e46ed69a62a6d10b5092b179ef517aee65f8#diff-b1da25b7ac375964cd28c5f8cf5f1a2e37b6ec72a48ac0dd3e4b80f38a2e8e1e
	//
	// Block sending with a timeout. Reuse timers
	// to avoid allocating on high frequency calls.
	//
	t, ok := timerPool.Get().(*time.Timer)
	if ok {
		// Reset should be only invoked on stopped or expired
		// timers with drained buffered channels.
		//
		// See the logic below, the timer is only placed in the pool when in
		// stopped state with drained channel.
		//
		t.Reset(a.cfg.BackoffTimeout)
	} else {
		t = time.NewTimer(a.cfg.BackoffTimeout)
	}
	defer timerPool.Put(t)

	select {
	case a.eventsCh <- pe:
		stopped := t.Stop()
		if !stopped {
			// Here and below, consume triggered (but not yet received) timer event
			// so that future reuse does not get a spurious timeout.
			// This code is only safe because <- t.C is in the same
			// event loop and can't happen in parallel.
			<-t.C
		}
		return nil
	case <-t.C:
		if setBackoff := a.maybeSetBackoff(a.cfg.Clock.Now().UTC().Add(a.cfg.BackoffDuration)); setBackoff {
			a.log.ErrorContext(ctx, "Audit write timed out. Will be losing events while applying backogg.", "timeout", a.cfg.BackoffTimeout, "backoff_duration", a.cfg.BackoffDuration)
		}
		a.lostEvents.Add(1)
		return nil
	case <-ctx.Done():
		a.lostEvents.Add(1)
		stopped := t.Stop()
		if !stopped {
			<-t.C
		}
		return trace.ConnectionProblem(ctx.Err(), "context canceled or timed out")
	case <-a.closeCtx.Done():
		a.lostEvents.Add(1)
		stopped := t.Stop()
		if !stopped {
			<-t.C
		}
		return trace.ConnectionProblem(a.closeCtx.Err(), "writer is closed")
	}
}

var timerPool sync.Pool

// Stats returns up to date stats from this session writer
func (a *SessionWriter) Stats() SessionWriterStats {
	return SessionWriterStats{
		AcceptedEvents: a.acceptedEvents.Load(),
		LostEvents:     a.lostEvents.Load(),
		SlowWrites:     a.slowWrites.Load(),
	}
}

// Close closes the stream and completes it,
// note that this behavior is different from Stream.Close,
// that aborts it, because of the way the writer is usually used
// the interface - io.WriteCloser has only close method
func (a *SessionWriter) Close(ctx context.Context) error {
	a.cancel()
	<-a.doneCh
	stats := a.Stats()
	if stats.LostEvents != 0 {
		a.log.ErrorContext(ctx, "Session has lost audit events because of disk or network issues. Check disk and network on this server.",
			"lost_events", stats.LostEvents,
			"accepted_events", stats.AcceptedEvents,
		)
	}
	if float64(stats.SlowWrites)/float64(stats.AcceptedEvents) > 0.15 {
		a.log.DebugContext(ctx, "Session has encountered slow writes. Check disk and network on this server.",
			"slow_writes", stats.SlowWrites, "events_written", stats.AcceptedEvents)
	}
	return nil
}

// Complete closes the stream and marks it finalized,
// releases associated resources, in case of failure,
// closes this stream on the client side
func (a *SessionWriter) Complete(ctx context.Context) error {
	return a.Close(ctx)
}

func (a *SessionWriter) processEvents() {
	defer a.cancel()

	for {
		// From the spec:
		//
		// https://golang.org/ref/spec#Select_statements
		//
		// If one or more of the communications can proceed, a single one that
		// can proceed is chosen via a uniform pseudo-random selection.
		//
		// This first drain is necessary to give status updates a priority
		// in the event processing loop. The loop could receive
		// a status update too late in cases with many events.
		// Internal buffer then grows too large and applies
		// backpressure without a need.
		//
		select {
		case status := <-a.stream.Status():
			a.updateStatus(status)
		default:
		}
		select {
		case status := <-a.stream.Status():
			a.updateStatus(status)
		case event := <-a.eventsCh:
			a.buffer = append(a.buffer, event)
			err := a.stream.RecordEvent(a.cfg.Context, event)
			if err != nil {
				if IsPermanentEmitError(err) {
					a.log.WarnContext(a.cfg.Context, "Failed to emit audit event due to permanent emit audit event error. Event will be omitted.", "error", err, "event", event)
					continue
				}

				if isUnrecoverableError(err) {
					a.log.DebugContext(a.cfg.Context, "Failed to emit audit event.", "error", err)
					return
				}

				a.log.DebugContext(a.cfg.Context, "Failed to emit audit event, attempting to recover stream.", "error", err)
				start := time.Now()
				if err := a.recoverStream(); err != nil {
					a.log.WarnContext(a.cfg.Context, "Failed to recover stream.", "error", err)
					return
				}
				a.log.DebugContext(a.cfg.Context, "Recovered stream", "duration", time.Since(start))
			}
		case <-a.stream.Done():
			if a.closeCtx.Err() != nil {
				// don't attempt recovery if we're closing
				return
			}
			a.log.DebugContext(a.cfg.Context, "Stream was closed by the server, attempting to recover.")
			if err := a.recoverStream(); err != nil {
				a.log.WarnContext(a.cfg.Context, "Failed to recover stream.", "error", err)

				return
			}
		case <-a.closeCtx.Done():
			a.completeStream(a.stream)
			return
		}
	}
}

// IsPermanentEmitError checks if the error contains either a sole
// [trace.BadParameter] error in its chain, or a [trace.Aggregate] error
// composed entirely of BadParameters.
func IsPermanentEmitError(err error) bool {
	return isPermanentEmitError(err, 1 /* depth */)
}

func isPermanentEmitError(err error, depth int) bool {
	const maxDepth = 50
	if depth >= maxDepth {
		return false
	}

	// If Aggregate, then it must match entirely.
	var agg trace.Aggregate
	if errors.As(err, &agg) {
		for _, e := range agg.Errors() {
			if !isPermanentEmitError(e, depth+1) {
				return false
			}
		}
		return true
	}

	// Otherwise, a sole BadParameter is enough.
	return trace.IsBadParameter(err)
}

func (a *SessionWriter) recoverStream() error {
	a.closeStream(a.stream)
	stream, err := a.tryResumeStream()
	if err != nil {
		return trace.Wrap(err)
	}
	a.stream = stream
	// replay all non-confirmed audit events to the resumed stream
	start := time.Now()
	for i := range a.buffer {
		err := a.stream.RecordEvent(a.cfg.Context, a.buffer[i])
		if err != nil {
			a.closeStream(a.stream)
			return trace.Wrap(err)
		}
	}
	a.log.DebugContext(a.cfg.Context, "Replayed buffer events to stream", "replayed_events", len(a.buffer), "replay_duration", time.Since(start))
	return nil
}

func (a *SessionWriter) closeStream(stream apievents.Stream) {
	ctx, cancel := context.WithTimeout(a.cfg.Context, NetworkRetryDuration)
	defer cancel()
	if err := stream.Close(ctx); err != nil {
		a.log.DebugContext(ctx, "Failed to close stream.")
	}
}

func (a *SessionWriter) completeStream(stream apievents.Stream) {
	// Cannot use the configured context because it's the server's and when the server
	// is requested to close (and hence the context is canceled), the stream will not be able
	// to complete
	ctx, cancel := context.WithTimeout(context.Background(), NetworkBackoffDuration)
	defer cancel()
	if err := stream.Complete(ctx); err != nil {
		a.log.WarnContext(ctx, "Failed to complete stream.", "error", err)
	}
}

func (a *SessionWriter) tryResumeStream() (apievents.Stream, error) {
	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		Step: NetworkRetryDuration,
		Max:  NetworkBackoffDuration,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var resumedStream apievents.Stream
	start := time.Now()
	for i := 0; i < FastAttempts; i++ {
		var streamType string
		if a.lastStatus == nil {
			// The stream was either never created or has failed to receive the
			// initial status update
			resumedStream, err = a.cfg.Streamer.CreateAuditStream(a.cfg.Context, a.cfg.SessionID)
			streamType = "new"
		} else {
			resumedStream, err = a.cfg.Streamer.ResumeAuditStream(
				a.cfg.Context, a.cfg.SessionID, a.lastStatus.UploadID)
			streamType = "existing"
		}
		retry.Inc()
		if err == nil {
			// The call to CreateAuditStream is async. To learn
			// if it was successful get the first status update
			// sent by the server after create.
			select {
			case status := <-resumedStream.Status():
				a.log.DebugContext(a.closeCtx, "Resumed audit stream",
					"stream", streamType,
					"attempt", i+1,
					"duration", time.Since(start),
					"upload_id", status.UploadID,
				)
				return resumedStream, nil
			case <-retry.After():
				err := resumedStream.Close(a.closeCtx)
				if err != nil {
					a.log.DebugContext(a.closeCtx, "Timed out waiting for stream status update, will retry.", "error", err)
				} else {
					a.log.DebugContext(a.closeCtx, "Timed out waiting for stream status update, will retry.")
				}
			case <-a.cfg.Context.Done():
				return nil, trace.ConnectionProblem(a.closeCtx.Err(), "operation has been canceled")
			}
		}

		if isUnrecoverableError(err) {
			return nil, trace.ConnectionProblem(err, "stream cannot be recovered")
		}

		select {
		case <-retry.After():
			a.log.DebugContext(a.closeCtx, "Retrying to resume stream after backoff.", "error", err)
		case <-a.closeCtx.Done():
			return nil, trace.ConnectionProblem(a.closeCtx.Err(), "operation has been canceled")
		}
	}
	return nil, trace.LimitExceeded("audit stream resume attempts exhausted, last error: %v", err)
}

func (a *SessionWriter) updateStatus(status apievents.StreamStatus) {
	a.lastStatus = &status
	if status.LastEventIndex < 0 {
		return
	}
	lastIndex := -1
	for i := 0; i < len(a.buffer); i++ {
		if status.LastEventIndex < a.buffer[i].GetAuditEvent().GetIndex() {
			break
		}
		lastIndex = i
	}
	if lastIndex > 0 {
		before := len(a.buffer)
		a.buffer = a.buffer[lastIndex+1:]
		a.log.DebugContext(a.closeCtx, "Removed saved events", "removed", before-len(a.buffer), "remaining", len(a.buffer))
	}
}

func diff(before, after time.Time) int64 {
	d := int64(after.Sub(before) / time.Millisecond)
	if d < 0 {
		return 0
	}
	return d
}

// isUnrecoverableError returns if the provided stream error is unrecoverable.
func isUnrecoverableError(err error) bool {
	return err != nil && strings.Contains(err.Error(), uploaderReservePartErrorMessage)
}
