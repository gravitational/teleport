/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package services

import (
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

type clusterConfig struct {
	types.ClusterConfig
}

func TestUpdateAuthPreferenceWithLegacyClusterConfig(t *testing.T) {
	cases := []struct {
		desc          string
		clusterConfig func() types.ClusterConfig
		assertion     require.ErrorAssertionFunc
	}{
		{
			desc: "unexpected-cluster-config",
			clusterConfig: func() types.ClusterConfig {
				return &clusterConfig{}
			},
			assertion: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err))
			},
		},
		{
			desc:          "cluster-config-without-legacy-data",
			clusterConfig: types.DefaultClusterConfig,
			assertion:     require.NoError,
		},
		{
			desc: "cluster-config-with-legacy-data",
			clusterConfig: func() types.ClusterConfig {
				cc, err := types.NewClusterConfig(types.ClusterConfigSpecV3{
					LegacyClusterConfigAuthFields: &types.LegacyClusterConfigAuthFields{
						DisconnectExpiredCert: true,
						AllowLocalAuth:        true,
					},
				})
				require.NoError(t, err)
				return cc
			},
			assertion: require.NoError,
		},
	}

	for _, tt := range cases {
		t.Run(tt.desc, func(t *testing.T) {
			tt.assertion(t, UpdateAuthPreferenceWithLegacyClusterConfig(tt.clusterConfig(), types.DefaultAuthPreference()))
		})
	}

}
