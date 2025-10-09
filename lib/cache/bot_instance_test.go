// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cache

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
)

// TestBotInstanceCache tests that CRUD operations on bot instances resources are
// replicated from the backend to the cache.
func TestBotInstanceCache(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs[*machineidv1.BotInstance]{
		newResource: func(key string) (*machineidv1.BotInstance, error) {
			return &machineidv1.BotInstance{
				Kind:     types.KindBotInstance,
				Version:  types.V1,
				Metadata: &headerv1.Metadata{},
				Spec: &machineidv1.BotInstanceSpec{
					BotName:    "bot-1",
					InstanceId: key,
				},
				Status: &machineidv1.BotInstanceStatus{},
			}, nil
		},
		cacheGet: func(ctx context.Context, key string) (*machineidv1.BotInstance, error) {
			return p.cache.GetBotInstance(ctx, "bot-1", key)
		},
		cacheList: func(ctx context.Context, pageSize int, pageToken string) ([]*machineidv1.BotInstance, string, error) {
			return p.cache.ListBotInstances(ctx, "", pageSize, pageToken, "", nil)
		},
		create: func(ctx context.Context, resource *machineidv1.BotInstance) error {
			_, err := p.botInstanceService.CreateBotInstance(ctx, resource)
			return err
		},
		list: func(ctx context.Context, pageSize int, pageToken string) ([]*machineidv1.BotInstance, string, error) {
			return p.botInstanceService.ListBotInstances(ctx, "", pageSize, pageToken, "", nil)
		},
		update: func(ctx context.Context, bi *machineidv1.BotInstance) error {
			_, err := p.botInstanceService.PatchBotInstance(ctx, "bot-1", bi.Metadata.GetName(), func(_ *machineidv1.BotInstance) (*machineidv1.BotInstance, error) {
				return bi, nil
			})
			return err
		},
		delete: func(ctx context.Context, key string) error {
			return p.botInstanceService.DeleteBotInstance(ctx, "bot-1", key)
		},
		deleteAll: func(ctx context.Context) error {
			return p.botInstanceService.DeleteAllBotInstances(ctx)
		},
	}, withSkipPaginationTest())
}

// TestBotInstanceCachePaging tests that items from the cache are paginated.
func TestBotInstanceCachePaging(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	for _, n := range []int{5, 1, 3, 4, 2} {
		_, err := p.botInstanceService.CreateBotInstance(ctx, &machineidv1.BotInstance{
			Kind:     types.KindBotInstance,
			Version:  types.V1,
			Metadata: &headerv1.Metadata{},
			Spec: &machineidv1.BotInstanceSpec{
				BotName:    "bot-1",
				InstanceId: "instance-" + strconv.Itoa(n),
			},
			Status: &machineidv1.BotInstanceStatus{},
		})
		require.NoError(t, err)
	}

	// Let the cache catch up
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		results, _, err := p.cache.ListBotInstances(ctx, "", 0, "", "", nil)
		require.NoError(t, err)
		require.Len(t, results, 5)
	}, 10*time.Second, 100*time.Millisecond)

	// page size equal to total items
	results, nextPageToken, err := p.cache.ListBotInstances(ctx, "", 0, "", "", nil)
	require.NoError(t, err)
	require.Empty(t, nextPageToken)
	require.Len(t, results, 5)
	require.Equal(t, "instance-1", results[0].GetMetadata().GetName())
	require.Equal(t, "instance-2", results[1].GetMetadata().GetName())
	require.Equal(t, "instance-3", results[2].GetMetadata().GetName())
	require.Equal(t, "instance-4", results[3].GetMetadata().GetName())
	require.Equal(t, "instance-5", results[4].GetMetadata().GetName())

	// page size smaller than total items
	results, nextPageToken, err = p.cache.ListBotInstances(ctx, "", 3, "", "", nil)
	require.NoError(t, err)
	require.Equal(t, "bot-1/instance-4", nextPageToken)
	require.Len(t, results, 3)
	require.Equal(t, "instance-1", results[0].GetMetadata().GetName())
	require.Equal(t, "instance-2", results[1].GetMetadata().GetName())
	require.Equal(t, "instance-3", results[2].GetMetadata().GetName())

	// next page
	results, nextPageToken, err = p.cache.ListBotInstances(ctx, "", 3, nextPageToken, "", nil)
	require.NoError(t, err)
	require.Empty(t, nextPageToken)
	require.Len(t, results, 2)
	require.Equal(t, "instance-4", results[0].GetMetadata().GetName())
	require.Equal(t, "instance-5", results[1].GetMetadata().GetName())
}

