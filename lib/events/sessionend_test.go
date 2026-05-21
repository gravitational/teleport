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
	"log/slog"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
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

func TestFindOrRecoverSessionEnd(t *testing.T) {
	const clusterName = "test-cluster"
	sessionID := session.NewID()
	clock := clockwork.NewFakeClock()

	userMeta := apievents.UserMetadata{User: "alice", Login: "root"}
	sessionMeta := apievents.SessionMetadata{SessionID: string(sessionID)}
	serverMeta := apievents.ServerMetadata{ServerID: "srv-1", ServerHostname: "host-1"}
	connMeta := apievents.ConnectionMetadata{LocalAddr: "127.0.0.1:3022", RemoteAddr: "10.0.0.1:9999"}
	dbMeta := apievents.DatabaseMetadata{DatabaseName: "mydb", DatabaseUser: "admin", DatabaseProtocol: "postgres"}
	appMeta := apievents.AppMetadata{AppName: "myapp", AppURI: "http://app.local"}

	startTime := clock.Now().UTC()
	lastTime := startTime.Add(time.Minute)

	makeConfig := func(streamer events.SessionStreamer, emitter apievents.Emitter) events.FindOrRecoverSessionEndConfig {
		return events.FindOrRecoverSessionEndConfig{
			ClusterName: clusterName,
			Streamer:    streamer,
			SessionID:   sessionID,
			AuditLog:    emitter,
			Log:         slog.Default(),
			Clock:       clock,
		}
	}

	t.Run("validation", func(t *testing.T) {
		base := makeConfig(eventstest.NewFakeStreamer(nil, 0), &eventstest.MockRecorderEmitter{})
		tests := []struct {
			name   string
			mutate func(*events.FindOrRecoverSessionEndConfig)
		}{
			{"missing ClusterName", func(c *events.FindOrRecoverSessionEndConfig) { c.ClusterName = "" }},
			{"missing Streamer", func(c *events.FindOrRecoverSessionEndConfig) { c.Streamer = nil }},
			{"missing SessionID", func(c *events.FindOrRecoverSessionEndConfig) { c.SessionID = "" }},
			{"missing AuditLog", func(c *events.FindOrRecoverSessionEndConfig) { c.AuditLog = nil }},
			{"missing Log", func(c *events.FindOrRecoverSessionEndConfig) { c.Log = nil }},
			{"missing Clock", func(c *events.FindOrRecoverSessionEndConfig) { c.Clock = nil }},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				c := base
				tt.mutate(&c)
				_, err := events.FindOrRecoverSessionEnd(t.Context(), c)
				require.Error(t, err)
			})
		}
	})

	type checkFn func(t *testing.T, gotEnd apievents.AuditEvent, emitted []apievents.AuditEvent)

	// alreadyExists returns a checkFn that asserts the exact start/end pair is
	// returned and that no event was emitted (because the end already existed).
	alreadyExists := func(wantEnd apievents.AuditEvent) checkFn {
		return func(t *testing.T, gotEnd apievents.AuditEvent, emitted []apievents.AuditEvent) {
			t.Helper()
			assert.Equal(t, wantEnd, gotEnd)
			assert.Empty(t, emitted, "expected no event to be emitted when end already exists")
		}
	}

	tests := []struct {
		name    string
		evts    []apievents.AuditEvent
		wantErr bool
		check   checkFn
	}{
		{
			name:    "no events",
			evts:    nil,
			wantErr: true,
		},
		{
			name: "no session start event",
			evts: []apievents.AuditEvent{
				&apievents.SessionPrint{Metadata: apievents.Metadata{Type: events.SessionPrintEvent, Time: lastTime}},
			},
			wantErr: true,
		},
		{
			name: "SSH/end already exists",
			evts: []apievents.AuditEvent{
				&apievents.SessionStart{
					Metadata:        apievents.Metadata{Type: events.SessionStartEvent, Time: startTime, ClusterName: clusterName},
					UserMetadata:    userMeta,
					SessionMetadata: sessionMeta,
				},
				&apievents.SessionEnd{
					Metadata:        apievents.Metadata{Type: events.SessionEndEvent, Time: lastTime, ClusterName: clusterName},
					UserMetadata:    userMeta,
					SessionMetadata: sessionMeta,
				},
			},
			check: alreadyExists(
				&apievents.SessionEnd{
					Metadata:        apievents.Metadata{Type: events.SessionEndEvent, Time: lastTime, ClusterName: clusterName},
					UserMetadata:    userMeta,
					SessionMetadata: sessionMeta,
				},
			),
		},
		{
			name: "SSH/end recovered",
			evts: []apievents.AuditEvent{
				&apievents.SessionStart{
					Metadata:           apievents.Metadata{Type: events.SessionStartEvent, Time: startTime, ClusterName: clusterName},
					UserMetadata:       userMeta,
					SessionMetadata:    sessionMeta,
					ServerMetadata:     serverMeta,
					ConnectionMetadata: connMeta,
					TerminalSize:       "80:25",
				},
				&apievents.SessionPrint{Metadata: apievents.Metadata{Type: events.SessionPrintEvent, Time: lastTime}},
			},
			check: func(t *testing.T, gotEnd apievents.AuditEvent, emitted []apievents.AuditEvent) {
				t.Helper()
				recovered, ok := gotEnd.(*apievents.SessionEnd)
				require.True(t, ok)
				assert.Equal(t, events.SessionEndEvent, recovered.Type)
				assert.Equal(t, events.SessionEndCode, recovered.Code)
				assert.Equal(t, userMeta, recovered.UserMetadata)
				assert.Equal(t, sessionMeta, recovered.SessionMetadata)
				assert.Equal(t, serverMeta, recovered.ServerMetadata)
				assert.Equal(t, connMeta, recovered.ConnectionMetadata)
				assert.True(t, recovered.Interactive)
				assert.Equal(t, lastTime, recovered.EndTime)
				assert.Len(t, emitted, 1)
			},
		},
		{
			name: "SSH/participants deduplicated",
			evts: []apievents.AuditEvent{
				&apievents.SessionStart{
					Metadata:        apievents.Metadata{Type: events.SessionStartEvent, Time: startTime, ClusterName: clusterName},
					UserMetadata:    userMeta,
					SessionMetadata: sessionMeta,
				},
				&apievents.SessionJoin{
					Metadata:     apievents.Metadata{Type: events.SessionJoinEvent, Time: startTime.Add(time.Second)},
					UserMetadata: userMeta, // same user — should be deduplicated
				},
				&apievents.SessionPrint{Metadata: apievents.Metadata{Type: events.SessionPrintEvent, Time: lastTime}},
			},
			check: func(t *testing.T, gotEnd apievents.AuditEvent, _ []apievents.AuditEvent) {
				t.Helper()
				recovered, ok := gotEnd.(*apievents.SessionEnd)
				require.True(t, ok)
				assert.Len(t, recovered.Participants, 1)
			},
		},
		{
			name: "Windows Desktop/end already exists",
			evts: []apievents.AuditEvent{
				&apievents.WindowsDesktopSessionStart{
					Metadata:        apievents.Metadata{Type: events.WindowsDesktopSessionStartEvent, Time: startTime, ClusterName: clusterName},
					UserMetadata:    userMeta,
					SessionMetadata: sessionMeta,
				},
				&apievents.WindowsDesktopSessionEnd{
					Metadata:        apievents.Metadata{Type: events.WindowsDesktopSessionEndEvent, Time: lastTime, ClusterName: clusterName},
					UserMetadata:    userMeta,
					SessionMetadata: sessionMeta,
				},
			},
			check: alreadyExists(
				&apievents.WindowsDesktopSessionEnd{
					Metadata:        apievents.Metadata{Type: events.WindowsDesktopSessionEndEvent, Time: lastTime, ClusterName: clusterName},
					UserMetadata:    userMeta,
					SessionMetadata: sessionMeta,
				},
			),
		},
		{
			name: "Windows Desktop/end recovered",
			evts: []apievents.AuditEvent{
				&apievents.WindowsDesktopSessionStart{
					Metadata:        apievents.Metadata{Type: events.WindowsDesktopSessionStartEvent, Time: startTime, ClusterName: clusterName},
					UserMetadata:    userMeta,
					SessionMetadata: sessionMeta,
					DesktopName:     "mydesktop",
					Domain:          "CORP",
				},
				&apievents.SessionPrint{Metadata: apievents.Metadata{Type: events.SessionPrintEvent, Time: lastTime}},
			},
			check: func(t *testing.T, gotEnd apievents.AuditEvent, emitted []apievents.AuditEvent) {
				t.Helper()
				recovered, ok := gotEnd.(*apievents.WindowsDesktopSessionEnd)
				require.True(t, ok)
				assert.Equal(t, events.WindowsDesktopSessionEndEvent, recovered.Type)
				assert.Equal(t, events.DesktopSessionEndCode, recovered.Code)
				assert.Equal(t, userMeta, recovered.UserMetadata)
				assert.Equal(t, sessionMeta, recovered.SessionMetadata)
				assert.Equal(t, "mydesktop (recovered)", recovered.DesktopName)
				assert.Equal(t, "CORP", recovered.Domain)
				assert.True(t, recovered.Recorded)
				assert.Equal(t, lastTime, recovered.EndTime)
				assert.Len(t, emitted, 1)
			},
		},
		{
			name: "Database/end already exists",
			evts: []apievents.AuditEvent{
				&apievents.DatabaseSessionStart{
					Metadata:         apievents.Metadata{Type: events.DatabaseSessionStartEvent, Time: startTime, ClusterName: clusterName},
					UserMetadata:     userMeta,
					SessionMetadata:  sessionMeta,
					DatabaseMetadata: dbMeta,
				},
				&apievents.DatabaseSessionEnd{
					Metadata:         apievents.Metadata{Type: events.DatabaseSessionEndEvent, Time: lastTime, ClusterName: clusterName},
					UserMetadata:     userMeta,
					SessionMetadata:  sessionMeta,
					DatabaseMetadata: dbMeta,
				},
			},
			check: alreadyExists(
				&apievents.DatabaseSessionEnd{
					Metadata:         apievents.Metadata{Type: events.DatabaseSessionEndEvent, Time: lastTime, ClusterName: clusterName},
					UserMetadata:     userMeta,
					SessionMetadata:  sessionMeta,
					DatabaseMetadata: dbMeta,
				},
			),
		},
		{
			name: "Database/end recovered",
			evts: []apievents.AuditEvent{
				&apievents.DatabaseSessionStart{
					Metadata:           apievents.Metadata{Type: events.DatabaseSessionStartEvent, Time: startTime, ClusterName: clusterName},
					UserMetadata:       userMeta,
					SessionMetadata:    sessionMeta,
					DatabaseMetadata:   dbMeta,
					ConnectionMetadata: connMeta,
				},
				&apievents.DatabaseSessionQuery{Metadata: apievents.Metadata{Type: events.DatabaseSessionQueryEvent, Time: lastTime}},
			},
			check: func(t *testing.T, gotEnd apievents.AuditEvent, emitted []apievents.AuditEvent) {
				t.Helper()
				recovered, ok := gotEnd.(*apievents.DatabaseSessionEnd)
				require.True(t, ok)
				assert.Equal(t, events.DatabaseSessionEndEvent, recovered.Type)
				assert.Equal(t, events.DatabaseSessionEndCode, recovered.Code)
				assert.Equal(t, userMeta, recovered.UserMetadata)
				assert.Equal(t, sessionMeta, recovered.SessionMetadata)
				assert.Equal(t, dbMeta, recovered.DatabaseMetadata)
				assert.Equal(t, connMeta, recovered.ConnectionMetadata)
				assert.Equal(t, startTime, recovered.StartTime)
				assert.Equal(t, lastTime, recovered.EndTime)
				assert.Len(t, emitted, 1)
			},
		},
		{
			name: "App/end already exists",
			evts: []apievents.AuditEvent{
				&apievents.AppSessionStart{
					Metadata:        apievents.Metadata{Type: events.AppSessionStartEvent, Time: startTime, ClusterName: clusterName},
					UserMetadata:    userMeta,
					SessionMetadata: sessionMeta,
					AppMetadata:     appMeta,
				},
				&apievents.AppSessionEnd{
					Metadata:        apievents.Metadata{Type: events.AppSessionEndEvent, Time: lastTime, ClusterName: clusterName},
					UserMetadata:    userMeta,
					SessionMetadata: sessionMeta,
					AppMetadata:     appMeta,
				},
			},
			check: alreadyExists(
				&apievents.AppSessionEnd{
					Metadata:        apievents.Metadata{Type: events.AppSessionEndEvent, Time: lastTime, ClusterName: clusterName},
					UserMetadata:    userMeta,
					SessionMetadata: sessionMeta,
					AppMetadata:     appMeta,
				},
			),
		},
		{
			name: "App/end recovered",
			evts: []apievents.AuditEvent{
				&apievents.AppSessionStart{
					Metadata:           apievents.Metadata{Type: events.AppSessionStartEvent, Time: startTime, ClusterName: clusterName},
					UserMetadata:       userMeta,
					SessionMetadata:    sessionMeta,
					ServerMetadata:     serverMeta,
					ConnectionMetadata: connMeta,
					AppMetadata:        appMeta,
				},
				&apievents.AppSessionChunk{Metadata: apievents.Metadata{Type: events.AppSessionChunkEvent, Time: lastTime}},
			},
			check: func(t *testing.T, gotEnd apievents.AuditEvent, emitted []apievents.AuditEvent) {
				t.Helper()
				recovered, ok := gotEnd.(*apievents.AppSessionEnd)
				require.True(t, ok)
				assert.Equal(t, events.AppSessionEndEvent, recovered.Type)
				assert.Equal(t, events.AppSessionEndCode, recovered.Code)
				assert.Equal(t, userMeta, recovered.UserMetadata)
				assert.Equal(t, sessionMeta, recovered.SessionMetadata)
				assert.Equal(t, serverMeta, recovered.ServerMetadata)
				assert.Equal(t, connMeta, recovered.ConnectionMetadata)
				assert.Equal(t, appMeta, recovered.AppMetadata)
				assert.Len(t, emitted, 1)
			},
		},
		{
			name: "MCP/end already exists",
			evts: []apievents.AuditEvent{
				&apievents.MCPSessionStart{
					Metadata:        apievents.Metadata{Type: events.MCPSessionStartEvent, Time: startTime, ClusterName: clusterName},
					UserMetadata:    userMeta,
					SessionMetadata: sessionMeta,
					AppMetadata:     appMeta,
				},
				&apievents.MCPSessionEnd{
					Metadata:        apievents.Metadata{Type: events.MCPSessionEndEvent, Time: lastTime, ClusterName: clusterName},
					UserMetadata:    userMeta,
					SessionMetadata: sessionMeta,
					AppMetadata:     appMeta,
				},
			},
			check: alreadyExists(
				&apievents.MCPSessionEnd{
					Metadata:        apievents.Metadata{Type: events.MCPSessionEndEvent, Time: lastTime, ClusterName: clusterName},
					UserMetadata:    userMeta,
					SessionMetadata: sessionMeta,
					AppMetadata:     appMeta,
				},
			),
		},
		{
			name: "MCP/end recovered",
			evts: []apievents.AuditEvent{
				&apievents.MCPSessionStart{
					Metadata:           apievents.Metadata{Type: events.MCPSessionStartEvent, Time: startTime, ClusterName: clusterName},
					UserMetadata:       userMeta,
					SessionMetadata:    sessionMeta,
					ServerMetadata:     serverMeta,
					ConnectionMetadata: connMeta,
					AppMetadata:        appMeta,
				},
				&apievents.SessionPrint{Metadata: apievents.Metadata{Type: events.SessionPrintEvent, Time: lastTime}},
			},
			check: func(t *testing.T, gotEnd apievents.AuditEvent, emitted []apievents.AuditEvent) {
				t.Helper()
				recovered, ok := gotEnd.(*apievents.MCPSessionEnd)
				require.True(t, ok)
				assert.Equal(t, events.MCPSessionEndEvent, recovered.Type)
				assert.Equal(t, events.MCPSessionEndCode, recovered.Code)
				assert.Equal(t, userMeta, recovered.UserMetadata)
				assert.Equal(t, sessionMeta, recovered.SessionMetadata)
				assert.Equal(t, serverMeta, recovered.ServerMetadata)
				assert.Equal(t, connMeta, recovered.ConnectionMetadata)
				assert.Equal(t, appMeta, recovered.AppMetadata)
				assert.Len(t, emitted, 1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emitter := &eventstest.MockRecorderEmitter{}
			cfg := makeConfig(eventstest.NewFakeStreamer(tt.evts, 0), emitter)
			gotEnd, err := events.FindOrRecoverSessionEnd(t.Context(), cfg)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			tt.check(t, gotEnd, emitter.Events())
		})
	}
}
