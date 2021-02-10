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
	"os"
	"path/filepath"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/filesessions"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// newStreamWriter creates a streamer that will be used to stream the
// requests that occur within this session to the audit log.
func (s *Server) newStreamWriter(sessionCtx *common.Session) (libevents.StreamWriter, error) {
	clusterConfig, err := s.cfg.AccessPoint.GetClusterConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clusterName, err := s.cfg.AccessPoint.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// TODO(r0mant): Add support for record-at-proxy.
	// Create a sync or async streamer depending on configuration of cluster.
	streamer, err := s.newStreamer(s.closeContext, sessionCtx.ID, clusterConfig)
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
		Namespace:    defaults.Namespace,
		ServerID:     sessionCtx.Server.GetHostID(),
		RecordOutput: clusterConfig.GetSessionRecording() != services.RecordOff,
		Component:    teleport.ComponentDatabase,
		ClusterName:  clusterName.GetClusterName(),
	})
}

// newStreamer returns sync or async streamer based on the configuration
// of the server and the session, sync streamer sends the events
// directly to the auth server and blocks if the events can not be received,
// async streamer buffers the events to disk and uploads the events later
func (s *Server) newStreamer(ctx context.Context, sessionID string, clusterConfig services.ClusterConfig) (libevents.Streamer, error) {
	mode := clusterConfig.GetSessionRecording()
	if auth.IsRecordSync(mode) {
		s.log.Debugf("Using sync streamer for session %v.", sessionID)
		return s.cfg.AuthClient, nil
	}
	s.log.Debugf("Using async streamer for session %v.", sessionID)
	uploadDir := filepath.Join(
		s.cfg.DataDir, teleport.LogsDir, teleport.ComponentUpload,
		libevents.StreamingLogsDir, defaults.Namespace)
	// Make sure the upload dir exists, otherwise file streamer will fail.
	_, err := utils.StatDir(uploadDir)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if trace.IsNotFound(err) {
		s.log.Debugf("Creating upload dir %v.", uploadDir)
		if err := os.MkdirAll(uploadDir, 0755); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	fileStreamer, err := filesessions.NewStreamer(uploadDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return libevents.NewTeeStreamer(fileStreamer, s.cfg.StreamEmitter), nil
}
