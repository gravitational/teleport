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

package db

import (
	"context"
	"path/filepath"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/filesessions"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv/db/common"

	"github.com/gravitational/trace"
)

// newStreamWriter creates a streamer that will be used to stream the
// requests that occur within this session to the audit log.
func (s *Server) newStreamWriter(sessionCtx *common.Session) (libevents.StreamWriter, error) {
	recConfig, err := s.cfg.AccessPoint.GetSessionRecordingConfig(s.closeContext)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clusterName, err := s.cfg.AccessPoint.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// TODO(r0mant): Add support for record-at-proxy.
	// Create a sync or async streamer depending on configuration of cluster.
	streamer, err := s.newStreamer(s.closeContext, sessionCtx.ID, recConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Audit stream is using server context, not session context,
	// to make sure that session is uploaded even after it is closed
	return libevents.NewAuditWriter(libevents.AuditWriterConfig{
		Context:      s.closeContext,
		Streamer:     streamer,
		Clock:        s.cfg.Clock,
		SessionID:    session.ID(sessionCtx.ID),
		Namespace:    apidefaults.Namespace,
		ServerID:     sessionCtx.HostID,
		RecordOutput: recConfig.GetMode() != types.RecordOff,
		Component:    teleport.ComponentDatabase,
		ClusterName:  clusterName.GetClusterName(),
	})
}

// newStreamer returns sync or async streamer based on the configuration
// of the server and the session, sync streamer sends the events
// directly to the auth server and blocks if the events can not be received,
// async streamer buffers the events to disk and uploads the events later
func (s *Server) newStreamer(ctx context.Context, sessionID string, recConfig types.SessionRecordingConfig) (libevents.Streamer, error) {
	if services.IsRecordSync(recConfig.GetMode()) {
		s.log.Debugf("Using sync streamer for session %v.", sessionID)
		return s.cfg.AuthClient, nil
	}
	s.log.Debugf("Using async streamer for session %v.", sessionID)
	uploadDir := filepath.Join(
		s.cfg.DataDir, teleport.LogsDir, teleport.ComponentUpload,
		libevents.StreamingSessionsDir, apidefaults.Namespace)
	fileStreamer, err := filesessions.NewStreamer(uploadDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return libevents.NewTeeStreamer(fileStreamer, s.cfg.StreamEmitter), nil
}
