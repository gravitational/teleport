/*
Copyright 2026 Gravitational, Inc.

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
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
)

func TestNewSyntheticDiscoveryConfig(t *testing.T) {
	const serverID = "00000000-0000-0000-0000-000000000001"

	for _, tt := range []struct {
		name       string
		inServerID string
		inSpec     Spec
		expected   *DiscoveryConfig
		errCheck   require.ErrorAssertionFunc
	}{
		{
			name:       "moves discovery group to label and uses name as sentinel group",
			inServerID: serverID,
			inSpec: Spec{
				DiscoveryGroup: "dg1",
				Kube: []types.KubernetesMatcher{{
					Types: []string{"app"},
				}},
			},
			expected: &DiscoveryConfig{
				ResourceHeader: header.ResourceHeader{
					Kind:    types.KindDiscoveryConfig,
					SubKind: SubKindSynthetic,
					Version: types.V1,
					Metadata: header.Metadata{
						Name: "synthetic-" + serverID,
						Labels: map[string]string{
							types.OriginLabel:                        types.OriginConfigFile,
							types.TeleportInternalDiscoveryGroupName: "dg1",
						},
					},
				},
				Spec: Spec{
					DiscoveryGroup: "synthetic-" + serverID,
					AWS:            make([]types.AWSMatcher, 0),
					Azure:          make([]types.AzureMatcher, 0),
					GCP:            make([]types.GCPMatcher, 0),
					Kube: []types.KubernetesMatcher{{
						Types:      []string{"app"},
						Namespaces: []string{"*"},
						Labels:     types.Labels{"*": []string{"*"}},
					}},
				},
			},
			errCheck: require.NoError,
		},
		{
			name:       "no discovery group label when the service has no group",
			inServerID: serverID,
			inSpec: Spec{
				Kube: []types.KubernetesMatcher{{
					Types: []string{"app"},
				}},
			},
			expected: &DiscoveryConfig{
				ResourceHeader: header.ResourceHeader{
					Kind:    types.KindDiscoveryConfig,
					SubKind: SubKindSynthetic,
					Version: types.V1,
					Metadata: header.Metadata{
						Name: "synthetic-" + serverID,
						Labels: map[string]string{
							types.OriginLabel: types.OriginConfigFile,
						},
					},
				},
				Spec: Spec{
					DiscoveryGroup: "synthetic-" + serverID,
					AWS:            make([]types.AWSMatcher, 0),
					Azure:          make([]types.AzureMatcher, 0),
					GCP:            make([]types.GCPMatcher, 0),
					Kube: []types.KubernetesMatcher{{
						Types:      []string{"app"},
						Namespaces: []string{"*"},
						Labels:     types.Labels{"*": []string{"*"}},
					}},
				},
			},
			errCheck: require.NoError,
		},
		{
			name:       "error when server ID is not present",
			inServerID: "",
			inSpec: Spec{
				DiscoveryGroup: "dg1",
			},
			errCheck: requireBadParameter,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewSyntheticDiscoveryConfig(tt.inServerID, tt.inSpec)
			if tt.errCheck != nil {
				tt.errCheck(t, err)
			}
			if tt.expected != nil {
				require.Equal(t, tt.expected, got)
				require.True(t, got.IsSynthetic())
			}
		})
	}
}

func TestCheckSyntheticDiscoveryConfig(t *testing.T) {
	const serverID = "00000000-0000-0000-0000-000000000001"

	for _, tt := range []struct {
		name     string
		mutate   func(dc *DiscoveryConfig)
		errCheck require.ErrorAssertionFunc
	}{
		{
			name:     "valid",
			mutate:   func(dc *DiscoveryConfig) {},
			errCheck: require.NoError,
		},
		{
			name: "error when not synthetic",
			mutate: func(dc *DiscoveryConfig) {
				dc.SetSubKind("")
			},
			errCheck: requireBadParameter,
		},
		{
			name: "error when name is not derived from the server ID",
			mutate: func(dc *DiscoveryConfig) {
				dc.Metadata.Name = SyntheticName("another-server-id")
			},
			errCheck: requireBadParameter,
		},
		{
			name: "error when discovery group is not the sentinel",
			mutate: func(dc *DiscoveryConfig) {
				dc.Spec.DiscoveryGroup = "dg1"
			},
			errCheck: requireBadParameter,
		},
		{
			name: "error when expiry is not present",
			mutate: func(dc *DiscoveryConfig) {
				dc.SetExpiry(time.Time{})
			},
			errCheck: requireBadParameter,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dc, err := NewSyntheticDiscoveryConfig(serverID, Spec{
				DiscoveryGroup: "dg1",
				Kube: []types.KubernetesMatcher{{
					Types: []string{"app"},
				}},
			})
			require.NoError(t, err)
			dc.SetExpiry(time.Now().Add(time.Hour))

			tt.mutate(dc)
			tt.errCheck(t, CheckSyntheticDiscoveryConfig(dc, serverID))
		})
	}
}
