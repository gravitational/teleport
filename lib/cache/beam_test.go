// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/set"
)

func TestBeamCache(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	nextBeamAlias := beamAliasGenerator(t,
		"amber-forest",
		"silver-canyon",
		"quiet-meadow",
		"steady-river",
		"brisk-harbor",
		"gold-valley",
		"clear-prairie",
		"north-garden",
	)
	testResources153(t, p, testFuncs[*beamsv1.Beam]{
		newResource: func(name string) (*beamsv1.Beam, error) {
			return newBeamResource(name, nextBeamAlias(), time.Now().Add(time.Hour)), nil
		},
		create: func(ctx context.Context, beam *beamsv1.Beam) error {
			return createBeamForCacheTest(ctx, p, beam)
		},
		list: func(ctx context.Context, pageSize int, pageToken string) ([]*beamsv1.Beam, string, error) {
			return p.beams.ListBeamsV2(ctx, pageSize, pageToken, nil)
		},
		delete: func(ctx context.Context, name string) error {
			return deleteBeamForCacheTest(ctx, p, name)
		},
		deleteAll: func(ctx context.Context) error {
			beams, _, err := p.beams.ListBeamsV2(ctx, 0, "", nil)
			require.NoError(t, err)
			for _, beam := range beams {
				require.NoError(t, deleteBeamForCacheTest(ctx, p, beam.GetMetadata().GetName()))
			}
			return nil
		},
		cacheList: func(ctx context.Context, pageSize int, pageToken string) ([]*beamsv1.Beam, string, error) {
			return p.cache.ListBeamsV2(ctx, pageSize, pageToken, nil)
		},
		cacheGet: p.cache.GetBeam,
	})
}

func TestBeamCache_GetBeamByAlias(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	beam := newBeamResource(uuid.NewString(), "warm-orbit", time.Now().Add(1*time.Hour))
	require.NoError(t, createBeamForCacheTest(ctx, p, beam))

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		got, err := p.cache.GetBeamByAlias(ctx, "warm-orbit")
		require.NoError(t, err)
		require.Equal(t, beam.GetMetadata().GetName(), got.GetMetadata().GetName())
	}, 2*time.Second, 50*time.Millisecond)

	require.Never(t, func() bool {
		_, err := p.cache.GetBeamByAlias(t.Context(), "tepid-spin")
		return !trace.IsNotFound(err)
	}, 2*time.Second, 100*time.Millisecond)
}

func TestBeamCache_ListPaging(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	for _, beam := range []*beamsv1.Beam{
		newBeamResource("beam-5", "gold-valley", time.Now().Add(5*time.Hour)),
		newBeamResource("beam-1", "amber-forest", time.Now().Add(1*time.Hour)),
		newBeamResource("beam-3", "quiet-meadow", time.Now().Add(3*time.Hour)),
		newBeamResource("beam-4", "steady-river", time.Now().Add(4*time.Hour)),
		newBeamResource("beam-2", "brisk-harbor", time.Now().Add(2*time.Hour)),
	} {
		require.NoError(t, createBeamForCacheTest(ctx, p, beam))
	}

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		results, _, err := p.cache.ListBeamsV2(ctx, 0, "", nil)
		require.NoError(t, err)
		require.Len(t, results, 5)
	}, 10*time.Second, 100*time.Millisecond)

	results, nextPageToken, err := p.cache.ListBeamsV2(ctx, 0, "", nil)
	require.NoError(t, err)
	require.Empty(t, nextPageToken)
	require.Len(t, results, 5)
	require.Equal(t, "beam-1", results[0].GetMetadata().GetName())
	require.Equal(t, "beam-2", results[1].GetMetadata().GetName())
	require.Equal(t, "beam-3", results[2].GetMetadata().GetName())
	require.Equal(t, "beam-4", results[3].GetMetadata().GetName())
	require.Equal(t, "beam-5", results[4].GetMetadata().GetName())

	results, nextPageToken, err = p.cache.ListBeamsV2(ctx, 3, "", nil)
	require.NoError(t, err)
	require.Equal(t, "beam-4", nextPageToken)
	require.Len(t, results, 3)
	require.Equal(t, "beam-1", results[0].GetMetadata().GetName())
	require.Equal(t, "beam-2", results[1].GetMetadata().GetName())
	require.Equal(t, "beam-3", results[2].GetMetadata().GetName())

	results, nextPageToken, err = p.cache.ListBeamsV2(ctx, 3, nextPageToken, nil)
	require.NoError(t, err)
	require.Empty(t, nextPageToken)
	require.Len(t, results, 2)
	require.Equal(t, "beam-4", results[0].GetMetadata().GetName())
	require.Equal(t, "beam-5", results[1].GetMetadata().GetName())
}

