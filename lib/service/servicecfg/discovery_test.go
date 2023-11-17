// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package servicecfg

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestDiscoveryConfig_IsEmpty(t *testing.T) {
	for _, tt := range []struct {
		name     string
		input    DiscoveryConfig
		expected bool
	}{
		{
			name: "returns false when discovery group is present",
			input: DiscoveryConfig{
				DiscoveryGroup: "my-discovery-group",
			},
			expected: false,
		},
		{
			name: "returns false when has at least one aws matcher",
			input: DiscoveryConfig{
				AWSMatchers: []types.AWSMatcher{{
					Types: []string{"ec2"},
				}},
			},
			expected: false,
		},
		{
			name: "returns false when has at least one azure matcher",
			input: DiscoveryConfig{
				AzureMatchers: []types.AzureMatcher{{
					Types: []string{"aks"},
				}},
			},
			expected: false,
		},
		{
			name: "returns false when has at least one gcp matcher",
			input: DiscoveryConfig{
				GCPMatchers: []types.GCPMatcher{{
					Types: []string{"gke"},
				}},
			},
			expected: false,
		},
		{
			name: "returns false when has at least one Kube matcher",
			input: DiscoveryConfig{
				KubernetesMatchers: []types.KubernetesMatcher{{
					Types: []string{"app"},
				}},
			},
			expected: false,
		},
		{
			name:     "returns true when there are no matchers and no discovery group",
			input:    DiscoveryConfig{},
			expected: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.input.IsEmpty()
			require.Equal(t, tt.expected, got)
		})
	}
}
