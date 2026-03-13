/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"testing"

	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
)

// noOpPreparer is a SessionEventPreparer that returns events unchanged.
// It is used in tests that only need the SessionWriterConfig to pass
// validation without actually preparing events.
type noOpPreparer struct{}

func (noOpPreparer) PrepareSessionEvent(event apievents.AuditEvent) (apievents.PreparedSessionEvent, error) {
	return preparedEvent{event: event}, nil
}

type preparedEvent struct {
	event apievents.AuditEvent
}

func (p preparedEvent) GetAuditEvent() apievents.AuditEvent { return p.event }

// TestSessionWriterConfigMaxBufferSize verifies that the MaxBufferSize
// config field defaults to DefaultMaxBufferSize when unset and
// preserves an explicit value when set. Without a default, the buffer
// cap would be zero, which would block all event processing.
//
// The SessionWriter's internal buffer accumulates PreparedSessionEvents
// until a status update from the upload stream confirms they have been
// persisted, at which point the confirmed prefix is trimmed. When the
// upload stream stalls (e.g., disk IOPS exhausted on a Kubernetes
// emptyDir volume), status updates stop arriving and the buffer grows
// without limit. Each event is small (~200 bytes), but at sustained
// throughput (hundreds of events per second across many sessions) this
// leads to OOM.
//
// The MaxBufferSize config caps the buffer. When full, processEvents
// stops reading from eventsCh, which creates backpressure through the
// unbuffered channel: RecordEvent blocks for BackoffTimeout (5s by
// default) then drops the event and enters backoff. This trades event
// loss for process survival.
func TestSessionWriterConfigMaxBufferSize(t *testing.T) {
	t.Run("defaults to DefaultMaxBufferSize when zero", func(t *testing.T) {
		cfg := SessionWriterConfig{
			SessionID: "test",
			Streamer:  NewDiscardStreamer(),
			Preparer:  noOpPreparer{},
			Context:   t.Context(),
		}
		require.NoError(t, cfg.CheckAndSetDefaults())
		require.Equal(t, DefaultMaxBufferSize, cfg.MaxBufferSize, "MaxBufferSize must default to DefaultMaxBufferSize so the buffer cap is active even when callers do not set it explicitly")
	})

	t.Run("preserves explicit value", func(t *testing.T) {
		cfg := SessionWriterConfig{
			SessionID:     "test",
			Streamer:      NewDiscardStreamer(),
			Preparer:      noOpPreparer{},
			Context:       t.Context(),
			MaxBufferSize: 500,
		}
		require.NoError(t, cfg.CheckAndSetDefaults())
		require.Equal(t, 500, cfg.MaxBufferSize)
	})
}

// TestUpdateStatusTrimsAtIndexZero verifies that updateStatus trims the
// buffer when the only confirmed event is at buffer index 0. Before the
// fix, the condition was lastIndex > 0 which skipped trimming when only
// one event was confirmed, causing the buffer to grow without bound
// even when the upload stream was healthy.
func TestUpdateStatusTrimsAtIndexZero(t *testing.T) {
	evt := &apievents.SessionPrint{}
	evt.SetIndex(0)

	w := &SessionWriter{
		log:      slog.Default(),
		buffer:   []apievents.PreparedSessionEvent{preparedEvent{event: evt}},
		cfg:      SessionWriterConfig{MaxBufferSize: DefaultMaxBufferSize},
		closeCtx: context.Background(),
	}

	w.updateStatus(apievents.StreamStatus{LastEventIndex: 0})
	require.Empty(t, w.buffer, "updateStatus must trim the buffer when the confirmed event is at index 0")
}
