/*
Copyright 2020 Gravitational, Inc.

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

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	logrus "github.com/sirupsen/logrus"
	"go.uber.org/atomic"
)

// NewAuditWriter returns a new instance of session writer
func NewAuditWriter(cfg AuditWriterConfig) (*AuditWriter, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	stream, err := cfg.Streamer.CreateAuditStream(cfg.Context, cfg.SessionID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(cfg.Context)
	writer := &AuditWriter{
		mtx:    sync.Mutex{},
		cfg:    cfg,
		stream: NewCheckingStream(stream, cfg.Clock, cfg.ClusterName),
		log: logrus.WithFields(logrus.Fields{
			trace.Component: cfg.Component,
		}),
		cancel:   cancel,
		closeCtx: ctx,
		eventsCh: make(chan AuditEvent),
		doneCh:   make(chan struct{}),
	}
	go func() {
		writer.processEvents()
		close(writer.doneCh)
	}()
	return writer, nil
}

// AuditWriterConfig configures audit writer
type AuditWriterConfig struct {
	// SessionID defines the session to record.
	SessionID session.ID

	// ServerID is a server ID to write
	ServerID string

	// Namespace is the session namespace.
	Namespace string

	// RecordOutput stores info on whether to record session output
	RecordOutput bool

	// Component is a component used for logging
	Component string

	// Streamer is used to create and resume audit streams
	Streamer Streamer

	// Context is a context to cancel the writes
	// or any other operations
	Context context.Context

	// Clock is used to override time in tests
	Clock clockwork.Clock

	// UID is UID generator
	UID utils.UID

	// BackoffTimeout is a backoff timeout
	// if set, failed audit write events will be lost
	// if audit writer fails to write events after this timeout
	BackoffTimeout time.Duration

	// BackoffDuration is a duration of the backoff before the next try
	BackoffDuration time.Duration

	// ClusterName defines the name of this teleport cluster.
	ClusterName string
}

// CheckAndSetDefaults checks and sets defaults
func (cfg *AuditWriterConfig) CheckAndSetDefaults() error {
	if cfg.SessionID.IsZero() {
		return trace.BadParameter("audit writer config: missing parameter SessionID")
	}
	if cfg.Streamer == nil {
		return trace.BadParameter("audit writer config: missing parameter Streamer")
	}
	if cfg.Context == nil {
		return trace.BadParameter("audit writer config: missing parameter Context")
	}
	if cfg.ClusterName == "" {
		return trace.BadParameter("audit writer config: missing parameter ClusterName")
	}
	if cfg.Namespace == "" {
		cfg.Namespace = defaults.Namespace
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.UID == nil {
		cfg.UID = utils.NewRealUID()
	}
	if cfg.BackoffTimeout == 0 {
		cfg.BackoffTimeout = defaults.AuditBackoffTimeout
	}
	if cfg.BackoffDuration == 0 {
		cfg.BackoffDuration = defaults.NetworkBackoffDuration
	}
	return nil
}

// AuditWriter wraps session stream
// and writes audit events to it
type AuditWriter struct {
	mtx            sync.Mutex
	cfg            AuditWriterConfig
	log            *logrus.Entry
	lastPrintEvent *SessionPrint
	eventIndex     int64
	buffer         []AuditEvent
	eventsCh       chan AuditEvent
	lastStatus     *StreamStatus
	stream         Stream
	cancel         context.CancelFunc
	closeCtx       context.Context
	// doneCh is closed when all internal processes have exited
	doneCh chan struct{}

	backoffUntil   time.Time
	lostEvents     atomic.Int64
	acceptedEvents atomic.Int64
	slowWrites     atomic.Int64
}

// AuditWriterStats provides stats about lost events and slow writes
type AuditWriterStats struct {
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
func (a *AuditWriter) Status() <-chan StreamStatus {
	return nil
}

// Done returns channel closed when streamer is closed
// should be used to detect sending errors
func (a *AuditWriter) Done() <-chan struct{} {
	return a.closeCtx.Done()
}

// Write takes a chunk and writes it into the audit log
func (a *AuditWriter) Write(data []byte) (int, error) {
	if !a.cfg.RecordOutput {
		return len(data), nil
	}
	// buffer is copied here to prevent data corruption:
	// io.Copy allocates single buffer and calls multiple writes in a loop
	// Write is async, this can lead to cases when the buffer is re-used
	// and data is corrupted unless we copy the data buffer in the first place
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	start := time.Now().UTC().Round(time.Millisecond)
	for len(dataCopy) != 0 {
		printEvent := &SessionPrint{
			Metadata: Metadata{
				Type: SessionPrintEvent,
				Time: start,
			},
			Data: dataCopy,
		}
		if printEvent.Size() > MaxProtoMessageSizeBytes {
			extraBytes := printEvent.Size() - MaxProtoMessageSizeBytes
			printEvent.Data = dataCopy[:extraBytes]
			printEvent.Bytes = int64(len(printEvent.Data))
			dataCopy = dataCopy[extraBytes:]
		} else {
			printEvent.Bytes = int64(len(printEvent.Data))
			dataCopy = nil
		}
		if err := a.EmitAuditEvent(a.cfg.Context, printEvent); err != nil {
			a.log.WithError(err).Error("Failed to emit session print event.")
			return 0, trace.Wrap(err)
		}
	}
	return len(data), nil
}

// checkAndResetBackoff checks whether the backoff is in place,
// also resets it if the time has passed. If the state is backoff,
// returns true
func (a *AuditWriter) checkAndResetBackoff(now time.Time) bool {
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
func (a *AuditWriter) maybeSetBackoff(backoffUntil time.Time) bool {
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

// EmitAuditEvent emits audit event
func (a *AuditWriter) EmitAuditEvent(ctx context.Context, event AuditEvent) error {
	// Event modification is done under lock and in the same goroutine
	// as the caller to avoid data races and event copying
	if err := a.setupEvent(event); err != nil {
		return trace.Wrap(err)
	}

	a.acceptedEvents.Inc()

	// Without serialization, EmitAuditEvent will call grpc's method directly.
	// When BPF callback is emitting events concurrently with session data to the grpc stream,
	// it becomes deadlocked (not just blocked temporarily, but permanently)
	// in flowcontrol.go, trying to get quota:
	// https://github.com/grpc/grpc-go/blob/a906ca0441ceb1f7cd4f5c7de30b8e81ce2ff5e8/internal/transport/flowcontrol.go#L60

	// If backoff is in effect, loose event, return right away
	if isBackoff := a.checkAndResetBackoff(a.cfg.Clock.Now()); isBackoff {
		a.lostEvents.Inc()
		return nil
	}

	// This fast path will be used all the time during normal operation.
	select {
	case a.eventsCh <- event:
		return nil
	case <-ctx.Done():
		return trace.ConnectionProblem(ctx.Err(), "context canceled or timed out")
	case <-a.closeCtx.Done():
		return trace.ConnectionProblem(a.closeCtx.Err(), "audit writer is closed")
	default:
		a.slowWrites.Inc()
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
	case a.eventsCh <- event:
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
			a.log.Errorf("Audit write timed out after %v. Will be loosing events for the next %v.", a.cfg.BackoffTimeout, a.cfg.BackoffDuration)
		}
		a.lostEvents.Inc()
		return nil
	case <-ctx.Done():
		a.lostEvents.Inc()
		stopped := t.Stop()
		if !stopped {
			<-t.C
		}
		return trace.ConnectionProblem(ctx.Err(), "context canceled or timed out")
	case <-a.closeCtx.Done():
		a.lostEvents.Inc()
		stopped := t.Stop()
		if !stopped {
			<-t.C
		}
		return trace.ConnectionProblem(a.closeCtx.Err(), "writer is closed")
	}
}

var timerPool sync.Pool

// Stats returns up to date stats from this audit writer
func (a *AuditWriter) Stats() AuditWriterStats {
	return AuditWriterStats{
		AcceptedEvents: a.acceptedEvents.Load(),
		LostEvents:     a.lostEvents.Load(),
		SlowWrites:     a.slowWrites.Load(),
	}
}

// Close closes the stream and completes it,
// note that this behavior is different from Stream.Close,
// that aborts it, because of the way the writer is usually used
// the interface - io.WriteCloser has only close method
func (a *AuditWriter) Close(ctx context.Context) error {
	a.cancel()
	<-a.doneCh
	stats := a.Stats()
	if stats.LostEvents != 0 {
		a.log.Errorf("Session has lost %v out of %v audit events because of disk or network issues. Check disk and network on this server.", stats.LostEvents, stats.AcceptedEvents)
	}
	if stats.SlowWrites != 0 {
		a.log.Debugf("Session has encountered %v slow writes out of %v. Check disk and network on this server.", stats.SlowWrites, stats.AcceptedEvents)
	}
	return nil
}

// Complete closes the stream and marks it finalized,
// releases associated resources, in case of failure,
// closes this stream on the client side
func (a *AuditWriter) Complete(ctx context.Context) error {
	return a.Close(ctx)
}

func (a *AuditWriter) processEvents() {
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
			err := a.stream.EmitAuditEvent(a.cfg.Context, event)
			if err == nil {
				continue
			}
			a.log.WithError(err).Debug("Failed to emit audit event, attempting to recover stream.")
			start := time.Now()
			if err := a.recoverStream(); err != nil {
				a.log.WithError(err).Warningf("Failed to recover stream.")
				a.cancel()
				return
			}
			a.log.Debugf("Recovered stream in %v.", time.Since(start))
		case <-a.stream.Done():
			a.log.Debugf("Stream was closed by the server, attempting to recover.")
			if err := a.recoverStream(); err != nil {
				a.log.WithError(err).Warningf("Failed to recover stream.")
				a.cancel()
				return
			}
		case <-a.closeCtx.Done():
			a.completeStream(a.stream)
			return
		}
	}
}

func (a *AuditWriter) recoverStream() error {
	a.closeStream(a.stream)
	stream, err := a.tryResumeStream()
	if err != nil {
		return trace.Wrap(err)
	}
	a.stream = stream
	// replay all non-confirmed audit events to the resumed stream
	start := time.Now()
	for i := range a.buffer {
		err := a.stream.EmitAuditEvent(a.cfg.Context, a.buffer[i])
		if err != nil {
			a.closeStream(a.stream)
			return trace.Wrap(err)
		}
	}
	a.log.Debugf("Replayed buffer of %v events to stream in %v", len(a.buffer), time.Since(start))
	return nil
}

func (a *AuditWriter) closeStream(stream Stream) {
	ctx, cancel := context.WithTimeout(a.cfg.Context, defaults.NetworkRetryDuration)
	defer cancel()
	if err := stream.Close(ctx); err != nil {
		a.log.WithError(err).Debug("Failed to close stream.")
	}
}

func (a *AuditWriter) completeStream(stream Stream) {
	ctx, cancel := context.WithTimeout(a.cfg.Context, defaults.NetworkBackoffDuration)
	defer cancel()
	if err := stream.Complete(ctx); err != nil {
		a.log.WithError(err).Warning("Failed to complete stream.")
	}
}

func (a *AuditWriter) tryResumeStream() (Stream, error) {
	retry, err := utils.NewLinear(utils.LinearConfig{
		Step: defaults.NetworkRetryDuration,
		Max:  defaults.NetworkBackoffDuration,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var resumedStream Stream
	start := time.Now()
	for i := 0; i < defaults.FastAttempts; i++ {
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
				a.log.Debugf("Resumed %v stream on %v attempt in %v, upload %v.",
					streamType, i+1, time.Since(start), status.UploadID)
				return resumedStream, nil
			case <-retry.After():
				err := resumedStream.Close(a.closeCtx)
				if err != nil {
					a.log.WithError(err).Debug("Timed out waiting for stream status update, will retry.")
				} else {
					a.log.Debug("Timed out waiting for stream status update, will retry.")
				}
			case <-a.cfg.Context.Done():
				return nil, trace.ConnectionProblem(a.closeCtx.Err(), "operation has been canceled")
			}
		}
		select {
		case <-retry.After():
			a.log.WithError(err).Debug("Retrying to resume stream after backoff.")
		case <-a.closeCtx.Done():
			return nil, trace.ConnectionProblem(a.closeCtx.Err(), "operation has been canceled")
		}
	}
	return nil, trace.Wrap(err)
}

func (a *AuditWriter) updateStatus(status StreamStatus) {
	a.lastStatus = &status
	if status.LastEventIndex < 0 {
		return
	}
	lastIndex := -1
	for i := 0; i < len(a.buffer); i++ {
		if status.LastEventIndex < a.buffer[i].GetIndex() {
			break
		}
		lastIndex = i
	}
	if lastIndex > 0 {
		before := len(a.buffer)
		a.buffer = a.buffer[lastIndex+1:]
		a.log.Debugf("Removed %v saved events, current buffer size: %v.", before-len(a.buffer), len(a.buffer))
	}
}

func (a *AuditWriter) setupEvent(event AuditEvent) error {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	if err := checkAndSetEventFields(event, a.cfg.Clock, a.cfg.UID, a.cfg.ClusterName); err != nil {
		return trace.Wrap(err)
	}

	sess, ok := event.(SessionMetadataSetter)
	if ok {
		sess.SetSessionID(string(a.cfg.SessionID))
	}

	srv, ok := event.(ServerMetadataSetter)
	if ok {
		srv.SetServerNamespace(a.cfg.Namespace)
		srv.SetServerID(a.cfg.ServerID)
	}

	event.SetIndex(a.eventIndex)
	a.eventIndex++

	printEvent, ok := event.(*SessionPrint)
	if !ok {
		return nil
	}

	if a.lastPrintEvent != nil {
		printEvent.Offset = a.lastPrintEvent.Offset + int64(len(a.lastPrintEvent.Data))
		printEvent.DelayMilliseconds = diff(a.lastPrintEvent.Time, printEvent.Time) + a.lastPrintEvent.DelayMilliseconds
		printEvent.ChunkIndex = a.lastPrintEvent.ChunkIndex + 1
	}
	a.lastPrintEvent = printEvent
	return nil
}
