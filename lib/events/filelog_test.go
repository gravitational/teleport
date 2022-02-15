/*
Copyright 2021 Gravitational, Inc.

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

package events

import (
	"context"
	"testing"
	"time"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestFileLogPagination(t *testing.T) {
	clock := clockwork.NewFakeClock()

	log, err := NewFileLog(FileLogConfig{
		Dir:            t.TempDir(),
		RotationPeriod: time.Hour * 24,
		Clock:          clock,
	})
	require.NoError(t, err)

	err = log.EmitAuditEvent(context.TODO(), &events.SessionJoin{
		Metadata: events.Metadata{
			ID:   "a",
			Type: SessionJoinEvent,
			Time: clock.Now().UTC(),
		},
		UserMetadata: events.UserMetadata{
			User: "bob",
		},
	})
	require.NoError(t, err)

	err = log.EmitAuditEvent(context.TODO(), &events.SessionJoin{
		Metadata: events.Metadata{
			ID:   "b",
			Type: SessionJoinEvent,
			Time: clock.Now().Add(time.Minute).UTC(),
		},
		UserMetadata: events.UserMetadata{
			User: "alice",
		},
	})
	require.NoError(t, err)

	err = log.EmitAuditEvent(context.TODO(), &events.SessionJoin{
		Metadata: events.Metadata{
			ID:   "c",
			Type: SessionJoinEvent,
			Time: clock.Now().Add(time.Minute * 2).UTC(),
		},
		UserMetadata: events.UserMetadata{
			User: "dave",
		},
	})
	require.NoError(t, err)

	from := clock.Now().Add(-time.Hour).UTC()
	to := clock.Now().Add(time.Hour).UTC()
	eventArr, checkpoint, err := log.SearchEvents(from, to, apidefaults.Namespace, nil, 2, types.EventOrderAscending, "")
	require.NoError(t, err)
	require.Len(t, eventArr, 2)
	require.NotEmpty(t, checkpoint)

	eventArr, checkpoint, err = log.SearchEvents(from, to, apidefaults.Namespace, nil, 2, types.EventOrderAscending, checkpoint)
	require.Nil(t, err)
	require.Len(t, eventArr, 1)
	require.Empty(t, checkpoint)
}

func TestSearchSessionEvents(t *testing.T) {
	clock := clockwork.NewFakeClock()
	start := clock.Now()

	log, err := NewFileLog(FileLogConfig{
		Dir:            t.TempDir(),
		RotationPeriod: time.Hour * 24,
		Clock:          clock,
	})
	require.Nil(t, err)
	clock.Advance(1 * time.Minute)

	require.NoError(t, log.EmitAuditEvent(context.Background(), &events.SessionEnd{
		Metadata: events.Metadata{
			ID:   "a",
			Type: SessionEndEvent,
			Time: clock.Now(),
		},
	}))
	clock.Advance(1 * time.Minute)

	result, _, err := log.SearchSessionEvents(start, clock.Now(),
		10, // limit
		types.EventOrderAscending,
		"",  // startKey
		nil, // cond
	)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, result[0].GetType(), SessionEndEvent)
	require.Equal(t, result[0].GetID(), "a")

	// emit a non-session event, it should not show up in the next query
	require.NoError(t, log.EmitAuditEvent(context.Background(), &events.SessionJoin{
		Metadata: events.Metadata{
			ID:   "b",
			Type: SessionJoinEvent,
			Time: clock.Now(),
		},
	}))
	clock.Advance(1 * time.Minute)

	result, _, err = log.SearchSessionEvents(start, clock.Now(),
		10, // limit
		types.EventOrderAscending,
		"",  // startKey
		nil, // cond
	)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, result[0].GetType(), SessionEndEvent)
	require.Equal(t, result[0].GetID(), "a")

	// emit a desktop session event, it should show up in the next query
	require.NoError(t, log.EmitAuditEvent(context.Background(), &events.WindowsDesktopSessionEnd{
		Metadata: events.Metadata{
			ID:   "c",
			Type: WindowsDesktopSessionEndEvent,
			Time: clock.Now(),
		},
	}))
	clock.Advance(1 * time.Minute)

	result, _, err = log.SearchSessionEvents(start, clock.Now(),
		10, // limit
		types.EventOrderAscending,
		"",  // startKey
		nil, // cond
	)
	require.NoError(t, err)
	require.Len(t, result, 2)
	require.Equal(t, result[0].GetType(), SessionEndEvent)
	require.Equal(t, result[0].GetID(), "a")
	require.Equal(t, result[1].GetType(), WindowsDesktopSessionEndEvent)
	require.Equal(t, result[1].GetID(), "c")
}
