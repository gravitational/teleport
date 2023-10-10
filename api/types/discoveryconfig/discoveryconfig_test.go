/*
Copyright 2023 Gravitational, Inc.

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

package discoveryconfig

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
)

func requireBadParameter(t require.TestingT, err error, i ...interface{}) {
	require.True(
		t,
		trace.IsBadParameter(err),
		"err should be bad parameter, was: %s", err,
	)
}

func TestNewDiscoveryConfig(t *testing.T) {
	for _, tt := range []struct {
		name       string
		inMetadata header.Metadata
		inSpec     Spec
		expected   *DiscoveryConfig
		errCheck   require.ErrorAssertionFunc
	}{
		{
			name: "valid",
			inMetadata: header.Metadata{
				Name: "my-first-dc",
			},
			inSpec: Spec{
				DiscoveryGroup: "dg1",
			},
			expected: &DiscoveryConfig{
				ResourceHeader: header.ResourceHeader{
					Kind:    types.KindDiscoveryConfig,
					Version: types.V1,
					Metadata: header.Metadata{
						Name: "my-first-dc",
					},
				},
				Spec: Spec{
					DiscoveryGroup: "dg1",
					AWS:            make([]types.AWSMatcher, 0),
					Azure:          make([]types.AzureMatcher, 0),
					GCP:            make([]types.GCPMatcher, 0),
					Kube:           make([]types.KubernetesMatcher, 0),
				},
			},
			errCheck: require.NoError,
		},
		{
			name: "error when name is not present",
			inMetadata: header.Metadata{
				Name: "",
			},
			inSpec: Spec{
				DiscoveryGroup: "dg1",
			},
			errCheck: requireBadParameter,
		},
		{
			name: "error when discovery group is not present",
			inMetadata: header.Metadata{
				Name: "my-first-dc",
			},
			inSpec: Spec{
				DiscoveryGroup: "",
			},
			errCheck: requireBadParameter,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewDiscoveryConfig(tt.inMetadata, tt.inSpec)
			if tt.errCheck != nil {
				tt.errCheck(t, err)
			}

			if tt.expected != nil {
				require.Equal(t, tt.expected, got)
			}
		})
	}
}
