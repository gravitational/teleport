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
	"sync"
	"sync/atomic"
	"time"

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

	// StartTime represents the time the recorder started. If not zero, this
	// value is used to generate the events index.
	StartTime time.Time
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

	event.SetIndex(c.nextIndex())

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

func (c *Preparer) nextIndex() int64 {
	if !c.cfg.StartTime.IsZero() {
		return c.cfg.Clock.Since(c.cfg.StartTime).Nanoseconds()
	}

	return int64(c.eventIndex.Add(1) - 1)
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
