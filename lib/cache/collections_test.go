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

package cache

import (
	"context"
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/stretchr/testify/require"
)

func TestClusterConfig_StoreDerived(t *testing.T) {
	ctx := context.Background()
	p := newPackForAuth(t)
	t.Cleanup(p.Close)

	cfg := clusterConfig{
		Cache: p.cache,
	}

	cases := []struct {
		desc          string
		clusterConfig func() types.ClusterConfig
		assertion     require.ErrorAssertionFunc
	}{
		{
			desc:          "no-derived-resources",
			clusterConfig: types.DefaultClusterConfig,
			assertion:     require.NoError,
		},
		{
			desc: "with-derived-resources",
			clusterConfig: func() types.ClusterConfig {
				cfg := types.DefaultClusterConfig()
				require.NoError(t, cfg.SetAuditConfig(types.DefaultClusterAuditConfig()))
				require.NoError(t, cfg.SetNetworkingFields(types.DefaultClusterNetworkingConfig()))
				require.NoError(t, cfg.SetSessionRecordingFields(types.DefaultSessionRecordingConfig()))
				return cfg
			},
			assertion: require.NoError,
		},
	}

	for _, tt := range cases {
		t.Run(tt.desc, func(t *testing.T) {
			tt.assertion(t, cfg.storeDerivedResources(tt.clusterConfig(), types.DefaultAuthPreference())(ctx))
		})
	}
}
