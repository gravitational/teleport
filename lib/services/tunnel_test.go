/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package services

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
)

func newReverseTunnel(clusterName string, dialAddrs []string) *types.ReverseTunnelV2 {
	return &types.ReverseTunnelV2{
		Kind:    types.KindReverseTunnel,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      clusterName,
			Namespace: apidefaults.Namespace,
		},
		Spec: types.ReverseTunnelSpecV2{
			ClusterName: clusterName,
			DialAddrs:   dialAddrs,
		},
	}
}

func TestValidateReverseTunnel(t *testing.T) {
	tests := []struct {
		name       string
		tunnel     types.ReverseTunnel
		requireErr require.ErrorAssertionFunc
	}{
		{
			name:       "valid tunnel",
			tunnel:     newReverseTunnel("example.com", []string{"example.com:3022"}),
			requireErr: require.NoError,
		},
		{
			name:   "empty cluster name",
			tunnel: newReverseTunnel("", []string{"example.com:3022"}),
			requireErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err))
			},
		},
		{
			name:   "empty dial address",
			tunnel: newReverseTunnel("example.com", []string{""}),
			requireErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err))
			},
		},
		{
			name:   "no dial address",
			tunnel: newReverseTunnel("example.com", []string{}),
			requireErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.requireErr(t, ValidateReverseTunnel(tt.tunnel))
		})
	}
}
