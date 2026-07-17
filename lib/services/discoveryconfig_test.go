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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/api/constants"
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

// TestUnmarshalPreservesInstallerParamsForClaimedSnapshotSubKind pins the
// tctl-create trust boundary: UnmarshalDiscoveryConfig also parses
// user-supplied YAML, so a claimed static-snapshot subkind (copy-pasted from a
// tctl get dump, or a mistake) must not silently delete a regular config's
// installer params. Sanitization belongs to the isolated snapshot range's
// UnmarshalStaticSnapshotDiscoveryConfig only.
func TestUnmarshalPreservesInstallerParamsForClaimedSnapshotSubKind(t *testing.T) {
	dc, err := discoveryconfig.NewDiscoveryConfig(
		header.Metadata{Name: "user-config"},
		discoveryconfig.Spec{
			DiscoveryGroup: "group",
			AWS: []types.AWSMatcher{{
				Types:   []string{types.AWSMatcherEC2},
				Regions: []string{"us-east-1"},
				Params: &types.InstallerParams{
					JoinToken: "my-enrollment-token",
				},
			}},
		},
	)
	require.NoError(t, err)
	dc.SetSubKind(discoveryconfig.SubKindStaticSnapshot)
	wantParams := dc.Spec.AWS[0].Params

	data, err := MarshalDiscoveryConfig(dc)
	require.NoError(t, err)

	parsed, err := UnmarshalDiscoveryConfig(data)
	require.NoError(t, err)
	// The round trip must carry the claimed subkind: were the marshal to
	// strip it, the assertions below would pass without ever exercising a
	// config that claims to be a snapshot.
	require.Equal(t, discoveryconfig.SubKindStaticSnapshot, parsed.GetSubKind())
	require.Equal(t, wantParams, parsed.Spec.AWS[0].Params,
		"user-supplied configs must keep installer params regardless of claimed subkind")

	sanitized, err := UnmarshalStaticSnapshotDiscoveryConfig(data)
	require.NoError(t, err)
	require.Nil(t, sanitized.Spec.AWS[0].Params,
		"the isolated snapshot range's unmarshal must sanitize")
}

func TestMarshalStaticSnapshotDiscoveryConfigSizeLimit(t *testing.T) {
	dc, err := discoveryconfig.NewStaticSnapshotDiscoveryConfig("server-01", discoveryconfig.Spec{})
	require.NoError(t, err)

	_, err = MarshalStaticSnapshotDiscoveryConfig(dc)
	require.NoError(t, err)

	errorMessage := strings.Repeat("x", discoveryconfig.MaxStaticSnapshotSize)
	dc.Status.ErrorMessage = &errorMessage
	_, err = MarshalStaticSnapshotDiscoveryConfig(dc)
	require.True(t, trace.IsLimitExceeded(err), "got %v", err)
}

// TestMarshalStaticSnapshotDiscoveryConfigRejectsInvalidUTF8Status pins the
// failure mode of the pre-serialization copy: status strings originate in
// cloud APIs and are not guaranteed to be valid UTF-8, which protojson
// refuses to encode. The write must be rejected with the encoding error, not
// panic on a nil copy the way the error-swallowing Clone() would.
func TestMarshalStaticSnapshotDiscoveryConfigRejectsInvalidUTF8Status(t *testing.T) {
	dc, err := discoveryconfig.NewStaticSnapshotDiscoveryConfig("server-01", discoveryconfig.Spec{})
	require.NoError(t, err)

	dc.Status.ServerStatus = map[string]*discoveryconfig.DiscoveryStatusServer{
		"server-01": {DiscoveryStatusServer: discoveryconfigv1.DiscoveryStatusServer_builder{
			IntegrationSummaries: map[string]*discoveryconfigv1.DiscoverSummary{
				"bad-\xff-integration": {},
			},
		}.Build()},
	}
	_, err = MarshalStaticSnapshotDiscoveryConfig(dc)
	require.ErrorContains(t, err, "UTF-8")
}

func TestStaticSnapshotIntegrationEC2MatcherStorageRoundtrip(t *testing.T) {
	t.Setenv(constants.UnstableEnableEICEEnvVar, "")

	dc, err := discoveryconfig.NewStaticSnapshotDiscoveryConfig("server-01", discoveryconfig.Spec{
		AWS: []types.AWSMatcher{{
			Types:       []string{types.AWSMatcherEC2},
			Regions:     []string{"us-east-1"},
			Integration: "integration",
		}},
		GCP: []types.GCPMatcher{{
			Types:      []string{types.GCPMatcherCompute},
			ProjectIDs: []string{"project"},
		}},
	})
	require.NoError(t, err)

	data, err := MarshalStaticSnapshotDiscoveryConfig(dc)
	require.NoError(t, err)
	require.NotContains(t, string(data), "\"params\"", "stored snapshots must not contain installer params")
	got, err := UnmarshalStaticSnapshotDiscoveryConfig(data)
	require.NoError(t, err)
	require.Equal(t, "integration", got.Spec.AWS[0].Integration)
	require.Nil(t, got.Spec.AWS[0].Params)
	require.Nil(t, got.Spec.AWS[0].SSM,
		"the storage path must not derive an SSM document the publisher never sent")
	require.Nil(t, got.Spec.GCP[0].Params)
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
				IntegrationDiscoveredSummary: discoveryconfigv1.IntegrationDiscoveredSummary_builder{
					AwsEc2: discoveryconfigv1.ResourcesDiscoveredSummary_builder{
						Found:  5,
						Failed: 5,
					}.Build(),
				}.Build(),
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
