/*
Copyright 2015-2021 Gravitational, Inc.

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

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	"github.com/gravitational/teleport/api/types/events"

	"github.com/gravitational/teleport/integrations/event-handler/lib"
)

func TestNew(t *testing.T) {
	e := &events.SessionPrint{
		Metadata: events.Metadata{
			ID:   "test",
			Type: "mock",
		},
	}

	protoEvent, err := eventToProto(events.AuditEvent(e))
	require.NoError(t, err)

	event, err := NewTeleportEvent(protoEvent)
	require.NoError(t, err)
	assert.Equal(t, "test", event.ID)
	assert.Equal(t, "mock", event.Type)
}

func TestGenID(t *testing.T) {
	e := &events.SessionPrint{}

	protoEvent, err := eventToProto(events.AuditEvent(e))
	require.NoError(t, err)

	event, err := NewTeleportEvent(protoEvent)
	require.NoError(t, err)
	assert.NotEmpty(t, event.ID)
}

func TestSessionEnd(t *testing.T) {
	e := &events.SessionUpload{
		Metadata: events.Metadata{
			Type: "session.upload",
		},
		SessionMetadata: events.SessionMetadata{
			SessionID: "session",
		},
	}

	protoEvent, err := eventToProto(events.AuditEvent(e))
	require.NoError(t, err)

	event, err := NewTeleportEvent(protoEvent)
	require.NoError(t, err)
	assert.NotEmpty(t, event.ID)
	assert.NotEmpty(t, event.SessionID)
	assert.True(t, event.IsSessionEnd)
}

func TestFailedLogin(t *testing.T) {
	e := &events.UserLogin{
		Metadata: events.Metadata{
			Type: "user.login",
		},
		Status: events.Status{
			Success: false,
		},
	}

	protoEvent, err := eventToProto(events.AuditEvent(e))
	require.NoError(t, err)

	event, err := NewTeleportEvent(protoEvent)
	require.NoError(t, err)
	assert.NotEmpty(t, event.ID)
	assert.True(t, event.IsFailedLogin)
}

func TestSuccessLogin(t *testing.T) {
	e := &events.UserLogin{
		Metadata: events.Metadata{
			Type: "user.login",
		},
		Status: events.Status{
			Success: true,
		},
	}

	protoEvent, err := eventToProto(events.AuditEvent(e))
	require.NoError(t, err)

	event, err := NewTeleportEvent(protoEvent)
	require.NoError(t, err)
	assert.NotEmpty(t, event.ID)
	assert.False(t, event.IsFailedLogin)
}

func eventToProto(e events.AuditEvent) (*auditlogpb.EventUnstructured, error) {
	data, err := lib.FastMarshal(e)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	str := &structpb.Struct{}
	if err = str.UnmarshalJSON(data); err != nil {
		return nil, trace.Wrap(err)
	}

	id := e.GetID()
	if id == "" {
		hash := sha256.Sum256(data)
		id = hex.EncodeToString(hash[:])
	}

	return &auditlogpb.EventUnstructured{
		Type:         e.GetType(),
		Unstructured: str,
		Id:           id,
		Index:        e.GetIndex(),
		Time:         timestamppb.New(e.GetTime()),
	}, nil
}

func eventsToProto(events []events.AuditEvent) ([]*auditlogpb.EventUnstructured, error) {
	protoEvents := make([]*auditlogpb.EventUnstructured, len(events))
	for i, event := range events {
		protoEvent, err := eventToProto(event)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		protoEvents[i] = protoEvent
	}
	return protoEvents, nil
}
