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

package eventstest

import (
	"bytes"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
)

// SessionParams specifies optional parameters
// for generated session
type SessionParams struct {
	// PrintEvents sets up print events count
	PrintEvents int64
	// Clock is an optional clock setting start
	// and offset time of the event
	Clock clockwork.Clock
	// ServerID is an optional server ID
	ServerID string
	// SessionID is an optional session ID to set
	SessionID string
	// ClusterName is an optional originating cluster name
	ClusterName string
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
}

// GenerateTestSession generates test session events starting with session start
// event, adds printEvents events and returns the result.
func GenerateTestSession(params SessionParams) []apievents.AuditEvent {
	params.SetDefaults()
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
			ServerID: params.ServerID,
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
			User:  "bob@example.com",
			Login: "bob",
		},
		ConnectionMetadata: apievents.ConnectionMetadata{
			LocalAddr:  "127.0.0.1:3022",
			RemoteAddr: "[::1]:37718",
		},
		TerminalSize: "80:25",
	}

	sessionEnd := apievents.SessionEnd{
		Metadata: apievents.Metadata{
			Index: 20,
			Type:  events.SessionEndEvent,
			ID:    "da455e0f-c27d-459f-a218-4e83b3db9426",
			Code:  events.SessionEndCode,
			Time:  params.Clock.Now().UTC().Add(time.Hour + time.Second + 7*time.Millisecond),
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerID:        params.ServerID,
			ServerNamespace: "default",
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: params.SessionID,
		},
		UserMetadata: apievents.UserMetadata{
			User: "alice@example.com",
		},
		EnhancedRecording: true,
		Interactive:       true,
		Participants:      []string{"alice@example.com"},
		StartTime:         params.Clock.Now().UTC(),
		EndTime:           params.Clock.Now().UTC().Add(3*time.Hour + time.Second + 7*time.Millisecond),
	}

	genEvents := []apievents.AuditEvent{&sessionStart}
	i := int64(0)
	for i = 0; i < params.PrintEvents; i++ {
		event := &apievents.SessionPrint{
			Metadata: apievents.Metadata{
				Index: i + 1,
				Type:  events.SessionPrintEvent,
				Time:  params.Clock.Now().UTC().Add(time.Minute + time.Duration(i)*time.Millisecond),
			},
			ChunkIndex:        i,
			DelayMilliseconds: i,
			Offset:            i,
			Data:              bytes.Repeat([]byte("hello"), int(i%177+1)),
		}
		event.Bytes = int64(len(event.Data))
		event.Time = event.Time.Add(time.Duration(i) * time.Millisecond)

		genEvents = append(genEvents, event)
	}

	i++
	sessionEnd.Metadata.Index = i
	genEvents = append(genEvents, &sessionEnd)

	return genEvents
}
