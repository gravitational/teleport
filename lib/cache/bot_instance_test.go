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

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
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
			return p.cache.ListBotInstances(ctx, pageSize, pageToken, nil)
		},
		create: func(ctx context.Context, resource *machineidv1.BotInstance) error {
			_, err := p.botInstanceService.CreateBotInstance(ctx, resource)
			return err
		},
		list: func(ctx context.Context, pageSize int, pageToken string) ([]*machineidv1.BotInstance, string, error) {
			return p.botInstanceService.ListBotInstances(ctx, pageSize, pageToken, nil)
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
	})
}

// TestBotInstanceCachePaging tests that items from the cache are paginated.
func TestBotInstanceCachePaging(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

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
		results, _, err := p.cache.ListBotInstances(ctx, 0, "", nil)
		require.NoError(t, err)
		require.Len(t, results, 5)
	}, 10*time.Second, 100*time.Millisecond)

	// page size equal to total items
	results, nextPageToken, err := p.cache.ListBotInstances(ctx, 0, "", nil)
	require.NoError(t, err)
	require.Empty(t, nextPageToken)
	require.Len(t, results, 5)
	require.Equal(t, "instance-1", results[0].GetMetadata().GetName())
	require.Equal(t, "instance-2", results[1].GetMetadata().GetName())
	require.Equal(t, "instance-3", results[2].GetMetadata().GetName())
	require.Equal(t, "instance-4", results[3].GetMetadata().GetName())
	require.Equal(t, "instance-5", results[4].GetMetadata().GetName())

	// page size smaller than total items
	results, nextPageToken, err = p.cache.ListBotInstances(ctx, 3, "", nil)
	require.NoError(t, err)
	require.Equal(t, "bot-1/instance-4", nextPageToken)
	require.Len(t, results, 3)
	require.Equal(t, "instance-1", results[0].GetMetadata().GetName())
	require.Equal(t, "instance-2", results[1].GetMetadata().GetName())
	require.Equal(t, "instance-3", results[2].GetMetadata().GetName())

	// next page
	results, nextPageToken, err = p.cache.ListBotInstances(ctx, 3, nextPageToken, nil)
	require.NoError(t, err)
	require.Empty(t, nextPageToken)
	require.Len(t, results, 2)
	require.Equal(t, "instance-4", results[0].GetMetadata().GetName())
	require.Equal(t, "instance-5", results[1].GetMetadata().GetName())
}

// TestBotInstanceCacheBotFilter tests that cache items are filtered by bot name.
func TestBotInstanceCacheBotFilter(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

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
		results, _, err := p.cache.ListBotInstances(ctx, 0, "", nil)
		require.NoError(t, err)
		require.Len(t, results, 10)
	}, 10*time.Second, 100*time.Millisecond)

	results, _, err := p.cache.ListBotInstances(ctx, 0, "", &services.ListBotInstancesRequestOptions{
		FilterBotName: "bot-1",
	})
	require.NoError(t, err)
	require.Len(t, results, 5)

	for _, b := range results {
		require.Equal(t, "bot-1", b.GetSpec().GetBotName())
	}
}

// TestBotInstanceCacheSearchFilter tests that cache items are filtered by
// search term.
func TestBotInstanceCacheSearchFilter(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

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
		results, _, err := p.cache.ListBotInstances(ctx, 0, "", nil)
		require.NoError(t, err)
		require.Len(t, results, 10)
	}, 10*time.Second, 100*time.Millisecond)

	results, _, err := p.cache.ListBotInstances(ctx, 0, "", &services.ListBotInstancesRequestOptions{
		FilterSearchTerm: "host-1",
	})
	require.NoError(t, err)
	require.Len(t, results, 5)
}

