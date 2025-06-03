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

package recorder

import (
	"context"
	"path/filepath"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/filesessions"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
)

// Config configures a session recorder.
type Config struct {
	// SessionID defines the session to record.
	SessionID session.ID

	// ServerID is a server ID to write.
	ServerID string

	// Namespace is the session namespace.
	Namespace string

	// Clock is used to override time in tests.
	Clock clockwork.Clock

	// UID is UID generator.
	UID utils.UID

	// ClusterName defines the name of this teleport cluster.
	ClusterName string

	// RecordingCfg is a session recording config.
	RecordingCfg types.SessionRecordingConfig

	// SyncStreamer will be used to create session recording streams if
	// RecordingCfg specifies that session recording should be done
	// synchronously.
	SyncStreamer events.Streamer

	// DataDir is the directory that data should be stored in.
	DataDir string

	// Component is a component used for logging.
	Component string

	// Context is a context to cancel the writes
	// or any other operations.
	Context context.Context

	// MakeEvents converts bytes written via the io.Writer interface
	// into AuditEvents that are written to the stream.
	// For backwards compatibility, SessionWriter will convert bytes to
	// SessionPrint events when MakeEvents is not provided.
	MakeEvents func([]byte) []apievents.AuditEvent

	// BackoffTimeout is a backoff timeout
	// if set, failed audit write events will be lost
	// if session writer fails to write events after this timeout.
	BackoffTimeout time.Duration

	// BackoffDuration is a duration of the backoff before the next try.
	BackoffDuration time.Duration

	// StartTime represents the time the recorder started. If not zero, this
	// value is used to generate the events index.
	StartTime time.Time
}

// New returns a [events.SessionPreparerRecorder]. If session recording is disabled,
// a recorder is returned that will discard all session events. If session
// recording is set to be synchronous, the returned recorder will use
// syncStream to create an event stream. Otherwise, a streamer will be
// used that will back recorded session events to disk for eventual upload.
func New(cfg Config) (events.SessionPreparerRecorder, error) {
	if cfg.RecordingCfg == nil {
		return nil, trace.BadParameter("RecordingCfg must be set")
	}
	if cfg.SyncStreamer == nil {
		return nil, trace.BadParameter("SyncStreamer must be set")
	}
	if cfg.DataDir == "" {
		return nil, trace.BadParameter("DataDir must be set")
	}

	preparer, err := events.NewPreparer(events.PreparerConfig{
		SessionID:   cfg.SessionID,
		ServerID:    cfg.ServerID,
		Namespace:   cfg.Namespace,
		Clock:       cfg.Clock,
		UID:         cfg.UID,
		ClusterName: cfg.ClusterName,
		StartTime:   cfg.StartTime,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.RecordingCfg.GetMode() == types.RecordOff {
		return events.NewSessionPreparerRecorder(preparer, events.NewDiscardRecorder()), nil
	}

	streamer := cfg.SyncStreamer
	if !services.IsRecordSync(cfg.RecordingCfg.GetMode()) {
		uploadDir := filepath.Join(
			cfg.DataDir, teleport.LogsDir, teleport.ComponentUpload,
			events.StreamingSessionsDir, cfg.Namespace,
		)
		fileStreamer, err := filesessions.NewStreamer(uploadDir)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		streamer = fileStreamer
	}

	rec, err := events.NewSessionWriter(events.SessionWriterConfig{
		SessionID:       cfg.SessionID,
		Component:       cfg.Component,
		MakeEvents:      cfg.MakeEvents,
		Preparer:        preparer,
		Streamer:        streamer,
		Context:         cfg.Context,
		Clock:           cfg.Clock,
		BackoffTimeout:  cfg.BackoffTimeout,
		BackoffDuration: cfg.BackoffDuration,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rec, nil
}
