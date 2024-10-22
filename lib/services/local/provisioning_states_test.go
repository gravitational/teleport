// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package local

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/lite"
)

func TestProvisioningUpdate(t *testing.T) {
	ctx := context.Background()

	backend, err := lite.NewWithConfig(ctx, lite.Config{
		Path:  t.TempDir(),
		Clock: clockwork.NewFakeClock(),
	})

	// backend, err := memory.New(memory.Config{
	// 	Context: ctx,
	// 	Clock:   clockwork.NewFakeClock(),
	// })
	require.NoError(t, err)

	uut, err := NewProvisioningStateService(backend)
	require.NoError(t, err)

	t.Run("locking", func(t *testing.T) {
		// GIVEN a service with an existing provisioning state
		s0, err := uut.CreateProvisioningState(ctx, &provisioningv1.PrincipalState{
			Kind:    types.KindProvisioningState,
			SubKind: "",
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "u-someuser@example.com",
			},
			Spec: &provisioningv1.PrincipalStateSpec{
				DownstreamId:  "",
				PrincipalType: provisioningv1.PrincipalType_PRINCIPAL_TYPE_USER,
				PrincipalId:   "someuser@example.com",
			},
			Status: &provisioningv1.PrincipalStateStatus{
				Status: provisioningv1.Status_STATUS_STALE,
			},
		})
		require.NoError(t, err)

		// GIVEN also that the resource has been updated...
		s1, err := uut.GetProvisioningState(ctx, s0.Metadata.Name)
		require.NoError(t, err)
		s1.Status.Status = provisioningv1.Status_STATUS_PROVISIONED
		_, err = uut.UpdateProvisioningState(ctx, s1)
		require.NoError(t, err)

		// WHEN I try to update the resource based on the original version...
		s0.Status.Status = provisioningv1.Status_STATUS_ERROR
		s0.Status.Error = "I can't find the database"
		_, err = uut.UpdateProvisioningState(ctx, s0)

		// EXPECT the update to fail due to optimistic locking
		require.Error(t, err)
		require.True(t, trace.IsCompareFailed(err))
	})
}
