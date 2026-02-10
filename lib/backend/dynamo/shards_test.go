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

package dynamo

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils/clocki"
)

// TestEventStream tests that DynamoDB Streams correctly handles shard splits
// without losing events. This test writes many large items to trigger DynamoDB to
// split shards, then verifies all events are received by the watcher.
func TestEventStream(t *testing.T) {
	ensureTestsEnabled(t)
	tabelName := uuid.New().String() + "events"

	dynamoCfg := map[string]any{
		"table_name":         tabelName,
		"poll_stream_period": 50 * time.Millisecond,
		"retry_period":       10 * time.Millisecond,
		"read_min_capacity":  10,
		"read_max_capacity":  10,
		"read_target_value":  30.0,
		"write_min_capacity": 10,
		"write_max_capacity": 20,
		"write_target_value": 50.0,
	}
	b, err := newBackend(dynamoCfg)
	require.NoError(t, err)
	defer b.Close()

	t.Cleanup(func() { require.NoError(t, deleteTable(context.Background(), b.svc, tabelName)) })

	time.Sleep(time.Second * 10)
	cfg := test.WatchEventsHeightVolumeConfig{
		NumEvents:    100,
		NumWriters:   10,
		ItemSize:     300 * 1024, // 300KB items (near DynamoDB limit) trigger splits faster.
		EventTimeout: time.Second * 30,
	}

	test.RunWatchEventsHeightVolume(t, func(options ...test.ConstructionOption) (backend.Backend, clocki.FakeClock, error) {
		return b, test.BlockingFakeClock{Clock: b.clock}, nil
	}, cfg)
}

func newBackend(config map[string]any, options ...test.ConstructionOption) (*Backend, error) {
	testCfg, err := test.ApplyOptions(options)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if testCfg.MirrorMode {
		return nil, test.ErrMirrorNotSupported
	}
	if testCfg.ConcurrentBackend != nil {
		return nil, test.ErrConcurrentAccessNotSupported
	}
	b, err := New(context.Background(), config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return b, nil
}