// TestBotInstanceCacheQueryFilter tests that cache items are filtered by query.
func TestBotInstanceCacheQueryFilter(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	{
		_, err := p.botInstanceService.CreateBotInstance(ctx, &machineidv1.BotInstance{
			Kind:     types.KindBotInstance,
			Version:  types.V1,
			Metadata: &headerv1.Metadata{},
			Spec: &machineidv1.BotInstanceSpec{
				BotName:    "bot-1",
				InstanceId: "00000000-0000-0000-0000-000000000000",
			},
			Status: &machineidv1.BotInstanceStatus{
				LatestHeartbeats: []*machineidv1.BotInstanceStatusHeartbeat{
					{
						Hostname: "host-1",
					},
				},
			},
		})
		require.NoError(t, err)
	}

	{
		_, err := p.botInstanceService.CreateBotInstance(ctx, &machineidv1.BotInstance{
			Kind:     types.KindBotInstance,
			Version:  types.V1,
			Metadata: &headerv1.Metadata{},
			Spec: &machineidv1.BotInstanceSpec{
				BotName:    "bot-1",
				InstanceId: uuid.New().String(),
			},
			Status: &machineidv1.BotInstanceStatus{
				LatestHeartbeats: []*machineidv1.BotInstanceStatusHeartbeat{
					{
						Hostname: "host-2",
					},
				},
			},
		})
		require.NoError(t, err)
	}

	// Let the cache catch up
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		results, _, err := p.cache.ListBotInstances(ctx, 0, "", nil)
		require.NoError(t, err)
		require.Len(t, results, 2)
	}, 10*time.Second, 100*time.Millisecond)

	results, _, err := p.cache.ListBotInstances(ctx, 0, "", &services.ListBotInstancesRequestOptions{
		FilterQuery: `status.latest_heartbeat.hostname == "host-1"`,
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "00000000-0000-0000-0000-000000000000", results[0].Spec.InstanceId)
}

