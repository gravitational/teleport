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
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

type clusterConfig struct {
}

func (c clusterConfig) GetKind() string {
	return ""
}

func (c clusterConfig) GetSubKind() string {
	return ""
}

func (c clusterConfig) SetSubKind(s string) {}

func (c clusterConfig) GetVersion() string {
	return ""
}

func (c clusterConfig) GetName() string {
	return ""
}

func (c clusterConfig) SetName(s string) {}

func (c clusterConfig) Expiry() time.Time {
	return time.Time{}
}

func (c clusterConfig) SetExpiry(time time.Time) {}

func (c clusterConfig) GetMetadata() types.Metadata {
	return types.Metadata{}
}

func (c clusterConfig) GetResourceID() int64 {
	return 0
}

func (c clusterConfig) SetResourceID(i int64) {}

func (c clusterConfig) CheckAndSetDefaults() error {
	return nil
}

func (c clusterConfig) GetLegacyClusterID() string {
	return ""
}

func (c clusterConfig) SetLegacyClusterID(s string) {}

func (c clusterConfig) HasAuditConfig() bool {
	return false
}

func (c clusterConfig) SetAuditConfig(config types.ClusterAuditConfig) error {
	return nil
}

func (c clusterConfig) GetClusterAuditConfig() (types.ClusterAuditConfig, error) {
	return nil, nil
}

func (c clusterConfig) HasNetworkingFields() bool {
	return false
}

func (c clusterConfig) SetNetworkingFields(config types.ClusterNetworkingConfig) error {
	return nil
}

func (c clusterConfig) GetClusterNetworkingConfig() (types.ClusterNetworkingConfig, error) {
	return nil, nil
}

func (c clusterConfig) HasSessionRecordingFields() bool {
	return false
}

func (c clusterConfig) SetSessionRecordingFields(config types.SessionRecordingConfig) error {
	return nil
}

func (c clusterConfig) GetSessionRecordingConfig() (types.SessionRecordingConfig, error) {
	return nil, nil
}

func (c clusterConfig) HasAuthFields() bool {
	return false
}

func (c clusterConfig) SetAuthFields(preference types.AuthPreference) error {
	return nil
}

func (c clusterConfig) Copy() types.ClusterConfig {
	return c
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
