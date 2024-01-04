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

package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/utils"
)

// TestDiscoveryConfigUnmarshal verifies a DiscoveryConfig resource can be unmarshaled.
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

// TestDiscoveryConfigMarshal verifies a marshaled DiscoveryConfig resource can be unmarshaled back.
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
