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

package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestStatusModelStructuredOutput(t *testing.T) {
	model := &statusModel{
		cluster: &clusterStatusModel{
			name:    "root.example.com",
			version: "19.0.0",
			caPins:  []string{"sha256:abc"},
		},
		authorities: []*authorityStatusModel{
			{
				clusterName:    "root.example.com",
				authorityType:  types.HostCA,
				rotationStatus: types.Rotation{State: types.RotationStateStandby},
				activeKeys: []*authorityKeyModel{
					{protocol: "TLS", algo: "ECDSA P-256", storage: "software"},
				},
				additionalTrustedKeys: []*authorityKeyModel{
					{protocol: "SSH", algo: "Ed25519", storage: "software"},
				},
			},
			{
				clusterName:    "leaf.example.com",
				authorityType:  types.UserCA,
				rotationStatus: types.Rotation{State: types.RotationStateStandby},
				activeKeys: []*authorityKeyModel{
					{protocol: "SSH", algo: "Ed25519", storage: "software"},
				},
			},
		},
	}

	out := model.output(false)
	require.Equal(t, "root.example.com", out.Cluster.Name)
	require.Len(t, out.Authorities, 1)
	require.Equal(t, []authorityKeyOutput{
		{Protocol: "SSH", Status: "trusted", Algorithm: "Ed25519", Storage: "software"},
		{Protocol: "TLS", Status: "active", Algorithm: "ECDSA P-256", Storage: "software"},
	}, out.Authorities[0].Keys)

	out = model.output(true)
	require.Len(t, out.Authorities, 2)

	// A never-rotated standby CA should omit time-valued fields so consumers
	// don't have to compare against Go's zero time.
	require.Equal(t, types.RotationStateStandby, out.Authorities[0].Rotation.State)
	require.Nil(t, out.Authorities[0].Rotation.LastRotated)
	require.Nil(t, out.Authorities[0].Rotation.Started)
	require.Nil(t, out.Authorities[0].Rotation.Schedule)
}

func TestRotationOutputOmitsZeroTimes(t *testing.T) {
	rotated := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	out := newRotationOutput(types.Rotation{
		State:       types.RotationStateInProgress,
		Phase:       types.RotationPhaseUpdateClients,
		LastRotated: rotated,
		Schedule:    types.RotationSchedule{Standby: rotated},
	})
	require.Equal(t, types.RotationStateInProgress, out.State)
	require.NotNil(t, out.LastRotated)
	require.Equal(t, rotated, *out.LastRotated)
	require.Nil(t, out.Started)
	require.NotNil(t, out.Schedule)
	require.NotNil(t, out.Schedule.Standby)
	require.Nil(t, out.Schedule.UpdateClients)
	require.Empty(t, out.GracePeriod)
}
