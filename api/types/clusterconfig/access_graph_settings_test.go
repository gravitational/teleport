/*
Copyright 2024 Gravitational, Inc.

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

package clusterconfig

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestNewAccessGraphSettings(t *testing.T) {
	tests := []struct {
		name      string
		spec      *clusterconfigpb.AccessGraphSettingsSpec
		want      *clusterconfigpb.AccessGraphSettings
		assertErr func(*testing.T, error, ...any)
	}{
		{
			name: "success disabled",
			spec: &clusterconfigpb.AccessGraphSettingsSpec{
				SecretsScanConfig: clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_DISABLED,
			},
			assertErr: func(t *testing.T, err error, a ...any) {
				require.NoError(t, err)
			},
			want: &clusterconfigpb.AccessGraphSettings{
				Kind:    types.KindAccessGraphSettings,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: types.MetaNameAccessGraphSettings,
				},
				Spec: &clusterconfigpb.AccessGraphSettingsSpec{
					SecretsScanConfig: clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_DISABLED,
				},
			},
		},
		{
			name: "success enabled",
			spec: &clusterconfigpb.AccessGraphSettingsSpec{
				SecretsScanConfig: clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_ENABLED,
			},
			assertErr: func(t *testing.T, err error, a ...any) {
				require.NoError(t, err)
			},
			want: &clusterconfigpb.AccessGraphSettings{
				Kind:    types.KindAccessGraphSettings,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: types.MetaNameAccessGraphSettings,
				},
				Spec: &clusterconfigpb.AccessGraphSettingsSpec{
					SecretsScanConfig: clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_ENABLED,
				},
			},
		},
		{
			name: "invalid",
			spec: &clusterconfigpb.AccessGraphSettingsSpec{
				SecretsScanConfig: 10,
			},
			assertErr: func(t *testing.T, err error, a ...any) {
				require.ErrorContains(t, err, "SecretsScanConfig is invalid")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewAccessGraphSettings(tt.spec)
			tt.assertErr(t, err)
			require.Empty(t, cmp.Diff(got, tt.want, protocmp.Transform()))
		})
	}
}
