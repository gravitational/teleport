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
		stream: NewCheckingStream(stream, cfg.Clock),
		log: logrus.WithFields(logrus.Fields{
			trace.Component: cfg.Component,
		}),
		cancel:   cancel,
		closeCtx: ctx,
		eventsCh: make(chan AuditEvent),
	}
	go writer.processEvents()
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
}

// CheckAndSetDefaults checks and sets defaults
func (cfg *AuditWriterConfig) CheckAndSetDefaults() error {
	if cfg.SessionID.IsZero() {
		return trace.BadParameter("missing parameter SessionID")
	}
	if cfg.Streamer == nil {
		return trace.BadParameter("missing parameter Streamer")
	}
	if cfg.Context == nil {
		return trace.BadParameter("missing parameter Context")
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

// EmitAuditEvent emits audit event
func (a *AuditWriter) EmitAuditEvent(ctx context.Context, event AuditEvent) error {
	// Event modification is done under lock and in the same goroutine
	// as the caller to avoid data races and event copying
	if err := a.setupEvent(event); err != nil {
		return trace.Wrap(err)
	}

	// Without serialization, EmitAuditEvent will call grpc's method directly.
	// When BPF callback is emitting events concurrently with session data to the grpc stream,
	// it becomes deadlocked (not just blocked temporarily, but permanently)
	// in flowcontrol.go, trying to get quota:
	// https://github.com/grpc/grpc-go/blob/a906ca0441ceb1f7cd4f5c7de30b8e81ce2ff5e8/internal/transport/flowcontrol.go#L60
	select {
	case a.eventsCh <- event:
		return nil
	case <-ctx.Done():
		return trace.ConnectionProblem(ctx.Err(), "context done")
	case <-a.closeCtx.Done():
		return trace.ConnectionProblem(a.closeCtx.Err(), "writer is closed")
	}
}

// Close closes the stream and completes it,
// note that this behavior is different from Stream.Close,
// that aborts it, because of the way the writer is usually used
// the interface - io.WriteCloser has only close method
func (a *AuditWriter) Close(ctx context.Context) error {
	a.cancel()
	return nil
}

// Complete closes the stream and marks it finalized,
// releases associated resources, in case of failure,
// closes this stream on the client side
func (a *AuditWriter) Complete(ctx context.Context) error {
	a.cancel()
	return nil
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
			a.log.WithError(err).Debugf("Failed to emit audit event, attempting to recover stream.")
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
			if err := a.stream.Complete(a.cfg.Context); err != nil {
				a.log.WithError(err).Warningf("Failed to complete stream")
				return
			}
			return
		}
	}
}

func (a *AuditWriter) recoverStream() error {
	// if there is a previous stream, close it
	if err := a.stream.Close(a.cfg.Context); err != nil {
		a.log.WithError(err).Debugf("Failed to close stream.")
	}
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
			if err := a.stream.Close(a.cfg.Context); err != nil {
				a.log.WithError(err).Debugf("Failed to close stream.")
			}
			return trace.Wrap(err)
		}
	}
	a.log.Debugf("Replayed buffer of %v events to stream in %v", len(a.buffer), time.Since(start))
	return nil
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
					a.log.WithError(err).Debugf("Timed out waiting for stream status update, will retry.")
				} else {
					a.log.Debugf("Timed out waiting for stream status update, will retry.")
				}
			case <-a.closeCtx.Done():
				return nil, trace.ConnectionProblem(a.closeCtx.Err(), "operation has been cancelled")
			}
		}
		select {
		case <-retry.After():
			a.log.WithError(err).Debugf("Retrying to resume stream after backoff.")
		case <-a.closeCtx.Done():
			return nil, trace.ConnectionProblem(a.closeCtx.Err(), "operation has been cancelled")
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

	if err := CheckAndSetEventFields(event, a.cfg.Clock, a.cfg.UID); err != nil {
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