// TestBotInstanceCacheBotFilter tests that cache items are filtered by bot name.
func TestBotInstanceCacheBotFilter(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	for n := range 10 {
		_, err := p.botInstanceService.CreateBotInstance(ctx, &machineidv1.BotInstance{
			Kind:     types.KindBotInstance,
			Version:  types.V1,
			Metadata: &headerv1.Metadata{},
			Spec: &machineidv1.BotInstanceSpec{
				BotName:    "bot-" + strconv.Itoa(n%2),
				InstanceId: "instance-" + strconv.Itoa(n),
			},
			Status: &machineidv1.BotInstanceStatus{},
		})
		require.NoError(t, err)
	}

	// Let the cache catch up
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		results, _, err := p.cache.ListBotInstances(ctx, "", 0, "", "", nil)
		require.NoError(t, err)
		require.Len(t, results, 10)
	}, 10*time.Second, 100*time.Millisecond)

	results, _, err := p.cache.ListBotInstances(ctx, "bot-1", 0, "", "", nil)
	require.NoError(t, err)
	require.Len(t, results, 5)

	for _, b := range results {
		require.Equal(t, "bot-1", b.GetSpec().GetBotName())
	}
}

// TestBotInstanceCacheSearchFilter tests that cache items are filtered by search query.
func TestBotInstanceCacheSearchFilter(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	for n := range 10 {
		instance := &machineidv1.BotInstance{
			Kind:     types.KindBotInstance,
			Version:  types.V1,
			Metadata: &headerv1.Metadata{},
			Spec: &machineidv1.BotInstanceSpec{
				BotName:    "bot-1",
				InstanceId: "instance-" + strconv.Itoa(n+1),
			},
			Status: &machineidv1.BotInstanceStatus{
				LatestHeartbeats: []*machineidv1.BotInstanceStatusHeartbeat{
					{
						Hostname: "host-" + strconv.Itoa(n%2),
					},
				},
			},
		}

		_, err := p.botInstanceService.CreateBotInstance(ctx, instance)
		require.NoError(t, err)
	}

	// Let the cache catch up
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		results, _, err := p.cache.ListBotInstances(ctx, "", 0, "", "", nil)
		require.NoError(t, err)
		require.Len(t, results, 10)
	}, 10*time.Second, 100*time.Millisecond)

	results, _, err := p.cache.ListBotInstances(ctx, "", 0, "", "host-1", nil)
	require.NoError(t, err)
	require.Len(t, results, 5)
}

