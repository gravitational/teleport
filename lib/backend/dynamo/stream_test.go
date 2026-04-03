/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package dynamo_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/dynamo"
	"github.com/gravitational/teleport/lib/backend/dynamo/dynamodbtest"
)

func TestStream(t *testing.T) {
	t.Parallel()
	mockServer := dynamodbtest.NewMockDynamoDBServer()
	b := newTestBackend(t, mockServer)
	w := newTestWatcher(t, b)

	mockServer.SetShardState(dynamodbtest.ShardState{
		"shard-1": {ID: "shard-1", StartingSequenceNumber: 1},
	})
	waitForWatcherInit(t, w)

	t.Run("receives record added to existing shard", func(t *testing.T) {
		mockServer.AddRecord("shard-1", dynamodbtest.CreateStreamRecord("item1", 1))
		waitForEvent(t, w, "item1")
	})

	t.Run("receives many records", func(t *testing.T) {
		mockServer.AddRecord("shard-1", dynamodbtest.CreateStreamRecord("item2", 2))
		mockServer.AddRecord("shard-1", dynamodbtest.CreateStreamRecord("item3", 3))
		waitForEvent(t, w, "item2")
		waitForEvent(t, w, "item3")
		ensureNoEvents(t, w)
	})

	t.Run("shard discovery after parent shard closed", func(t *testing.T) {
		// TODO(smallinsky) Fix the underlying issue and enable this test.
		t.Skip("Skipping due to bug where LATEST iterator omits the records from the new shard")

		mockServer.UpsertShard(&dynamodbtest.Shard{
			ID:                     "shard-2",
			StartingSequenceNumber: 3,
			ParentShardID:          "shard-1",
		})
		mockServer.CloseShard("shard-1")
		mockServer.AddRecord("shard-2", dynamodbtest.CreateStreamRecord("item4", 4))
		waitForEvent(t, w, "item4")
		ensureNoEvents(t, w)
	})
}

func newTestBackend(t *testing.T, mockServer *dynamodbtest.Server) *dynamo.Backend {
	t.Helper()
	b, err := dynamo.NewFromConfig(t.Context(), &dynamo.Config{
		Region:           "us-east-1",
		AccessKey:        "test-key",
		SecretKey:        "test-secret",
		TableName:        "TestTable",
		RetryPeriod:      50 * time.Millisecond,
		PollStreamPeriod: 50 * time.Millisecond,
		BufferSize:       1000,
		HTTPClient: &http.Client{
			Transport: &muxToTransportWrapper{handler: mockServer.Mux},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { b.Close() })
	return b
}

func newTestWatcher(t *testing.T, b *dynamo.Backend) backend.Watcher {
	t.Helper()
	w, err := b.NewWatcher(t.Context(), backend.Watch{Prefixes: []backend.Key{backend.NewKey("")}})
	require.NoError(t, err)
	return w
}

func ensureNoEvents(t *testing.T, w backend.Watcher) {
	t.Helper()
	select {
	case event := <-w.Events():
		t.Fatalf("unexpected event: %#v", event)
	default:
	}
}

func waitForWatcherInit(t *testing.T, w backend.Watcher) {
	t.Helper()
	select {
	case event := <-w.Events():
		require.Equal(t, types.OpInit, event.Type)
	case <-time.After(2 * time.Second):
		t.Fatal("Did not receive init event in time")
	}
}

func waitForEvent(t *testing.T, w backend.Watcher, item string) {
	t.Helper()
	select {
	case event := <-w.Events():
		require.Equal(t, types.OpPut, event.Type)
		require.Equal(t, "/test/"+item, event.Item.Key.String())
	case <-time.After(2 * time.Second):
		t.Fatalf("Did not receive put event in time: %v", item)
	}
}

type muxToTransportWrapper struct {
	handler http.Handler
}

func (rt *muxToTransportWrapper) RoundTrip(req *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	rt.handler.ServeHTTP(rec, req)
	return rec.Result(), nil
}
