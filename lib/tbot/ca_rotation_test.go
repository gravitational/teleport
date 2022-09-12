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

package tbot

import (
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/stretchr/testify/require"
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

	log := utils.NewLoggerForTests()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ignoreReason := filterCAEvent(log, tt.event, clusterName)
			require.Equal(t, tt.expectedIgnoreReason, ignoreReason)
		})
	}
}
