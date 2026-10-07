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

package githttp

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/recorder"
	rsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/tlsca"
)

// getOrCreateSessionChunk returns the cached session chunk for this session,
// creating one if needed. Multiple requests in the same session share one
// chunk.
func (h *Handler) getOrCreateSessionChunk(identity *tlsca.Identity, gitServer types.Server) (*sessionChunk, error) {
	return h.sessionChunks.Get(h.closeContext, identity.RouteToGit.SessionID, func(context.Context) (*sessionChunk, error) {
		if err := h.sessionChunkLimiter.RegisterRequest(identity.Username); err != nil {
			return nil, trace.Wrap(err)
		}
		chunkID := uuid.NewString()
		recorder, err := h.newSessionRecorderFn(h.closeContext, chunkID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		h.emitAuditEvent(&apievents.GitSessionChunk{
			Metadata: apievents.Metadata{
				Type:        events.GitSessionChunkEvent,
				Code:        events.GitSessionChunkCode,
				ClusterName: h.cfg.ClusterName,
			},
			UserMetadata:       identity.GetUserMetadata(),
			SessionMetadata:    makeSessionMetadata(identity),
			ServerMetadata:     makeServerMetadata(h.cfg.HostID),
			ConnectionMetadata: apievents.ConnectionMetadata{},
			GitMetadata:        makeGitMetadata(gitServer),
			SessionChunkId:     chunkID,
		})
		return &sessionChunk{
			logger:       h.cfg.Logger,
			clock:        h.cfg.Clock,
			closeContext: h.closeContext,
			recorder:     recorder,
		}, nil
	})
}

func (h *Handler) newSessionRecorder(ctx context.Context, chunkID string) (events.SessionPreparerRecorder, error) {
	recConfig, err := h.cfg.AccessPoint.GetSessionRecordingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return recorder.New(recorder.Config{
		SessionID:    rsession.ID(chunkID),
		ServerID:     h.cfg.HostID,
		Namespace:    apidefaults.Namespace,
		Clock:        h.cfg.Clock,
		ClusterName:  h.cfg.ClusterName,
		RecordingCfg: recConfig,
		SyncStreamer: h.cfg.AuthClient,
		DataDir:      h.cfg.DataDir,
		Component:    teleport.Component(teleport.ComponentSession, teleport.ComponentGit),
		Context:      ctx,
		StartTime:    h.cfg.Clock.Now().UTC(),
	})
}

// sessionChunk is a session recording chunk shared by requests of the same
// session.
//
// TODO(greedy52) record more than the incoming request, e.g. response status
// code, upstream errors. Capturing them requires handling the session chunk
// lifecycle for in-flight requests.
type sessionChunk struct {
	recorder     events.SessionPreparerRecorder
	logger       *slog.Logger
	closeContext context.Context
	clock        clockwork.Clock
}

func (c *sessionChunk) recordEvent(event apievents.AuditEvent) {
	if err := events.SetupAndRecordEvent(c.closeContext, c.recorder, event); err != nil {
		c.logger.WarnContext(c.closeContext, "Failed to record event", "error", err, "event_type", event.GetType())
	}
}

// close is called when the chunk expires from the cache. The recorder is closed
// after a short delay so requests that got the chunk right before it expired
// can still record.
func (c *sessionChunk) close() error {
	select {
	case <-c.clock.After(time.Minute):
	case <-c.closeContext.Done():
	}
	return trace.Wrap(c.recorder.Close(c.closeContext))
}
