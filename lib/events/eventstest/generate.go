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

package eventstest

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/events"
)

// SessionParams specifies optional parameters
// for generated session
type SessionParams struct {
	// PrintEvents sets up print events count. Ignored if PrintData is set.
	PrintEvents int64
	// PrintData is optional data to use for print events. Each element of the
	// slice represents data for one print event.
	PrintData []string
	// Clock is an optional clock setting start
	// and offset time of the event
	Clock clockwork.Clock
	// ServerID is an optional server ID
	ServerID string
	// SessionID is an optional session ID to set
	SessionID string
	// ClusterName is an optional originating cluster name
	ClusterName string
	// UserName is name of the user interacting with the session
	UserName string
}

// SetDefaults sets parameters defaults
func (p *SessionParams) SetDefaults() {
	if p.Clock == nil {
		p.Clock = clockwork.NewFakeClockAt(
			time.Date(2020, 0o3, 30, 15, 58, 54, 561*int(time.Millisecond), time.UTC))
	}
	if p.ServerID == "" {
		p.ServerID = uuid.New().String()
	}
	if p.SessionID == "" {
		p.SessionID = uuid.New().String()
	}
	if p.PrintData == nil {
		p.PrintData = make([]string, p.PrintEvents)
		for i := range p.PrintEvents {
			p.PrintData[i] = strings.Repeat("hello", int(i%177+1))
		}
	}
	if p.UserName == "" {
		p.UserName = "alice@example.com"
	}
}

// GenerateTestSession generates test session events starting with session start
// event, adds printEvents events and returns the result.
func GenerateTestSession(params SessionParams) []apievents.AuditEvent {
	params.SetDefaults()
	connectionMetadata := apievents.ConnectionMetadata{
		LocalAddr:  "127.0.0.1:3022",
		RemoteAddr: "[::1]:37718",
		Protocol:   events.EventProtocolSSH,
	}
	sessionStart := apievents.SessionStart{
		Metadata: apievents.Metadata{
			Index:       0,
			Type:        events.SessionStartEvent,
			ID:          "36cee9e9-9a80-4c32-9163-3d9241cdac7a",
			Code:        events.SessionStartCode,
			Time:        params.Clock.Now().UTC(),
			ClusterName: params.ClusterName,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerVersion: teleport.Version,
			ServerID:      params.ServerID,
			ServerLabels: map[string]string{
				"kernel": "5.3.0-42-generic",
				"date":   "Mon Mar 30 08:58:54 PDT 2020",
				"group":  "gravitational/devc",
			},
			ServerHostname:  "planet",
			ServerNamespace: "default",
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: params.SessionID,
		},
		UserMetadata: apievents.UserMetadata{
			User:  params.UserName,
			Login: "bob",
		},
		ConnectionMetadata: connectionMetadata,
		TerminalSize:       "80:25",
	}

	sessionEnd := apievents.SessionEnd{
		Metadata: apievents.Metadata{
			Index:       20,
			Type:        events.SessionEndEvent,
			ID:          "da455e0f-c27d-459f-a218-4e83b3db9426",
			Code:        events.SessionEndCode,
			Time:        params.Clock.Now().UTC().Add(time.Hour + time.Second + 7*time.Millisecond),
			ClusterName: params.ClusterName,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerVersion:   teleport.Version,
			ServerID:        params.ServerID,
			ServerNamespace: "default",
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: params.SessionID,
		},
		UserMetadata: apievents.UserMetadata{
			User: params.UserName,
		},
		ConnectionMetadata: connectionMetadata,
		EnhancedRecording:  true,
		Interactive:        true,
		Participants:       []string{params.UserName},
		StartTime:          params.Clock.Now().UTC(),
		EndTime:            params.Clock.Now().UTC().Add(3*time.Hour + time.Second + 7*time.Millisecond),
	}

	genEvents := []apievents.AuditEvent{&sessionStart}
	for i, data := range params.PrintData {
		event := &apievents.SessionPrint{
			Metadata: apievents.Metadata{
				Index: int64(i) + 1,
				Type:  events.SessionPrintEvent,
				Time:  params.Clock.Now().UTC().Add(time.Minute + time.Duration(i)*time.Millisecond),
			},
			ChunkIndex:        int64(i),
			DelayMilliseconds: int64(i),
			Offset:            int64(i),
			Data:              []byte(data),
		}
		event.Bytes = int64(len(event.Data))
		event.Time = event.Time.Add(time.Duration(i) * time.Millisecond)

		genEvents = append(genEvents, event)
	}

	sessionEnd.Metadata.Index = int64(len(genEvents))
	genEvents = append(genEvents, &sessionEnd)

	return genEvents
}

