/*
Copyright 2023 Gravitational, Inc.

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

package discovery

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestGetUpsertBatchSize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		queueLen          int
		lastBatchSize     int
		expectedBatchSize int
	}{
		{
			name:              "small batches",
			queueLen:          100,
			lastBatchSize:     0,
			expectedBatchSize: minBatchSize,
		},
		{
			name:              "continue previous batch size",
			queueLen:          100,
			lastBatchSize:     20,
			expectedBatchSize: 20,
		},
		{
			name:              "large batches",
			queueLen:          10000,
			lastBatchSize:     0,
			expectedBatchSize: 12,
		},
		{
			name:              "larger batch than previous",
			queueLen:          10000,
			lastBatchSize:     10,
			expectedBatchSize: 12,
		},
		{
			name:              "last batch larger than queue size",
			queueLen:          10,
			lastBatchSize:     15,
			expectedBatchSize: 10,
		},
		{
			name:              "short queue",
			queueLen:          3,
			lastBatchSize:     0,
			expectedBatchSize: 3,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expectedBatchSize, getUpsertBatchSize(tc.queueLen, tc.lastBatchSize))
		})
	}
}

func generateServerInfos(t *testing.T, n int) []types.ServerInfo {
	serverInfos := make([]types.ServerInfo, 0, n)
	for i := 0; i < n; i++ {
		si, err := types.NewServerInfo(types.Metadata{
			Name:   fmt.Sprintf("instance-%d", i),
			Labels: map[string]string{"foo": "bar"},
		}, types.ServerInfoSpecV1{})
		require.NoError(t, err)
		serverInfos = append(serverInfos, si)
	}
	return serverInfos
}

func initLabelReconcilerForTests(t *testing.T, clock clockwork.Clock) (*labelReconciler, *fakeAccessPoint) {
	ap := newFakeAccessPoint()
	lr, err := newLabelReconciler(&labelReconcilerConfig{
		clock:       clock,
		accessPoint: ap,
	})
	require.NoError(t, err)
	return lr, ap
}

func TestLabelReconciler(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClock()
	lr, ap := initLabelReconcilerForTests(t, clock)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	go lr.run(ctx)

	serverInfos := generateServerInfos(t, 25)
	lr.queueServerInfos(serverInfos)
	b := minBatchSize

	for i := 0; i < 5; i++ {
		clock.BlockUntil(1)
		clock.Advance(time.Second)
		var upsertedServerInfos []types.ServerInfo
	outer:
		for {
			select {
			case si := <-ap.upsertedServerInfos:
				upsertedServerInfos = append(upsertedServerInfos, si)
			case <-time.After(10 * time.Millisecond):
				break outer
			case <-ctx.Done():
				require.Fail(t, "timed out waiting for server infos")
			}
		}
		require.Len(t, upsertedServerInfos, b)
		require.Equal(t, serverInfos[b*i:b*(i+1)], upsertedServerInfos)
	}
}

func TestQueueServerInfos(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClock()
	nearFuture := clock.Now().Add(10 * time.Minute)
	farFuture := clock.Now().Add(time.Hour)

	newServerInfo := func(mod func(si types.ServerInfo)) types.ServerInfo {
		defaultServerInfo, err := types.NewServerInfo(types.Metadata{
			Name:    "default",
			Labels:  map[string]string{"foo": "bar"},
			Expires: &farFuture,
		}, types.ServerInfoSpecV1{})
		require.NoError(t, err)

		if mod != nil {
			mod(defaultServerInfo)
		}
		return defaultServerInfo
	}

	defaultServerInfos := []types.ServerInfo{newServerInfo(nil)}

	tests := []struct {
		name          string
		existingInfos []types.ServerInfo
		newInfos      []types.ServerInfo
		expectedInfos []types.ServerInfo
	}{
		{
			name:          "new info",
			newInfos:      defaultServerInfos,
			expectedInfos: defaultServerInfos,
		},
		{
			name:          "ignore existing info",
			existingInfos: defaultServerInfos,
			newInfos:      defaultServerInfos,
			expectedInfos: []types.ServerInfo{},
		},
		{
			name: "re-queue updated labels",
			existingInfos: []types.ServerInfo{newServerInfo(func(si types.ServerInfo) {
				si.SetNewLabels(map[string]string{"foo": "baz"})
			})},
			newInfos:      defaultServerInfos,
			expectedInfos: defaultServerInfos,
		},
		{
			name: "re-queue expiring soon",
			existingInfos: []types.ServerInfo{newServerInfo(func(si types.ServerInfo) {
				si.SetExpiry(nearFuture)
			})},
			newInfos:      defaultServerInfos,
			expectedInfos: defaultServerInfos,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lr, _ := initLabelReconcilerForTests(t, clock)
			for _, si := range tc.existingInfos {
				lr.discoveredServers[si.GetName()] = si
			}
			lr.queueServerInfos(tc.newInfos)
			require.Empty(t, cmp.Diff(tc.expectedInfos, lr.serverInfoQueue,
				cmpopts.IgnoreFields(types.Metadata{}, "Expires")))
		})
	}
}