// TestBotInstanceCacheSorting tests that cache items are sorted.
func TestBotInstanceCacheSorting(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	items := []struct {
		botName           string
		instanceId        string
		recordedAtSeconds int64
	}{
		{"bot-1", "instance-1", 2},
		{"bot-1", "instance-3", 1},
		{"bot-2", "instance-2", 3},
	}

	for _, b := range items {
		instance := &machineidv1.BotInstance{
			Kind:     types.KindBotInstance,
			Version:  types.V1,
			Metadata: &headerv1.Metadata{},
			Spec: &machineidv1.BotInstanceSpec{
				BotName:    b.botName,
				InstanceId: b.instanceId,
			},
			Status: &machineidv1.BotInstanceStatus{
				LatestHeartbeats: []*machineidv1.BotInstanceStatusHeartbeat{
					{
						RecordedAt: &timestamppb.Timestamp{
							Seconds: b.recordedAtSeconds,
						},
					},
				},
			},
		}

		_, err := p.botInstanceService.CreateBotInstance(ctx, instance)
		require.NoError(t, err)
	}

	// Let the cache catch up
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		results, _, err := p.cache.ListBotInstances(ctx, "", 0, "", "", nil)
		require.NoError(t, err)
		require.Len(t, results, 3)
	}, 10*time.Second, 100*time.Millisecond)

	// sort ascending by active_at_latest
	results, _, err := p.cache.ListBotInstances(ctx, "", 0, "", "", &types.SortBy{
		Field:  "active_at_latest",
		IsDesc: false,
	})
	require.NoError(t, err)
	require.Len(t, results, 3)
	require.Equal(t, "instance-3", results[0].GetMetadata().GetName())
	require.Equal(t, "instance-1", results[1].GetMetadata().GetName())
	require.Equal(t, "instance-2", results[2].GetMetadata().GetName())

	// sort descending by active_at_latest
	results, _, err = p.cache.ListBotInstances(ctx, "", 0, "", "", &types.SortBy{
		Field:  "active_at_latest",
		IsDesc: true,
	})
	require.NoError(t, err)
	require.Len(t, results, 3)
	require.Equal(t, "instance-2", results[0].GetMetadata().GetName())
	require.Equal(t, "instance-1", results[1].GetMetadata().GetName())
	require.Equal(t, "instance-3", results[2].GetMetadata().GetName())

	// sort ascending by bot_name
	results, _, err = p.cache.ListBotInstances(ctx, "", 0, "", "", nil) // empty sort should default to `bot_name:asc`
	require.NoError(t, err)
	require.Len(t, results, 3)
	require.Equal(t, "instance-1", results[0].GetMetadata().GetName())
	require.Equal(t, "instance-3", results[1].GetMetadata().GetName())
	require.Equal(t, "instance-2", results[2].GetMetadata().GetName())

	// sort descending by bot_name
	results, _, err = p.cache.ListBotInstances(ctx, "", 0, "", "", &types.SortBy{
		Field:  "bot_name",
		IsDesc: true,
	})
	require.NoError(t, err)
	require.Len(t, results, 3)
	require.Equal(t, "instance-2", results[0].GetMetadata().GetName())
	require.Equal(t, "instance-3", results[1].GetMetadata().GetName())
	require.Equal(t, "instance-1", results[2].GetMetadata().GetName())
}

// TestBotInstanceCacheFallback tests that requests fallback to the upstream when the cache is unhealthy.
func TestBotInstanceCacheFallback(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	p := newTestPack(t, func(cfg Config) Config {
		cfg.neverOK = true // Force the cache into an unhealthy state
		return ForAuth(cfg)
	})
	t.Cleanup(p.Close)

	_, err := p.botInstanceService.CreateBotInstance(ctx, &machineidv1.BotInstance{
		Kind:     types.KindBotInstance,
		Version:  types.V1,
		Metadata: &headerv1.Metadata{},
		Spec: &machineidv1.BotInstanceSpec{
			BotName:    "bot-1",
			InstanceId: "instance-1",
		},
		Status: &machineidv1.BotInstanceStatus{},
	})
	require.NoError(t, err)

	// Let the cache catch up
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		results, _, err := p.cache.ListBotInstances(ctx, "", 0, "", "", nil)
		require.NoError(t, err)
		require.Len(t, results, 1)
	}, 10*time.Second, 100*time.Millisecond)

	// sort ascending by bot_name
	results, _, err := p.cache.ListBotInstances(ctx, "", 0, "", "", &types.SortBy{
		Field:  "bot_name",
		IsDesc: false,
	})
	require.NoError(t, err) // asc by bot_name is the only sort supported by the upstream
	require.Len(t, results, 1)

	// sort descending by bot_name
	_, _, err = p.cache.ListBotInstances(ctx, "", 0, "", "", &types.SortBy{
		Field:  "bot_name",
		IsDesc: true,
	})
	require.Error(t, err)
	require.Equal(t, "unsupported sort, only bot_name:asc is supported, but got \"bot_name\" (desc = true)", err.Error())

	// sort ascending by active_at_latest
	_, _, err = p.cache.ListBotInstances(ctx, "", 0, "", "", &types.SortBy{
		Field:  "active_at_latest",
		IsDesc: false,
	})
	require.Error(t, err)
	require.Equal(t, "unsupported sort, only bot_name:asc is supported, but got \"active_at_latest\" (desc = false)", err.Error())

}
