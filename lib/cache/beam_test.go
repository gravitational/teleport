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
)

func TestBeam(t *testing.T) {
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
		list: p.beams.ListBeams,
		delete: func(ctx context.Context, name string) error {
			return deleteBeamForCacheTest(ctx, p, name)
		},
		deleteAll: func(ctx context.Context) error {
			beams, _, err := p.beams.ListBeams(ctx, 0, "")
			require.NoError(t, err)
			for _, beam := range beams {
				require.NoError(t, deleteBeamForCacheTest(ctx, p, beam.GetMetadata().GetName()))
			}
			return nil
		},
		cacheList: p.cache.ListBeams,
		cacheGet:  p.cache.GetBeam,
	})
}

func TestBeam_GetBeamByAlias(t *testing.T) {
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
