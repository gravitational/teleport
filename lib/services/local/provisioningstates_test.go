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
	"github.com/gravitational/teleport/lib/services"
)

func TestProvisioningUpdate(t *testing.T) {
	ctx := context.Background()

	backend, err := lite.NewWithConfig(ctx, lite.Config{
		Path:  t.TempDir(),
		Clock: clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	uut, err := NewProvisioningStateService(backend)
	require.NoError(t, err)

	t.Run("downstream is honored", func(t *testing.T) {
		downstreamA := services.DownstreamID("a")
		downstreamB := services.DownstreamID("b")

		// GIVEN a user provisioning state on one downstream
		stateA, err := uut.CreateProvisioningState(ctx, mkUserProvisioningState(
			"someuser@example.com",
			downstreamA,
			provisioningv1.ProvisioningState_PROVISIONING_STATE_STALE))
		require.NoError(t, err)

		// WHEN I try to create a provisioning state for the same user on a
		// *different* downstream
		stateB, err := uut.CreateProvisioningState(ctx, mkUserProvisioningState(
			"someuser@example.com",
			downstreamB,
			provisioningv1.ProvisioningState_PROVISIONING_STATE_PROVISIONED))

		// EXPECT that the operation succeeds
		require.NoError(t, err)

		// WHEN I try to fetch the user from Downstream A
		retrievedStateA, err := uut.GetProvisioningState(ctx, downstreamA, services.ProvisioningStateID(stateA.Metadata.Name))

		// EXPECT the operation to succeed, and  to have retrieved the correct record
		require.NoError(t, err)
		require.Equal(t, string(downstreamA), retrievedStateA.Spec.DownstreamId)
		require.Equal(t,
			provisioningv1.ProvisioningState_PROVISIONING_STATE_STALE,
			retrievedStateA.Status.ProvisioningState)

		// WHEN I try to fetch the user from Downstream B
		retrievedStateB, err := uut.GetProvisioningState(ctx, downstreamA, services.ProvisioningStateID(stateB.Metadata.Name))

		// EXPECT the operation to succeed, and  to have retrieved the correct record
		require.NoError(t, err)
		require.Equal(t, string(downstreamA), retrievedStateB.Spec.DownstreamId)
		require.Equal(t,
			provisioningv1.ProvisioningState_PROVISIONING_STATE_STALE,
			retrievedStateB.Status.ProvisioningState)

	})

	t.Run("locking", func(t *testing.T) {
		// GIVEN a service with an existing provisioning state
		downstreamID := services.DownstreamID("some-scim-server")
		s0, err := uut.CreateProvisioningState(ctx, mkUserProvisioningState(
			"someuser@example.com",
			downstreamID,
			provisioningv1.ProvisioningState_PROVISIONING_STATE_STALE))
		require.NoError(t, err)

		// GIVEN also that the resource has been updated...
		s1, err := uut.GetProvisioningState(ctx, downstreamID, services.ProvisioningStateID(s0.Metadata.Name))
		require.NoError(t, err)
		s1.Status.ProvisioningState = provisioningv1.ProvisioningState_PROVISIONING_STATE_PROVISIONED
		_, err = uut.UpdateProvisioningState(ctx, s1)
		require.NoError(t, err)

		// WHEN I try to update the resource based on the original version...
		s0.Status.ProvisioningState = provisioningv1.ProvisioningState_PROVISIONING_STATE_STALE
		s0.Status.Error = "I can't find the database"
		_, err = uut.UpdateProvisioningState(ctx, s0)

		// EXPECT the update to fail due to optimistic locking
		require.Error(t, err)
		require.True(t, trace.IsCompareFailed(err))
	})
}

func mkUserProvisioningState(username string, downstream services.DownstreamID, initialStatus provisioningv1.ProvisioningState) *provisioningv1.PrincipalState {
	return &provisioningv1.PrincipalState{
		Kind:    types.KindProvisioningPrincipalState,
		SubKind: "",
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "u-" + username,
		},
		Spec: &provisioningv1.PrincipalStateSpec{
			DownstreamId:  string(downstream),
			PrincipalType: provisioningv1.PrincipalType_PRINCIPAL_TYPE_USER,
			PrincipalId:   username,
		},
		Status: &provisioningv1.PrincipalStateStatus{
			ProvisioningState: initialStatus,
		},
	}
}
