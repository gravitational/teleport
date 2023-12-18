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
