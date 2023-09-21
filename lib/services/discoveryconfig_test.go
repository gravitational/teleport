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

package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/utils"
)

// TestDiscoveryConfigUnmarshal verifies an access list resource can be unmarshaled.
func TestDiscoveryConfigUnmarshal(t *testing.T) {
	expected, err := discoveryconfig.NewDiscoveryConfig(
		header.Metadata{
			Name: "test-discovery-config",
		},
		discoveryconfig.Spec{
			DiscoveryGroup: "dg01",
			AWS: []types.AWSMatcher{
				{
					Types:   []string{"ec2"},
					Regions: []string{"eu-west-2"},
				},
			},
		},
	)
	require.NoError(t, err)
	data, err := utils.ToJSON([]byte(discoveryConfigYAML))
	require.NoError(t, err)
	actual, err := UnmarshalDiscoveryConfig(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

// TestDiscoveryConfigMarshal verifies a marshaled access list resource can be unmarshaled back.
func TestDiscoveryConfigMarshal(t *testing.T) {
	expected, err := discoveryconfig.NewDiscoveryConfig(
		header.Metadata{
			Name: "test-discovery-config",
		},
		discoveryconfig.Spec{
			DiscoveryGroup: "dg01",
			AWS: []types.AWSMatcher{
				{
					Types:   []string{"ec2"},
					Regions: []string{"eu-west-2"},
				},
			},
		},
	)
	require.NoError(t, err)
	data, err := MarshalDiscoveryConfig(expected)
	require.NoError(t, err)
	actual, err := UnmarshalDiscoveryConfig(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

var discoveryConfigYAML = `---
kind: discovery_group
version: v1
metadata:
  name: test-discovery-config
spec:
  discovery_group: dg01
  aws:
  - types: ["ec2"]
    regions: ["eu-west-2"]
`