// GenerateTestKubeSession generates Kubernetes test session events starting
// with session start event, adds printEvents events and returns the result.
func GenerateTestKubeSession(params SessionParams) []apievents.AuditEvent {
	params.SetDefaults()
	connectionMetadata := apievents.ConnectionMetadata{
		LocalAddr:  "127.0.0.1:3022",
		RemoteAddr: "[::1]:37718",
		Protocol:   events.EventProtocolKube,
	}
	kubernetesClusterMetadata := apievents.KubernetesClusterMetadata{
		KubernetesCluster: "my-kube-cluster",
		KubernetesUsers:   []string{"admin"},
		KubernetesGroups:  []string{"viewers"},
		KubernetesLabels: map[string]string{
			"teleport.internal/resource-id": "ed910b7b-fe3b-4959-bf2e-ac45f4648f2a",
		},
	}
	kubernetesPodMetadata := apievents.KubernetesPodMetadata{
		KubernetesPodName:        "simple-shell-pod",
		KubernetesPodNamespace:   "default",
		KubernetesContainerName:  "shell-container",
		KubernetesContainerImage: "busybox",
		KubernetesNodeName:       "docker-desktop",
	}
	sessionStart := apievents.SessionStart{
		Metadata: apievents.Metadata{
			Index:       0,
			Type:        events.SessionStartEvent,
			ID:          "36cee9e9-9a80-4c32-9163-3d9241cdac7a",
			Code:        events.SessionStartCode,
			Time:        params.Clock.Now().UTC(),
			ClusterName: params.ClusterName,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerVersion: teleport.Version,
			ServerID:      params.ServerID,
			ServerLabels: map[string]string{
				"teleport.internal/resource-id": "ed910b7b-fe3b-4959-bf2e-ac45f4648f2a",
			},
			ServerHostname:  "planet",
			ServerNamespace: "default",
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: params.SessionID,
		},
		UserMetadata: apievents.UserMetadata{
			User:  params.UserName,
			Login: "bob",
		},
		ConnectionMetadata:        connectionMetadata,
		TerminalSize:              "80:25",
		KubernetesClusterMetadata: kubernetesClusterMetadata,
		KubernetesPodMetadata:     kubernetesPodMetadata,
	}

	sessionEnd := apievents.SessionEnd{
		Metadata: apievents.Metadata{
			Index:       20,
			Type:        events.SessionEndEvent,
			ID:          "da455e0f-c27d-459f-a218-4e83b3db9426",
			Code:        events.SessionEndCode,
			Time:        params.Clock.Now().UTC().Add(time.Hour + time.Second + 7*time.Millisecond),
			ClusterName: params.ClusterName,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerVersion:   teleport.Version,
			ServerID:        params.ServerID,
			ServerNamespace: "default",
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: params.SessionID,
		},
		UserMetadata: apievents.UserMetadata{
			User: params.UserName,
		},
		ConnectionMetadata:        connectionMetadata,
		EnhancedRecording:         true,
		Interactive:               true,
		Participants:              []string{params.UserName},
		StartTime:                 params.Clock.Now().UTC(),
		EndTime:                   params.Clock.Now().UTC().Add(3*time.Hour + time.Second + 7*time.Millisecond),
		KubernetesClusterMetadata: kubernetesClusterMetadata,
		KubernetesPodMetadata:     kubernetesPodMetadata,
	}

	genEvents := []apievents.AuditEvent{&sessionStart}
	for i, data := range params.PrintData {
		event := &apievents.SessionPrint{
			Metadata: apievents.Metadata{
				Index: int64(i) + 1,
				Type:  events.SessionPrintEvent,
				Time:  params.Clock.Now().UTC().Add(time.Minute + time.Duration(i)*time.Millisecond),
			},
			ChunkIndex:        int64(i),
			DelayMilliseconds: int64(i),
			Offset:            int64(i),
			Data:              []byte(data),
		}
		event.Bytes = int64(len(event.Data))
		event.Time = event.Time.Add(time.Duration(i) * time.Millisecond)

		genEvents = append(genEvents, event)
	}

	sessionEnd.Metadata.Index = int64(len(genEvents))
	genEvents = append(genEvents, &sessionEnd)

	return genEvents
}

// DBSessionParams specifies optional parameters
// for a generated database session.
type DBSessionParams struct {
	// Queries is the number of queries to generate.
	Queries int64
	// Clock is an optional clock setting start
	// and offset time of the event
	Clock clockwork.Clock
	// ServerID is an optional server ID
	ServerID string
	// DatabaseService is an optional database service name. (Caveat: this is
	// actually the database resource name, not a database service, but that's
	// how the event field is named.)
	DatabaseService string
	// SessionID is an optional session ID to set
	SessionID string
	// ClusterName is an optional originating cluster name
	ClusterName string
	// UserName is name of the user interacting with the session
	UserName string
}

