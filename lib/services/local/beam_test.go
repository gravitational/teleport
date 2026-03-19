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

package local

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestBeamServiceGetBeamByAlias(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	service, err := NewBeamService(backend)
	require.NoError(t, err)

	beam, err := service.CreateBeam(ctx, testBeam("brisk-otter"))
	require.NoError(t, err)

	require.NoError(t, service.CreateBeamAliasLease(ctx,
		beam.GetStatus().GetAlias(),
		beam.GetMetadata().GetName(),
		beam.GetMetadata().GetExpires().AsTime()),
	)

	got, err := service.GetBeamByAlias(ctx, "brisk-otter")
	require.NoError(t, err)
	require.Equal(t,
		beam.GetMetadata().GetName(),
		got.GetMetadata().GetName(),
	)

	err = service.CreateBeamAliasLease(ctx,
		beam.GetStatus().GetAlias(),
		uuid.NewString(),
		time.Now().Add(1*time.Hour),
	)
	require.True(t, trace.IsAlreadyExists(err))
}

func testBeam(alias string) *beamsv1.Beam {
	return &beamsv1.Beam{
		Kind:    types.KindBeam,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:    uuid.NewString(),
			Expires: timestamppb.New(time.Now().Add(time.Hour)),
		},
		Spec: &beamsv1.BeamSpec{
			Egress:         beamsv1.EgressMode_EGRESS_MODE_RESTRICTED,
			AllowedDomains: []string{"example.com"},
		},
		Status: &beamsv1.BeamStatus{
			User:  "alice",
			Alias: alias,
		},
	}
}
