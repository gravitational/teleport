// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"testing"

	"github.com/peterbourgon/diskv/v3"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client"
	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
)

// TestConsumeSessionNoEventsFound tests that the consumeSession method returns without error
// if no events are found.
func TestConsumeSessionNoEventsFound(t *testing.T) {
	sessionID := "test"
	j := &SessionEventsJob{
		app: &App{
			Config: &StartCmdConfig{},
			EventWatcher: &TeleportEventsWatcher{
				client: &mockClient{},
			},
			State: &State{
				dv: diskv.New(diskv.Options{
					BasePath: t.TempDir(),
				}),
			},
		},
	}
	_, err := j.consumeSession(context.Background(), session{ID: sessionID})
	require.NoError(t, err)
}

type mockClient struct {
	client.Client
}

// StreamSessionEvents overrides the client.Client method to return a closed channel
// to ensure that the consumeSession method returns without error if no events are found.
func (m *mockClient) StreamUnstructuredSessionEvents(ctx context.Context, sessionID string, startIndex int64) (chan *auditlogpb.EventUnstructured, chan error) {
	c := make(chan *auditlogpb.EventUnstructured)
	e := make(chan error)
	close(c)
	return c, e
}