func TestBeamCache_ListSorting(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	beams := []*beamsv1.Beam{
		newBeamResourceWithUser("beam-3", "copper-meadow", "bob", time.Unix(300, 0)),
		newBeamResourceWithUser("beam-1", "amber-forest", "alice", time.Unix(100, 0)),
		newBeamResourceWithUser("beam-2", "brisk-harbor", "carol", time.Unix(200, 0)),
	}

	for _, beam := range beams {
		require.NoError(t, createBeamForCacheTest(ctx, p, beam))
	}

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		results, _, err := p.cache.ListBeamsV2(ctx, 0, "", nil)
		require.NoError(t, err)
		require.Len(t, results, 3)
	}, 10*time.Second, 100*time.Millisecond)

	t.Run("sort ascending by name", func(t *testing.T) {
		results, _, err := p.cache.ListBeamsV2(ctx, 0, "", nil)
		require.NoError(t, err)
		require.Len(t, results, 3)
		assert.Equal(t, "beam-1", results[0].GetMetadata().GetName())
		assert.Equal(t, "beam-2", results[1].GetMetadata().GetName())
		assert.Equal(t, "beam-3", results[2].GetMetadata().GetName())
	})

	t.Run("sort descending by name", func(t *testing.T) {
		results, _, err := p.cache.ListBeamsV2(ctx, 0, "", &services.ListBeamsRequestOptions{
			SortField: beamsv1.BeamSortField_BEAM_SORT_FIELD_NAME,
			SortOrder: beamsv1.BeamSortOrder_BEAM_SORT_ORDER_DESCENDING,
		})
		require.NoError(t, err)
		require.Len(t, results, 3)
		assert.Equal(t, "beam-3", results[0].GetMetadata().GetName())
		assert.Equal(t, "beam-2", results[1].GetMetadata().GetName())
		assert.Equal(t, "beam-1", results[2].GetMetadata().GetName())
	})

	t.Run("sort ascending by alias", func(t *testing.T) {
		results, _, err := p.cache.ListBeamsV2(ctx, 0, "", &services.ListBeamsRequestOptions{
			SortField: beamsv1.BeamSortField_BEAM_SORT_FIELD_ALIAS,
		})
		require.NoError(t, err)
		require.Len(t, results, 3)
		assert.Equal(t, "amber-forest", results[0].GetStatus().GetAlias())
		assert.Equal(t, "brisk-harbor", results[1].GetStatus().GetAlias())
		assert.Equal(t, "copper-meadow", results[2].GetStatus().GetAlias())
	})

	t.Run("sort ascending by user", func(t *testing.T) {
		results, _, err := p.cache.ListBeamsV2(ctx, 0, "", &services.ListBeamsRequestOptions{
			SortField: beamsv1.BeamSortField_BEAM_SORT_FIELD_USER,
		})
		require.NoError(t, err)
		require.Len(t, results, 3)
		assert.Equal(t, "alice", results[0].GetStatus().GetUser())
		assert.Equal(t, "bob", results[1].GetStatus().GetUser())
		assert.Equal(t, "carol", results[2].GetStatus().GetUser())
	})

	t.Run("sort ascending by expires", func(t *testing.T) {
		results, _, err := p.cache.ListBeamsV2(ctx, 0, "", &services.ListBeamsRequestOptions{
			SortField: beamsv1.BeamSortField_BEAM_SORT_FIELD_EXPIRES,
		})
		require.NoError(t, err)
		require.Len(t, results, 3)
		assert.Equal(t, "beam-1", results[0].GetMetadata().GetName())
		assert.Equal(t, "beam-2", results[1].GetMetadata().GetName())
		assert.Equal(t, "beam-3", results[2].GetMetadata().GetName())
	})
}

func TestBeamCacheList_FilterByUser(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	require.NoError(t, createBeamForCacheTest(ctx, p, newBeamResourceWithUser("beam-1", "amber-forest", "alice", time.Now().Add(time.Hour))))
	require.NoError(t, createBeamForCacheTest(ctx, p, newBeamResourceWithUser("beam-2", "brisk-harbor", "bob", time.Now().Add(2*time.Hour))))
	require.NoError(t, createBeamForCacheTest(ctx, p, newBeamResourceWithUser("beam-3", "copper-meadow", "alice", time.Now().Add(3*time.Hour))))

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		results, _, err := p.cache.ListBeamsV2(ctx, 0, "", nil)
		require.NoError(t, err)
		require.Len(t, results, 3)
	}, 10*time.Second, 100*time.Millisecond)

	results, _, err := p.cache.ListBeamsV2(ctx, 0, "", &services.ListBeamsRequestOptions{
		FilterUsers: set.New("alice"),
	})
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "beam-1", results[0].GetMetadata().GetName())
	assert.Equal(t, "beam-3", results[1].GetMetadata().GetName())

	results, _, err = p.cache.ListBeamsV2(ctx, 0, "", &services.ListBeamsRequestOptions{
		FilterUsers: set.New("bob"),
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "beam-2", results[0].GetMetadata().GetName())
}

