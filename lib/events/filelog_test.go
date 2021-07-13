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
	require.Nil(t, err)

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
	require.Nil(t, err)

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
	require.Nil(t, err)

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
	require.Nil(t, err)

	from := clock.Now().Add(-time.Hour).UTC()
	to := clock.Now().Add(time.Hour).UTC()
	eventArr, checkpoint, err := log.SearchEvents(from, to, apidefaults.Namespace, nil, 2, types.EventOrderAscending, "")
	require.Nil(t, err)
	require.Len(t, eventArr, 2)
	require.NotEqual(t, checkpoint, "")

	eventArr, checkpoint, err = log.SearchEvents(from, to, apidefaults.Namespace, nil, 2, types.EventOrderAscending, checkpoint)
	require.Nil(t, err)
	require.Len(t, eventArr, 1)
	require.Equal(t, checkpoint, "")
}
