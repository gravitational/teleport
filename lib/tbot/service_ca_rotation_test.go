/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package tbot

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

func Test_filterCAEvent(t *testing.T) {
	clusterName := "example.com"
	createCertAuthority := func(t *testing.T, modifier func(spec *types.CertAuthoritySpecV2)) types.CertAuthority {
		t.Helper()
		validSpec := types.CertAuthoritySpecV2{
			ClusterName: clusterName,
			Type:        "host",
			Rotation: &types.Rotation{
				Phase: "update_clients",
			},
		}

		if modifier != nil {
			modifier(&validSpec)
		}

		ca, err := types.NewCertAuthority(validSpec)
		require.NoError(t, err)
		return ca
	}

	tests := []struct {
		name                 string
		event                types.Event
		expectedIgnoreReason string
	}{
		{
			name: "valid host CA rotation",
			event: types.Event{
				Type:     types.OpPut,
				Resource: createCertAuthority(t, nil),
			},
		},
		{
			name: "valid user CA rotation",
			event: types.Event{
				Type: types.OpPut,
				Resource: createCertAuthority(t, func(spec *types.CertAuthoritySpecV2) {
					spec.Type = "user"
				}),
			},
		},
		{
			name: "valid DB CA rotation",
			event: types.Event{
				Type: types.OpPut,
				Resource: createCertAuthority(t, func(spec *types.CertAuthoritySpecV2) {
					spec.Type = "db"
				}),
			},
		},
		{
			name: "wrong type",
			event: types.Event{
				Type:     types.OpDelete,
				Resource: createCertAuthority(t, nil),
			},
			expectedIgnoreReason: "type not PUT",
		},
		{
			name: "wrong underlying resource",
			event: types.Event{
				Type:     types.OpPut,
				Resource: &types.Namespace{},
			},
			expectedIgnoreReason: "event resource was not CertAuthority (*types.Namespace)",
		},
		{
			name: "wrong phase",
			event: types.Event{
				Type: types.OpPut,
				Resource: createCertAuthority(t, func(spec *types.CertAuthoritySpecV2) {
					spec.Rotation.Phase = "init"
				}),
			},
			expectedIgnoreReason: "skipping due to phase 'init'",
		},
		{
			name: "wrong cluster name",
			event: types.Event{
				Type: types.OpPut,
				Resource: createCertAuthority(t, func(spec *types.CertAuthoritySpecV2) {
					spec.ClusterName = "wrong"
				}),
			},
			expectedIgnoreReason: "skipping due to cluster name of CA: was 'wrong', wanted 'example.com'",
		},
		{
			name: "wrong CA type",
			event: types.Event{
				Type: types.OpPut,
				Resource: createCertAuthority(t, func(spec *types.CertAuthoritySpecV2) {
					spec.Type = "jwt"
				}),
			},
			expectedIgnoreReason: "skipping due to CA kind 'jwt'",
		},
	}

	ctx := context.Background()
	log := utils.NewSlogLoggerForTests()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ignoreReason := filterCAEvent(ctx, log, tt.event, clusterName)
			require.Equal(t, tt.expectedIgnoreReason, ignoreReason)
		})
	}
}

func TestChannelBroadcaster(t *testing.T) {
	cb := channelBroadcaster{chanSet: map[chan struct{}]struct{}{}}
	sub1, unsubscribe1 := cb.subscribe()
	t.Cleanup(unsubscribe1)
	sub2, unsubscribe2 := cb.subscribe()
	t.Cleanup(unsubscribe2)

	cb.broadcast()
	require.NotEmpty(t, sub1)
	require.NotEmpty(t, sub2)

	// remove value from sub1 to check that if sub2 is full broadcasting still
	// works
	<-sub1
	cb.broadcast()
	require.NotEmpty(t, sub1)

	// empty out both channels and ensure unsubscribing means they no longer
	// receive values
	<-sub1
	<-sub2
	unsubscribe1()
	unsubscribe2()
	cb.broadcast()
	require.Empty(t, sub1)
	require.Empty(t, sub2)

	// ensure unsubscribing twice doesn't cause panic
	unsubscribe1()
}
