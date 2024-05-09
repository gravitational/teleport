/*
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package accessgraph

import (
	"context"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/events"
)

type mockTagEventWatcher struct {
	events    []*accessgraphv1alpha.EventsStreamV2Request
	eventsMtx sync.Mutex
}

func (m *mockTagEventWatcher) Send(event *accessgraphv1alpha.EventsStreamV2Request) error {
	m.eventsMtx.Lock()
	defer m.eventsMtx.Unlock()

	m.events = append(m.events, event)
	return nil
}

func unpackEvent(t *testing.T, event *accessgraphv1alpha.EventsStreamV2Request) *types.ServerV2 {
	t.Helper()

	require.NotNil(t, event)

	operation, ok := event.Operation.(*accessgraphv1alpha.EventsStreamV2Request_Upsert)
	require.True(t, ok)

	// assert that there is only one resource
	resources := operation.Upsert.Resources
	require.Len(t, resources, 1)

	// assert that the resource is a Server
	server, ok := resources[0].Resource.(*accessgraphv1alpha.ResourceEntry_Server)
	require.True(t, ok)

	return server.Server
}

func Test_tagEventWatcher_Send(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mock := &mockTagEventWatcher{}
	eventWatcher := newTagEventWatcher(ctx, mock)

	// Init should be ignored
	err := eventWatcher.Send(&proto.Event{Type: proto.Operation_INIT})
	require.NoError(t, err)

	err = eventWatcher.Send(&proto.Event{Type: proto.Operation_PUT,
		Resource: &proto.Event_Server{Server: &types.ServerV2{Metadata: types.Metadata{Name: "1"}}},
	})
	require.NoError(t, err)

	err = eventWatcher.markReady()
	require.NoError(t, err)

	err = eventWatcher.Send(&proto.Event{Type: proto.Operation_PUT,
		Resource: &proto.Event_Server{Server: &types.ServerV2{Metadata: types.Metadata{Name: "2"}}},
	})
	require.NoError(t, err)

	require.Len(t, mock.events, 2)
	require.Equal(t, "1", unpackEvent(t, mock.events[0]).GetName())
	require.Equal(t, "2", unpackEvent(t, mock.events[1]).GetName())
}

func Test_tagEventWatcher_Send_Concurrent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mock := &mockTagEventWatcher{}
	eventWatcher := newTagEventWatcher(ctx, mock)

	// Init should be ignored
	err := eventWatcher.Send(&proto.Event{Type: proto.Operation_INIT})
	require.NoError(t, err)

	for i := 0; i < 100; i++ {
		err := eventWatcher.Send(&proto.Event{Type: proto.Operation_PUT,
			Resource: &proto.Event_Server{Server: &types.ServerV2{Metadata: types.Metadata{Name: strconv.Itoa(i)}}},
		})
		assert.NoError(t, err)
	}

	// Watcher is not ready yet, so no events should be sent
	require.Empty(t, mock.events)

	wg := sync.WaitGroup{}
	wg.Add(1)
	// Send a bunch of events concurrently
	go func() {
		defer wg.Done()

		for i := 100; i < 200; i++ {
			err := eventWatcher.Send(&proto.Event{Type: proto.Operation_PUT,
				Resource: &proto.Event_Server{Server: &types.ServerV2{Metadata: types.Metadata{Name: strconv.Itoa(i)}}},
			})
			assert.NoError(t, err)
		}
	}()

	// Mark ready. This should flush the cache and send all events
	err = eventWatcher.markReady()
	require.NoError(t, err)

	// wait for all events to be sent
	wg.Wait()

	require.Len(t, mock.events, 200)

	// All events should be in order
	for i := 0; i < 200; i++ {
		require.Equal(t, strconv.Itoa(i), unpackEvent(t, mock.events[i]).GetName())
	}
}

func TestConvertEvent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		inputEvent *accessgraphv1alpha.AuditEvent
		validate   func(t *testing.T, outputEvent apievents.AuditEvent)
	}{
		{
			name: "nil event",
			inputEvent: &accessgraphv1alpha.AuditEvent{
				Event: nil,
			},
			validate: func(t *testing.T, outputEvent apievents.AuditEvent) {
				require.Nil(t, outputEvent)
			},
		},
		{
			name: "AccessPathChanged event",
			inputEvent: &accessgraphv1alpha.AuditEvent{
				Event: &accessgraphv1alpha.AuditEvent_AccessPathChanged{
					AccessPathChanged: &accessgraphv1alpha.AccessPathChanged{
						ChangeId:               "sample-change-id",
						AffectedResourceName:   "sample-resource-name",
						AffectedResourceSource: "sample-resource-source",
					},
				},
			},
			validate: func(t *testing.T, outputEvent apievents.AuditEvent) {
				require.NotNil(t, outputEvent)
				require.Equal(t, events.AccessGraphAccessPathChanged, outputEvent.GetType())
				require.Equal(t, events.AccessGraphAccessPathChangedCode, outputEvent.GetCode())

				accessPathEvent, ok := outputEvent.(*apievents.AccessPathChanged)
				require.True(t, ok)
				require.Equal(t, "sample-change-id", accessPathEvent.ChangeID)
				require.Equal(t, "sample-resource-name", accessPathEvent.AffectedResourceName)
				require.Equal(t, "sample-resource-source", accessPathEvent.AffectedResourceSource)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auditEvent := convertEvent(tt.inputEvent)
			tt.validate(t, auditEvent)
		})
	}

}
