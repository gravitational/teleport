/*
Copyright 2015-2020 Gravitational, Inc.

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
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/session"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// SessionRecorder implements io.Writer to be plugged into the multi-writer
// associated with every session. It forwards session stream to the audit log
type SessionRecorder interface {
	io.Writer
	Emitter
	Close(ctx context.Context) error
}

// DiscardRecorder discards all writes
type DiscardRecorder struct {
	DiscardAuditLog
}

// Write acks all writes but discards them
func (*DiscardRecorder) Write(b []byte) (int, error) {
	return len(b), nil
}

// Close does nothing and always succeeds
func (*DiscardRecorder) Close() error {
	return nil
}

// GetAuditLog returns audit log associated with this recorder
func (d *DiscardRecorder) GetAuditLog() IAuditLog {
	return &d.DiscardAuditLog
}

// ForwardRecorder implements io.Writer to be plugged into the multi-writer
// associated with every session. It forwards session stream to the audit log
type ForwardRecorder struct {
	// ForwardRecorderConfig specifies session recorder configuration
	ForwardRecorderConfig

	// Entry holds the structured logger
	*logrus.Entry

	// AuditLog is the audit log to store session chunks
	AuditLog IAuditLog
}

// ForwardRecorderConfig specifies config for session recording
type ForwardRecorderConfig struct {
	// DataDir is a data directory to record
	DataDir string

	// SessionID defines the session to record.
	SessionID session.ID

	// Namespace is the session namespace.
	Namespace string

	// RecordSessions stores info on whether to record sessions
	RecordSessions bool

	// Component is a component used for logging
	Component string

	// ForwardTo is external audit log where events will be forwarded
	ForwardTo IAuditLog
}

func (cfg *ForwardRecorderConfig) CheckAndSetDefaults() error {
	if cfg.DataDir == "" {
		return trace.BadParameter("missing parameter DataDir")
	}
	if cfg.SessionID.IsZero() {
		return trace.BadParameter("missing parameter DataDir")
	}
	if cfg.Namespace == "" {
		cfg.Namespace = defaults.Namespace
	}
	if cfg.ForwardTo == nil {
		cfg.ForwardTo = &DiscardAuditLog{}
	}
	return nil
}

// NewForwardRecorder returns a new instance of session recorder
func NewForwardRecorder(cfg ForwardRecorderConfig) (*ForwardRecorder, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Always write sessions to local disk first, then forward them to the Auth
	// Server later.
	auditLog, err := NewForwarder(ForwarderConfig{
		SessionID:      cfg.SessionID,
		ServerID:       teleport.ComponentUpload,
		DataDir:        cfg.DataDir,
		RecordSessions: cfg.RecordSessions,
		Namespace:      cfg.Namespace,
		ForwardTo:      cfg.ForwardTo,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sr := &ForwardRecorder{
		ForwardRecorderConfig: cfg,
		Entry: logrus.WithFields(logrus.Fields{
			trace.Component: cfg.Component,
		}),
		AuditLog: auditLog,
	}
	return sr, nil
}

// GetAuditLog returns audit log associated with this recorder
func (r *ForwardRecorder) GetAuditLog() IAuditLog {
	return r.AuditLog
}

// Write takes a chunk and writes it into the audit log
func (r *ForwardRecorder) Write(data []byte) (int, error) {
	// we are copying buffer to prevent data corruption:
	// io.Copy allocates single buffer and calls multiple writes in a loop
	// our PostSessionSlice is async and sends reader wrapping buffer
	// to the channel. This can lead to cases when the buffer is re-used
	// and data is corrupted unless we copy the data buffer in the first place
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	// post the chunk of bytes to the audit log:
	chunk := &SessionChunk{
		EventType: SessionPrintEvent,
		Data:      dataCopy,
		Time:      time.Now().UTC().UnixNano(),
	}
	if err := r.AuditLog.PostSessionSlice(SessionSlice{
		Namespace: r.Namespace,
		SessionID: string(r.SessionID),
		Chunks:    []*SessionChunk{chunk},
	}); err != nil {
		r.Error(trace.DebugReport(err))
	}
	return len(data), nil
}

// Close closes audit log session recorder
func (r *ForwardRecorder) Close() error {
	var errors []error
	err := r.AuditLog.Close()
	errors = append(errors, err)

	// wait until all events from recorder get flushed, it is important
	// to do so before we send SessionEndEvent to advise the audit log
	// to release resources associated with this session.
	// not doing so will not result in memory leak, but could result
	// in missing playback events
	context, cancel := context.WithTimeout(context.TODO(), defaults.ReadHeadersTimeout)
	defer cancel() // releases resources if slowOperation completes before timeout elapses
	err = r.AuditLog.WaitForDelivery(context)
	if err != nil {
		errors = append(errors, err)
		r.Warnf("Timeout waiting for session to flush events: %v", trace.DebugReport(err))
	}

	return trace.NewAggregate(errors...)
}