func TestBeamCache_ListFallback(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	p := newTestPack(t, func(cfg Config) Config {
		cfg.neverOK = true
		return ForAuth(cfg)
	})
	t.Cleanup(p.Close)

	require.NoError(t, createBeamForCacheTest(ctx, p, newBeamResourceWithUser("beam-1", "amber-forest", "alice", time.Now().Add(time.Hour))))
	require.NoError(t, createBeamForCacheTest(ctx, p, newBeamResourceWithUser("beam-2", "brisk-harbor", "bob", time.Now().Add(2*time.Hour))))

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		results, _, err := p.cache.ListBeamsV2(ctx, 0, "", nil)
		require.NoError(t, err)
		require.Len(t, results, 2)
	}, 10*time.Second, 100*time.Millisecond)

	t.Run("supported sort", func(t *testing.T) {
		results, _, err := p.cache.ListBeamsV2(ctx, 0, "", &services.ListBeamsRequestOptions{
			SortField: beamsv1.BeamSortField_BEAM_SORT_FIELD_NAME,
			SortOrder: beamsv1.BeamSortOrder_BEAM_SORT_ORDER_ASCENDING,
		})
		require.NoError(t, err)
		require.Len(t, results, 2)
	})

	t.Run("filter by user", func(t *testing.T) {
		results, _, err := p.cache.ListBeamsV2(ctx, 0, "", &services.ListBeamsRequestOptions{
			FilterUsers: set.New("bob"),
		})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, "beam-2", results[0].GetMetadata().GetName())
	})

	t.Run("unsupported sort field", func(t *testing.T) {
		_, _, err := p.cache.ListBeamsV2(ctx, 0, "", &services.ListBeamsRequestOptions{
			SortField: beamsv1.BeamSortField_BEAM_SORT_FIELD_EXPIRES,
		})
		require.ErrorContains(t, err, `unsupported sort, only name field is supported`)
	})

	t.Run("unsupported sort dir", func(t *testing.T) {
		_, _, err := p.cache.ListBeamsV2(ctx, 0, "", &services.ListBeamsRequestOptions{
			SortOrder: beamsv1.BeamSortOrder_BEAM_SORT_ORDER_DESCENDING,
		})
		require.ErrorContains(t, err, "unsupported sort, only ascending order is supported")
	})
}

func newBeamResource(name, alias string, expires time.Time) *beamsv1.Beam {
	ts := timestamppb.New(expires)
	return &beamsv1.Beam{
		Kind:    types.KindBeam,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:    name,
			Expires: ts,
		},
		Spec: &beamsv1.BeamSpec{
			Egress:         beamsv1.EgressMode_EGRESS_MODE_RESTRICTED,
			AllowedDomains: []string{"example.com."},
			Publish: &beamsv1.PublishSpec{
				Port:     8080,
				Protocol: beamsv1.Protocol_PROTOCOL_HTTP,
			},
			Expires: ts,
		},
		Status: &beamsv1.BeamStatus{
			User:                 "alice",
			Alias:                alias,
			BotName:              uuid.NewString(),
			JoinTokenName:        uuid.NewString(),
			DelegationSessionId:  uuid.NewString(),
			WorkloadIdentityName: uuid.NewString(),
			ComputeStatus:        beamsv1.ComputeStatus_COMPUTE_STATUS_PROVISION_PENDING,
		},
	}
}

func newBeamResourceWithUser(name, alias, user string, expires time.Time) *beamsv1.Beam {
	beam := newBeamResource(name, alias, expires)
	beam.Status.User = user
	return beam
}

func beamAliasGenerator(t *testing.T, aliases ...string) func() string {
	var i int
	return func() string {
		if i >= len(aliases) {
			t.Fatal("beam alias list exhausted")
		}
		alias := aliases[i]
		i++
		return alias
	}
}

func createBeamForCacheTest(ctx context.Context, p *testPack, beam *beamsv1.Beam) error {
	actions, err := p.beams.AppendPutBeamActions(nil, beam, backend.NotExists())
	if err != nil {
		return err
	}

	_, err = p.backend.AtomicWrite(ctx, actions)
	return err
}

func deleteBeamForCacheTest(ctx context.Context, p *testPack, name string) error {
	beam, err := p.beams.GetBeam(ctx, name)
	if err != nil {
		return err
	}

	actions, err := p.beams.AppendDeleteBeamActions(nil, beam, backend.Exists())
	if err != nil {
		return err
	}

	_, err = p.backend.AtomicWrite(ctx, actions)
	return err
}