// TestBotInstanceCacheSorting tests that cache items are sorted.
func TestBotInstanceCacheSorting(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	items := []struct {
		botName           string
		instanceId        string
		recordedAtSeconds int64
		version           string
		hostname          string
	}{
		{"bot-1", "instance-1", 2, "2.0.0", "hostname-2"},
		{"bot-1", "instance-3", 1, "2.0.0-rc1", "hostname-3"},
		{"bot-2", "instance-2", 3, "1.0.0", "hostname-1"},
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
						Version:  b.version,
						Hostname: b.hostname,
					},
				},
			},
		}

		_, err := p.botInstanceService.CreateBotInstance(ctx, instance)
		require.NoError(t, err)
	}

	// Let the cache catch up
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		results, _, err := p.cache.ListBotInstances(ctx, 0, "", nil)
		require.NoError(t, err)
		require.Len(t, results, 3)
	}, 10*time.Second, 100*time.Millisecond)

	t.Run("sort ascending by active_at_latest", func(t *testing.T) {
		results, _, err := p.cache.ListBotInstances(ctx, 0, "", &services.ListBotInstancesRequestOptions{
			SortField: "active_at_latest",
			SortDesc:  false,
		})
		require.NoError(t, err)
		require.Len(t, results, 3)
		assert.Equal(t, "instance-3", results[0].GetMetadata().GetName())
		assert.Equal(t, "instance-1", results[1].GetMetadata().GetName())
		assert.Equal(t, "instance-2", results[2].GetMetadata().GetName())
	})

	t.Run("sort descending by active_at_latest", func(t *testing.T) {
		results, _, err := p.cache.ListBotInstances(ctx, 0, "", &services.ListBotInstancesRequestOptions{
			SortField: "active_at_latest",
			SortDesc:  true,
		})
		require.NoError(t, err)
		require.Len(t, results, 3)
		assert.Equal(t, "instance-2", results[0].GetMetadata().GetName())
		assert.Equal(t, "instance-1", results[1].GetMetadata().GetName())
		assert.Equal(t, "instance-3", results[2].GetMetadata().GetName())
	})

	t.Run("sort ascending by bot_name", func(t *testing.T) {
		results, _, err := p.cache.ListBotInstances(ctx, 0, "", nil) // empty sort should default to `bot_name:asc`
		require.NoError(t, err)
		require.Len(t, results, 3)
		assert.Equal(t, "instance-1", results[0].GetMetadata().GetName())
		assert.Equal(t, "instance-3", results[1].GetMetadata().GetName())
		assert.Equal(t, "instance-2", results[2].GetMetadata().GetName())
	})

	t.Run("sort descending by bot_name", func(t *testing.T) {
		results, _, err := p.cache.ListBotInstances(ctx, 0, "", &services.ListBotInstancesRequestOptions{
			SortField: "bot_name",
			SortDesc:  true,
		})
		require.NoError(t, err)
		require.Len(t, results, 3)
		assert.Equal(t, "instance-2", results[0].GetMetadata().GetName())
		assert.Equal(t, "instance-3", results[1].GetMetadata().GetName())
		assert.Equal(t, "instance-1", results[2].GetMetadata().GetName())
	})

	t.Run("sort ascending by version", func(t *testing.T) {
		results, _, err := p.cache.ListBotInstances(ctx, 0, "", &services.ListBotInstancesRequestOptions{
			SortField: "version_latest",
			SortDesc:  false,
		})
		require.NoError(t, err)
		require.Len(t, results, 3)
		assert.Equal(t, "instance-2", results[0].GetMetadata().GetName())
		assert.Equal(t, "instance-3", results[1].GetMetadata().GetName())
		assert.Equal(t, "instance-1", results[2].GetMetadata().GetName())
	})

	t.Run("sort descending by version", func(t *testing.T) {
		results, _, err := p.cache.ListBotInstances(ctx, 0, "", &services.ListBotInstancesRequestOptions{
			SortField: "version_latest",
			SortDesc:  true,
		})
		require.NoError(t, err)
		require.Len(t, results, 3)
		assert.Equal(t, "instance-1", results[0].GetMetadata().GetName())
		assert.Equal(t, "instance-3", results[1].GetMetadata().GetName())
		assert.Equal(t, "instance-2", results[2].GetMetadata().GetName())
	})

	t.Run("sort ascending by hostname", func(t *testing.T) {
		results, _, err := p.cache.ListBotInstances(ctx, 0, "", &services.ListBotInstancesRequestOptions{
			SortField: "host_name_latest",
			SortDesc:  false,
		})
		require.NoError(t, err)
		require.Len(t, results, 3)
		require.Equal(t, "instance-2", results[0].GetMetadata().GetName())
		require.Equal(t, "instance-1", results[1].GetMetadata().GetName())
		require.Equal(t, "instance-3", results[2].GetMetadata().GetName())
	})

	t.Run("sort descending by hostname", func(t *testing.T) {
		results, _, err := p.cache.ListBotInstances(ctx, 0, "", &services.ListBotInstancesRequestOptions{
			SortField: "host_name_latest",
			SortDesc:  true,
		})
		require.NoError(t, err)
		require.Len(t, results, 3)
		require.Equal(t, "instance-3", results[0].GetMetadata().GetName())
		require.Equal(t, "instance-1", results[1].GetMetadata().GetName())
		require.Equal(t, "instance-2", results[2].GetMetadata().GetName())
	})

	t.Run("sort invalid field", func(t *testing.T) {
		_, _, err := p.cache.ListBotInstances(ctx, 0, "", &services.ListBotInstancesRequestOptions{
			SortField: "blah",
		})
		require.Error(t, err)
		assert.ErrorContains(t, err, `unsupported sort "blah" but expected bot_name, active_at_latest, version_latest or host_name_latest`)
	})
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
		results, _, err := p.cache.ListBotInstances(ctx, 0, "", nil)
		require.NoError(t, err)
		require.Len(t, results, 1)
	}, 10*time.Second, 100*time.Millisecond)

	// sort ascending by bot_name
	results, _, err := p.cache.ListBotInstances(ctx, 0, "", &services.ListBotInstancesRequestOptions{
		SortField: "bot_name",
		SortDesc:  false,
	})
	require.NoError(t, err) // asc by bot_name is the only sort supported by the upstream
	require.Len(t, results, 1)

	// sort descending by bot_name
	_, _, err = p.cache.ListBotInstances(ctx, 0, "", &services.ListBotInstancesRequestOptions{
		SortField: "bot_name",
		SortDesc:  true,
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "unsupported sort, only ascending order is supported")

	// sort ascending by active_at_latest
	_, _, err = p.cache.ListBotInstances(ctx, 0, "", &services.ListBotInstancesRequestOptions{
		SortField: "active_at_latest",
		SortDesc:  false,
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "unsupported sort, only bot_name field is supported, but got \"active_at_latest\"")
}

func TestKeyForVersionIndex(t *testing.T) {
	t.Parallel()

	tcs := []struct {
		name      string
		mutatorFn func(*machineidv1.BotInstance)
		key       string
	}{
		{
			name:      "zero heartbeats",
			mutatorFn: func(b *machineidv1.BotInstance) {},
			key:       "000000.000000.000000-~/bot-instance-1",
		},
		{
			name: "invalid version",
			mutatorFn: func(b *machineidv1.BotInstance) {
				b.Status = &machineidv1.BotInstanceStatus{
					InitialHeartbeat: &machineidv1.BotInstanceStatusHeartbeat{
						Version: "a.b.c",
					},
				}
			},
			key: "000000.000000.000000-~/bot-instance-1",
		},
		{
			name: "initial heartbeat",
			mutatorFn: func(b *machineidv1.BotInstance) {
				b.Status = &machineidv1.BotInstanceStatus{
					InitialHeartbeat: &machineidv1.BotInstanceStatusHeartbeat{
						Version: "1.0.0",
					},
				}
			},
			key: "000001.000000.000000-~/bot-instance-1",
		},
		{
			name: "latest heartbeat",
			mutatorFn: func(b *machineidv1.BotInstance) {
				b.Status = &machineidv1.BotInstanceStatus{
					LatestHeartbeats: []*machineidv1.BotInstanceStatusHeartbeat{
						{
							Version: "1.0.0",
						},
					},
				}
			},
			key: "000001.000000.000000-~/bot-instance-1",
		},
		{
			name: "with release",
			mutatorFn: func(b *machineidv1.BotInstance) {
				b.Status = &machineidv1.BotInstanceStatus{
					InitialHeartbeat: &machineidv1.BotInstanceStatusHeartbeat{
						Version: "1.0.0-dev",
					},
				}
			},
			key: "000001.000000.000000-dev/bot-instance-1",
		},
		{
			name: "with build",
			mutatorFn: func(b *machineidv1.BotInstance) {
				b.Status = &machineidv1.BotInstanceStatus{
					InitialHeartbeat: &machineidv1.BotInstanceStatusHeartbeat{
						Version: "1.0.0+build1",
					},
				}
			},
			key: "000001.000000.000000-~/bot-instance-1",
		},
		{
			name: "with release and build",
			mutatorFn: func(b *machineidv1.BotInstance) {
				b.Status = &machineidv1.BotInstanceStatus{
					InitialHeartbeat: &machineidv1.BotInstanceStatusHeartbeat{
						Version: "1.0.0-dev+build1",
					},
				}
			},
			key: "000001.000000.000000-dev/bot-instance-1",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			instance := &machineidv1.BotInstance{
				Kind:    types.KindBotInstance,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "bot-instance-1",
				},
				Spec:   &machineidv1.BotInstanceSpec{},
				Status: &machineidv1.BotInstanceStatus{},
			}
			tc.mutatorFn(instance)

			versionKey := keyForBotInstanceVersionIndex(instance)

			assert.Equal(t, tc.key, versionKey)
		})
	}
}
