/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package events_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/session"
)

func TestFindSessionEndEvent(t *testing.T) {
	uploader := eventstest.NewMemoryUploader()
	alog, err := events.NewAuditLog(events.AuditLogConfig{
		DataDir:       t.TempDir(),
		ServerID:      "server1",
		UploadHandler: uploader,
	})
	require.NoError(t, err)
	t.Cleanup(func() { alog.Close() })

	sshSessionID := session.NewID()
	sshSessionEvents := eventstest.GenerateTestSession(eventstest.SessionParams{
		PrintEvents: 1000,
		UserName:    "bob",
		SessionID:   string(sshSessionID),
		ServerID:    "testcluster",
		PrintData:   []string{"ls", "whoami"},
	})

	kubernetesSessionID := session.NewID()
	kubernetesSessionEvents := eventstest.GenerateTestKubeSession(eventstest.SessionParams{
		UserName:    "carol",
		SessionID:   string(kubernetesSessionID),
		ServerID:    "testcluster",
		PrintData:   []string{"get pods", "describe pod"},
		PrintEvents: 1000,
	})

	databaseSessionID := session.NewID()
	databaseSessionEvents := eventstest.GenerateTestDBSession(eventstest.DBSessionParams{
		UserName:  "dave",
		SessionID: string(databaseSessionID),
		ServerID:  "testcluster",
		Queries:   1000,
	})

	tests := []struct {
		name        string // description of this test case
		sessionID   session.ID
		auditEvents []apievents.AuditEvent
		want        apievents.AuditEvent
		assertErr   require.ErrorAssertionFunc
	}{
		{
			name:        "SSH session with SessionEnd event",
			sessionID:   sshSessionID,
			auditEvents: sshSessionEvents,
			want:        sshSessionEvents[len(sshSessionEvents)-1],
			assertErr:   require.NoError,
		},
		{
			name:        "Kubernetes session with KubeSessionEnd event",
			sessionID:   kubernetesSessionID,
			auditEvents: kubernetesSessionEvents,
			want:        kubernetesSessionEvents[len(kubernetesSessionEvents)-1],
			assertErr:   require.NoError,
		},
		{
			name:        "Database session with DBSessionEnd event",
			sessionID:   databaseSessionID,
			auditEvents: databaseSessionEvents,
			want:        databaseSessionEvents[len(databaseSessionEvents)-1],
			assertErr:   require.NoError,
		},
		{
			name:        "No session end event",
			sessionID:   session.NewID(),
			auditEvents: eventstest.GenerateTestSession(eventstest.SessionParams{PrintEvents: 10})[:9],
			assertErr:   require.Error,
		},
		{
			name:      "missing session ID",
			sessionID: session.NewID(),
			want:      nil,
			assertErr: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.auditEvents) > 0 {
				streamer, err := events.NewProtoStreamer(
					events.ProtoStreamerConfig{
						Uploader: uploader,
					},
				)
				require.NoError(t, err)

				stream, err := streamer.CreateAuditStream(t.Context(), tt.sessionID)
				require.NoError(t, err)
				for _, event := range tt.auditEvents {
					require.NoError(t, stream.RecordEvent(t.Context(), eventstest.PrepareEvent(event)))
				}
				require.NoError(t, stream.Complete(t.Context()))
			}
			got, gotErr := events.FindSessionEndEvent(t.Context(), alog, tt.sessionID)
			tt.assertErr(t, gotErr)
			require.Equal(t, tt.want, got)
		})
	}
}
