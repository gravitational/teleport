/*
Copyright 2023 Gravitational, Inc.

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
	"sync/atomic"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
)

func NewPreparer(cfg PreparerConfig) (*Preparer, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Preparer{
		cfg: cfg,
	}, nil
}

// PreparerConfig configures an event setter
type PreparerConfig struct {
	// SessionID defines the session to record.
	SessionID session.ID

	// ServerID is a server ID to write
	ServerID string

	// Namespace is the session namespace.
	Namespace string

	// Clock is used to override time in tests
	Clock clockwork.Clock

	// UID is UID generator
	UID utils.UID

	// ClusterName defines the name of this teleport cluster.
	ClusterName string
}

// CheckAndSetDefaults checks and sets defaults
func (cfg *PreparerConfig) CheckAndSetDefaults() error {
	if cfg.SessionID.IsZero() {
		return trace.BadParameter("event checker config: missing parameter SessionID")
	}
	if cfg.ClusterName == "" {
		return trace.BadParameter("event checker config: missing parameter ClusterName")
	}
	if cfg.Namespace == "" {
		cfg.Namespace = apidefaults.Namespace
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.UID == nil {
		cfg.UID = utils.NewRealUID()
	}

	return nil
}

// Preparer sets necessary unset fields in session events.
type Preparer struct {
	mtx            sync.Mutex
	cfg            PreparerConfig
	lastPrintEvent *apievents.SessionPrint
	eventIndex     atomic.Uint64
}

// PrepareSessionEvent will set necessary event fields for session-related
// events and must be called before the event is recorded, regardless
// of whether the event will be recorded, emitted, or both.
func (c *Preparer) PrepareSessionEvent(event apievents.AuditEvent) (apievents.PreparedSessionEvent, error) {
	if err := checkAndSetEventFields(event, c.cfg.Clock, c.cfg.UID, c.cfg.ClusterName); err != nil {
		return nil, trace.Wrap(err)
	}

	sess, ok := event.(SessionMetadataSetter)
	if ok {
		sess.SetSessionID(string(c.cfg.SessionID))
	}

	srv, ok := event.(ServerMetadataSetter)
	if ok {
		srv.SetServerNamespace(c.cfg.Namespace)
		srv.SetServerID(c.cfg.ServerID)
	}

	// ensure index is incremented and loaded atomically
	event.SetIndex(int64(c.eventIndex.Add(1) - 1))

	preparedEvent := preparedSessionEvent{
		event: event,
	}

	printEvent, ok := preparedEvent.event.(*apievents.SessionPrint)
	if !ok {
		return preparedEvent, nil
	}

	c.mtx.Lock()
	defer c.mtx.Unlock()

	if c.lastPrintEvent != nil {
		printEvent.Offset = c.lastPrintEvent.Offset + int64(len(c.lastPrintEvent.Data))
		printEvent.DelayMilliseconds = diff(c.lastPrintEvent.Time, printEvent.Time) + c.lastPrintEvent.DelayMilliseconds
		printEvent.ChunkIndex = c.lastPrintEvent.ChunkIndex + 1
	}
	c.lastPrintEvent = printEvent

	return preparedEvent, nil
}

type preparedSessionEvent struct {
	event apievents.AuditEvent
}

func (p preparedSessionEvent) GetAuditEvent() apievents.AuditEvent {
	return p.event
}

type setterAndRecorder struct {
	SessionEventPreparer
	SessionRecorder
}

// NewSessionPreparerRecorder returns a SessionPreparerRecorder that can both
// setup and record session events.
func NewSessionPreparerRecorder(setter SessionEventPreparer, recorder SessionRecorder) SessionPreparerRecorder {
	return setterAndRecorder{
		SessionEventPreparer: setter,
		SessionRecorder:      recorder,
	}
}

// SetupAndRecordEvent will set necessary event fields for session-related
// events and record them.
func SetupAndRecordEvent(ctx context.Context, s SessionPreparerRecorder, e apievents.AuditEvent) error {
	event, err := s.PrepareSessionEvent(e)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := s.RecordEvent(ctx, event); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
