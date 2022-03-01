/*
Copyright 2018 Gravitational, Inc.

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
	"encoding/json"
	"sync"
	"time"

	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// ForwarderConfig forwards session log events
// to the auth server, and writes the session playback to disk
type ForwarderConfig struct {
	// IAuditLog is the audit log to forward non-print events to
	IAuditLog

	// SessionID is a session id to write
	SessionID session.ID
	// ServerID is a serverID data directory
	ServerID string
	// DataDir is a data directory
	DataDir string
	// RecordSessions is a sessions recording setting
	RecordSessions bool
	// Namespace is a namespace of the session
	Namespace string
	// Clock is a clock to set for tests
	Clock clockwork.Clock
	// UID is UID generator
	UID utils.UID
}

// CheckAndSetDefaults checks and sets default values
func (s *ForwarderConfig) CheckAndSetDefaults() error {
	if s.IAuditLog == nil {
		return trace.BadParameter("missing parameter bucket")
	}
	if s.DataDir == "" {
		return trace.BadParameter("missing data dir")
	}
	if s.Clock == nil {
		s.Clock = clockwork.NewRealClock()
	}
	if s.UID == nil {
		s.UID = utils.NewRealUID()
	}
	return nil
}

// NewForwarder returns a new instance of session forwarder
func NewForwarder(cfg ForwarderConfig) (*Forwarder, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	diskLogger, err := NewDiskSessionLogger(DiskSessionLoggerConfig{
		SessionID:      cfg.SessionID,
		DataDir:        cfg.DataDir,
		RecordSessions: cfg.RecordSessions,
		Namespace:      cfg.Namespace,
		ServerID:       cfg.ServerID,
		Clock:          cfg.Clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Forwarder{
		ForwarderConfig: cfg,
		sessionLogger:   diskLogger,
		enhancedIndexes: map[string]int64{
			SessionCommandEvent:     0,
			SessionDiskEvent:        0,
			SessionNetworkEvent:     0,
			SessionProcessExitEvent: 0,
		},
	}, nil
}

// ForwarderConfig forwards session log events
// to the auth server, and writes the session playback to disk
type Forwarder struct {
	ForwarderConfig
	sessionLogger   *DiskSessionLogger
	lastChunk       *SessionChunk
	eventIndex      int64
	enhancedIndexes map[string]int64
	sync.Mutex
	isClosed bool
}

// Closer releases connection and resources associated with log if any
func (l *Forwarder) Close() error {
	l.Lock()
	defer l.Unlock()
	if l.isClosed {
		return nil
	}
	l.isClosed = true
	return l.sessionLogger.Finalize()
}

// EmitAuditEventLegacy emits audit event
func (l *Forwarder) EmitAuditEventLegacy(event Event, fields EventFields) error {
	err := UpdateEventFields(event, fields, l.Clock, l.UID)
	if err != nil {
		return trace.Wrap(err)
	}
	data, err := json.Marshal(fields)
	if err != nil {
		return trace.Wrap(err)
	}
	chunks := []*SessionChunk{
		{
			EventType: event.Name,
			Data:      data,
			Time:      time.Now().UTC().UnixNano(),
		},
	}
	return l.PostSessionSlice(SessionSlice{
		Namespace: l.Namespace,
		SessionID: string(l.SessionID),
		Version:   V3,
		Chunks:    chunks,
	})
}

// PostSessionSlice sends chunks of recorded session to the event log
func (l *Forwarder) PostSessionSlice(slice SessionSlice) error {
	// setup slice sets slice version, properly numerates
	// all chunks and
	chunksWithoutPrintEvents, err := l.setupSlice(&slice)
	if err != nil {
		return trace.Wrap(err)
	}

	// log all events and session recording locally
	err = l.sessionLogger.PostSessionSlice(slice)
	if err != nil {
		return trace.Wrap(err)
	}

	// no chunks to post (all chunks are print events)
	if len(chunksWithoutPrintEvents) == 0 {
		return nil
	}
	slice.Chunks = chunksWithoutPrintEvents
	slice.Version = V3
	err = l.IAuditLog.PostSessionSlice(slice)
	return err
}

func (l *Forwarder) setupSlice(slice *SessionSlice) ([]*SessionChunk, error) {
	l.Lock()
	defer l.Unlock()

	if l.isClosed {
		return nil, trace.BadParameter("write on closed forwarder")
	}

	// Setup chunk indexes.
	var chunks []*SessionChunk
	for _, chunk := range slice.Chunks {

		switch chunk.EventType {
		case "":
			return nil, trace.BadParameter("missing event type")
		case SessionCommandEvent, SessionDiskEvent, SessionNetworkEvent, SessionProcessExitEvent:
			chunk.EventIndex = l.enhancedIndexes[chunk.EventType]
			l.enhancedIndexes[chunk.EventType]++

			chunks = append(chunks, chunk)
		case SessionPrintEvent:
			chunk.EventIndex = l.eventIndex
			l.eventIndex++

			// Filter out chunks with session print events, as this logger forwards
			// only audit events to the auth server.
			if l.lastChunk != nil {
				chunk.Offset = l.lastChunk.Offset + int64(len(l.lastChunk.Data))
				chunk.Delay = diff(time.Unix(0, l.lastChunk.Time), time.Unix(0, chunk.Time)) + l.lastChunk.Delay
				chunk.ChunkIndex = l.lastChunk.ChunkIndex + 1
			}
			l.lastChunk = chunk
		default:
			chunk.EventIndex = l.eventIndex
			l.eventIndex++

			chunks = append(chunks, chunk)
		}
	}

	return chunks, nil
}
