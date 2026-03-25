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
	"bytes"
	"context"
	"log/slog"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gravitational/trace"
	"github.com/peterbourgon/diskv/v3"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client"
	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
)

// TestConsumeSessionNoEventsFound tests that the consumeSession method returns without error
// if no events are found.
func TestConsumeSessionNoEventsFound(t *testing.T) {
	sessionID := "test"
	j := NewSessionEventsJob(&App{
		Config: &StartCmdConfig{},
		State: &State{
			dv: diskv.New(diskv.Options{
				BasePath: t.TempDir(),
			}),
		},
		client: &mockClient{},
		log:    slog.Default(),
	})
	_, err := j.consumeSession(t.Context(), session{ID: sessionID})
	require.NoError(t, err)
}

// TestIngestSession tests that the ingestSession method returns without error if a malformed
// session event is processed.
func TestIngestSession(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		startTime := time.Now().Add(-time.Minute)
		out := &bytes.Buffer{}
		log := slog.New(slog.NewTextHandler(out, &slog.HandlerOptions{Level: slog.LevelError}))

		j := NewSessionEventsJob(&App{
			Config: &StartCmdConfig{
				IngestConfig: IngestConfig{
					StorageDir:       t.TempDir(),
					Timeout:          time.Second,
					BatchSize:        100,
					Concurrency:      5,
					StartTime:        &startTime,
					SkipSessionTypes: map[string]struct{}{"print": {}, "desktop.recording": {}},
					WindowSize:       time.Hour * 24,
					DryRun:           true,
				},
			},
			State: &State{
				dv: diskv.New(diskv.Options{
					BasePath: t.TempDir(),
				}),
			},
			client: &mockClient{},
			log:    log,
		})

		j.processSessionFunc = func(ctx context.Context, s session, processingAttempt int) error {
			return trace.LimitExceeded("Session ingestion exceeded attempt limit")
		}

		err := j.ingestSession(t.Context(), session{ID: "test"}, 0, nil)
		require.NoError(t, err)

		synctest.Wait()

		require.Contains(t, out.String(), "Failed processing session recording")
		require.Contains(t, out.String(), "Session ingestion exceeded attempt limit")
	})
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