// SetDefaults sets parameters defaults
func (p *DBSessionParams) SetDefaults() {
	if p.Clock == nil {
		p.Clock = clockwork.NewFakeClockAt(
			time.Date(2020, 0o3, 30, 15, 58, 54, 561*int(time.Millisecond), time.UTC))
	}
	if p.ServerID == "" {
		p.ServerID = uuid.New().String()
	}
	if p.DatabaseService == "" {
		p.DatabaseService = "testdb"
	}
	if p.SessionID == "" {
		p.SessionID = uuid.New().String()
	}
	if p.UserName == "" {
		p.UserName = "bob@example.com"
	}
}

// GenerateTestDBSession generates test database session events starting with
// session start event, adds params.Queries events and returns the result.
func GenerateTestDBSession(params DBSessionParams) []apievents.AuditEvent {
	params.SetDefaults()

	startTime := params.Clock.Now().UTC()
	endTime := startTime.Add(time.Minute)
	userMetadata := apievents.UserMetadata{
		User:     params.UserName,
		UserKind: apievents.UserKind_USER_KIND_HUMAN,
	}
	sessionMetadata := apievents.SessionMetadata{
		SessionID:        params.SessionID,
		PrivateKeyPolicy: string(keys.PrivateKeyPolicyNone),
	}
	databaseMetadata := apievents.DatabaseMetadata{
		DatabaseService:  params.DatabaseService,
		DatabaseProtocol: types.DatabaseProtocolPostgreSQL,
		DatabaseURI:      "localhost:5432",
		DatabaseName:     "Northwind",
		DatabaseUser:     "postgres",
		DatabaseType:     types.DatabaseTypeSelfHosted,
		DatabaseOrigin:   types.OriginConfigFile,
	}

	sessionStart := apievents.DatabaseSessionStart{
		Metadata: apievents.Metadata{
			Index:       0,
			Type:        events.DatabaseSessionStartEvent,
			ID:          "3f2876b7-6467-4741-8dc4-43c133bdd748",
			Code:        events.DatabaseSessionStartCode,
			Time:        startTime,
			ClusterName: params.ClusterName,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerVersion:   teleport.Version,
			ServerID:        params.ServerID,
			ServerNamespace: "default",
		},
		UserMetadata:     userMetadata,
		SessionMetadata:  sessionMetadata,
		DatabaseMetadata: databaseMetadata,
		Status: apievents.Status{
			Success: true,
		},
		PostgresPID: 12345,
		ClientMetadata: apievents.ClientMetadata{
			UserAgent: "psql",
		},
	}

	sessionEnd := apievents.DatabaseSessionEnd{
		Metadata: apievents.Metadata{
			Index:       20,
			Type:        events.DatabaseSessionEndEvent,
			ID:          "9ee71c9-e509-47ed-bbac-16b7a38b660d",
			Code:        events.DatabaseSessionEndCode,
			Time:        endTime,
			ClusterName: params.ClusterName,
		},
		UserMetadata:     userMetadata,
		SessionMetadata:  sessionMetadata,
		DatabaseMetadata: databaseMetadata,
		StartTime:        startTime,
		EndTime:          endTime,
		Participants:     []string{userMetadata.User},
	}

	genEvents := []apievents.AuditEvent{&sessionStart}
	for i := range params.Queries {
		query := &apievents.DatabaseSessionQuery{
			Metadata: apievents.Metadata{
				Index:       i*2 + 1,
				Type:        events.DatabaseSessionQueryEvent,
				ID:          uuid.New().String(),
				Code:        events.DatabaseSessionQueryCode,
				Time:        startTime.Add(time.Minute + time.Duration(i*2)*time.Millisecond),
				ClusterName: params.ClusterName,
			},
			UserMetadata:     userMetadata,
			SessionMetadata:  sessionMetadata,
			DatabaseMetadata: databaseMetadata,
			DatabaseQuery:    fmt.Sprintf("SELECT order_id FROM order where customer_id=%d", i),
			Status: apievents.Status{
				Success: true,
			},
		}

		result := &apievents.DatabaseSessionCommandResult{
			Metadata: apievents.Metadata{
				Index:       i*2 + 2,
				Type:        events.DatabaseSessionCommandResultEvent,
				ID:          uuid.New().String(),
				Code:        events.DatabaseSessionCommandResultCode,
				Time:        startTime.Add(time.Minute + time.Duration(i*2+1)*time.Millisecond),
				ClusterName: params.ClusterName,
			},
			UserMetadata:     userMetadata,
			SessionMetadata:  sessionMetadata,
			DatabaseMetadata: databaseMetadata,
			Status: apievents.Status{
				Success: true,
			},
			AffectedRecords: 10,
		}

		genEvents = append(genEvents, query, result)
	}

	sessionEnd.Metadata.Index = int64(len(genEvents))
	genEvents = append(genEvents, &sessionEnd)

	return genEvents
}
