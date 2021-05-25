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

package workflows_test

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/client/workflows"
	"github.com/gravitational/teleport/api/types"
	"github.com/stretchr/testify/require"
)

// mockWatcher is a mock types.Watcher.
type mockWatcher struct {
	eventsc chan types.Event
	done    chan struct{}
}

func newMockWatcher() *mockWatcher {
	return &mockWatcher{
		eventsc: make(chan types.Event, 1),
		done:    make(chan struct{}),
	}
}

func (m *mockWatcher) Events() <-chan types.Event {
	return m.eventsc
}

func (m *mockWatcher) Done() <-chan struct{} {
	return m.done
}

func (m *mockWatcher) Close() error {
	close(m.done)
	close(m.eventsc)
	return nil
}

func (m *mockWatcher) Error() error {
	return nil
}

func TestWaitInit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mockWatcher := newMockWatcher()
	clt := newMockClient(mockWatcher)

	watcher, err := workflows.NewRequestWatcher(ctx, clt, types.AccessRequestFilter{})
	require.NoError(t, err)

	// WaitInit should fail when the ctx is canceled and OpInit is not receieved.
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	require.Error(t, watcher.WaitInit(timeoutCtx))

	// Watcher should successfully wait for and consume OpInit event.
	mockWatcher.eventsc <- types.Event{Type: types.OpInit}
	require.NoError(t, watcher.WaitInit(ctx))
	select {
	case <-watcher.Events():
		t.Error("OpInit event should have been consumed by WaitInit.")
	default:
	}
}

func TestEvents(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tests := []struct {
		desc      string
		event     types.Event
		expectErr string
	}{
		{
			desc: "Send OpPut AccessRequest event should succeed",
			event: types.Event{
				Resource: &types.AccessRequestV3{},
				Type:     types.OpPut,
			},
		}, {
			desc: "Send OpDelete AccessRequest event should succeed",
			event: types.Event{
				Resource: &types.AccessRequestV3{},
				Type:     types.OpDelete,
			},
		}, {
			desc: "Sending unexpected OpType in event should fail",
			event: types.Event{
				Resource: &types.AccessRequestV3{},
				Type:     types.OpGet,
			},
			expectErr: "unexpected event op type",
		}, {
			desc: "Sending unexpected Resource in event should fail",
			event: types.Event{
				Resource: &types.UserV2{},
				Type:     types.OpDelete,
			},
			expectErr: "unexpected resource type",
		},
		{
			desc: "OpInit event should be consumed automatically",
			event: types.Event{
				Type: types.OpInit,
			},
			expectErr: "context deadline exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			// create new client and send the test event
			mockWatcher := newMockWatcher()
			mockWatcher.eventsc <- tt.event
			clt := newMockClient(mockWatcher)

			watcher, err := workflows.NewRequestWatcher(ctx, clt, types.AccessRequestFilter{})
			require.NoError(t, err)

			timeoutCtx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()

			if tt.expectErr != "" {
				select {
				case <-watcher.Done():
					require.Contains(t, watcher.Error().Error(), tt.expectErr)
				case <-timeoutCtx.Done():
					require.Contains(t, timeoutCtx.Err().Error(), tt.expectErr)
				}
				return
			}

			select {
			case <-watcher.Done():
			case event := <-watcher.Events():
				require.Equal(t, event.Type, tt.event.Type)
				require.Equal(t, tt.event.Resource.(*types.AccessRequestV3), event.Request)
			case <-timeoutCtx.Done():
				t.Errorf("watcher is stagnant, expected to receive event")
			}
		})
	}
}
