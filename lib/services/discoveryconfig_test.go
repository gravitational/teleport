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

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
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

func TestUnmarshalStatusCountersAsInt(t *testing.T) {
	// We changed to protojson so that sync timers use a human readable format (instead of unix timestamps).
	// However, using protojson also changed the way numeric fields are stored in the backend: from int64 to string.
	// See: https://github.com/golang/protobuf/issues/1414
	// This test simulates a scenario where the stored value for IntegrationDiscoveredSummary used int64 and ensures it doesn't cause any issues when loading the resource.

	const legacyYAML = `---
kind: discovery_group
version: v1
metadata:
  name: test-discovery-config
spec:
  discovery_group: dg01
  aws:
  - types: ["ec2"]
    regions: ["eu-west-2"]
status:
  integration_discovered_resources:
    integration:
      awsEc2:
        failed: 5
        found: 5
`
	dataLegacy, err := utils.ToJSON([]byte(legacyYAML))
	require.NoError(t, err)

	const newFormat = `---
kind: discovery_group
version: v1
metadata:
  name: test-discovery-config
spec:
  discovery_group: dg01
  aws:
  - types: ["ec2"]
    regions: ["eu-west-2"]
status:
  integration_discovered_resources:
    integration:
      awsEc2:
        failed: "5"
        found: "5"
`
	dataNewFormat, err := utils.ToJSON([]byte(newFormat))
	require.NoError(t, err)

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
	expected.Status = discoveryconfig.Status{
		IntegrationDiscoveredResources: map[string]*discoveryconfig.IntegrationDiscoveredSummary{
			"integration": {
				IntegrationDiscoveredSummary: &discoveryconfigv1.IntegrationDiscoveredSummary{
					AwsEc2: &discoveryconfigv1.ResourcesDiscoveredSummary{
						Found:  5,
						Failed: 5,
					},
				},
			},
		},
	}
	actualLegacy, err := UnmarshalDiscoveryConfig(dataLegacy)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(expected.Status, actualLegacy.Status, protocmp.Transform()))

	actualNewFormat, err := UnmarshalDiscoveryConfig(dataNewFormat)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(expected.Status, actualNewFormat.Status, protocmp.Transform()))
}
