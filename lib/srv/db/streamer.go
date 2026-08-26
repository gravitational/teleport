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

package db

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/recorder"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

// newSessionRecorder creates a streamer that will be used to stream the
// requests that occur within this session to the audit log.
func (s *Server) newSessionRecorder(sessionCtx *common.Session) (libevents.SessionPreparerRecorder, error) {
	recConfig, err := s.cfg.AccessPoint.GetSessionRecordingConfig(s.connContext)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clusterName, err := s.cfg.AccessPoint.GetClusterName(s.connContext)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return recorder.New(recorder.Config{
		SessionID:    session.ID(sessionCtx.ID),
		ServerID:     sessionCtx.HostID,
		Namespace:    apidefaults.Namespace,
		Clock:        s.cfg.Clock,
		ClusterName:  clusterName.GetClusterName(),
		RecordingCfg: recConfig,
		SyncStreamer: s.cfg.AuthClient,
		DataDir:      s.cfg.DataDir,
		Component:    teleport.Component(teleport.ComponentSession, teleport.ComponentDatabase),
		// Session stream is using server context, not session context,
		// to make sure that session is uploaded even after it is closed
		Context: s.connContext,
	})
}
