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

package v1

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/types/header"
)

func TestRoundtrip(t *testing.T) {
	discoveryConfig := newDiscoveryConfig(t, "discovery-config-01")

	converted, err := FromProtoWithSubKind(ToProto(discoveryConfig))
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(discoveryConfig, converted))
}

// TestRoundtripStaticSnapshot ensures a static snapshot survives the proto conversion in
// both directions: the subkind marks it for authorization and consumption filtering,
// and the spec carries the observed inventory.
func TestRoundtripStaticSnapshot(t *testing.T) {
	discoveryConfig, err := discoveryconfig.NewStaticSnapshotDiscoveryConfig("server-01", discoveryconfig.Spec{
		DiscoveryGroup: "discovery-group-01",
		AWS: []types.AWSMatcher{{
			Types:   []string{types.AWSMatcherEC2},
			Regions: []string{"eu-west-2"},
		}},
	})
	require.NoError(t, err)
	discoveryConfig.SetExpiry(time.Now().UTC().Truncate(time.Second).Add(time.Hour))

	converted, err := FromProtoWithSubKind(ToProto(discoveryConfig))
	require.NoError(t, err)

	require.True(t, converted.IsStaticSnapshot())
	require.Empty(t, cmp.Diff(discoveryConfig, converted))

	// A snapshot of a service with no discovery group must roundtrip too:
	// only the static-snapshot subkind relaxes the group requirement.
	groupless, err := discoveryconfig.NewStaticSnapshotDiscoveryConfig("server-01", discoveryconfig.Spec{})
	require.NoError(t, err)
	converted, err = FromProtoWithSubKind(ToProto(groupless))
	require.NoError(t, err)
	require.True(t, converted.IsStaticSnapshot())
	require.Empty(t, converted.GetDiscoveryGroup())
}

// TestRoundtripEmptySnapshotNormalizesMatcherFamilies pins the nil-versus-
// empty representation of an inventory-less snapshot across the proto
// conversion: proto repeated fields cannot distinguish nil from empty, so
// the roundtrip must land every matcher family on the validation-normalized
// form (an empty, non-nil list), identical to the source resource.
func TestRoundtripEmptySnapshotNormalizesMatcherFamilies(t *testing.T) {
	groupless, err := discoveryconfig.NewStaticSnapshotDiscoveryConfig("server-01", discoveryconfig.Spec{})
	require.NoError(t, err)

	converted, err := FromProtoWithSubKind(ToProto(groupless))
	require.NoError(t, err)

	require.NotNil(t, converted.Spec.AWS, "AWS matchers must normalize to an empty list, not nil")
	require.Empty(t, converted.Spec.AWS)
	require.NotNil(t, converted.Spec.Azure, "Azure matchers must normalize to an empty list, not nil")
	require.Empty(t, converted.Spec.Azure)
	require.NotNil(t, converted.Spec.GCP, "GCP matchers must normalize to an empty list, not nil")
	require.Empty(t, converted.Spec.GCP)
	require.NotNil(t, converted.Spec.Kube, "Kube matchers must normalize to an empty list, not nil")
	require.Empty(t, converted.Spec.Kube)

	// cmp.Diff distinguishes nil from empty slices, so full equality with
	// the source pins the representation of every remaining field too.
	require.Empty(t, cmp.Diff(groupless, converted))
}

// TestFromProtoSanitizesStaticSnapshot ensures received snapshots have
// installer params discarded before validation, in their entirety: installer
// params are excluded from snapshot inventory including non-secret fields
// such as EnrollMode. The conversion must not mutate its input.
func TestFromProtoSanitizesStaticSnapshot(t *testing.T) {
	discoveryConfig, err := discoveryconfig.NewStaticSnapshotDiscoveryConfig("server-01", discoveryconfig.Spec{
		AWS: []types.AWSMatcher{{
			Types:   []string{types.AWSMatcherEC2},
			Regions: []string{"eu-west-2"},
		}},
	})
	require.NoError(t, err)
	msg := ToProto(discoveryConfig)
	params := &types.InstallerParams{
		EnrollMode: types.InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_SCRIPT,
		JoinToken:  "secret-token",
		HTTPProxySettings: &types.HTTPProxySettings{
			HTTPSProxy: "not-a-valid-proxy-with-secret",
		},
	}
	msg.Spec.Aws[0].Params = params
	before := proto.Clone(msg).(*discoveryconfigv1.DiscoveryConfig)

	converted, err := FromProtoWithSubKind(msg)
	require.NoError(t, err)
	require.Nil(t, converted.Spec.AWS[0].Params)
	require.True(t, proto.Equal(before, msg), "conversion must not mutate its input")
}

// TestFromProtoPreservesUnknownSubKind ensures conversion preserves unknown
// subkinds without weakening regular DiscoveryConfig validation.
func TestFromProtoPreservesUnknownSubKind(t *testing.T) {
	msg := ToProto(newDiscoveryConfig(t, "discovery-config-01"))
	msg.Header.SubKind = "some-future-subkind"

	converted, err := FromProtoWithSubKind(msg)
	require.NoError(t, err)
	require.Equal(t, "some-future-subkind", converted.GetSubKind())
	require.Equal(t, "discovery-group-01", converted.Spec.DiscoveryGroup)

	msg.Spec.DiscoveryGroup = ""
	_, err = FromProtoWithSubKind(msg)
	require.True(t, trace.IsBadParameter(err), "got %v", err)
}

// TestFromProtoDiscardsSubKind pins the generic-write conversion: any
// client-supplied subkind is discarded, installer params survive regardless
// of the claimed subkind, and regular validation applies in full.
func TestFromProtoDiscardsSubKind(t *testing.T) {
	msg := ToProto(newDiscoveryConfig(t, "discovery-config-01"))
	msg.Header.SubKind = discoveryconfig.SubKindStaticSnapshot
	msg.Spec.Aws[0].Params = &types.InstallerParams{JoinToken: "token-name"}

	converted, err := FromProto(msg)
	require.NoError(t, err)
	require.Empty(t, converted.GetSubKind())
	require.Equal(t, "token-name", converted.Spec.AWS[0].Params.JoinToken,
		"the default user-input conversion keeps installer params regardless of claimed subkind")

	msg.Spec.DiscoveryGroup = ""
	_, err = FromProto(msg)
	require.True(t, trace.IsBadParameter(err), "got %v", err)

	_, err = FromProto(nil)
	require.True(t, trace.IsBadParameter(err), "got %v", err)
}

// TestFromProtoMissingSpec ensures conversion rejects a message without a
// spec instead of panicking on it.
func TestFromProtoMissingSpec(t *testing.T) {
	discoveryConfig := ToProto(newDiscoveryConfig(t, "discovery-config-01"))
	discoveryConfig.Spec = nil

	_, err := FromProto(discoveryConfig)
	require.Error(t, err)
}

func newDiscoveryConfig(t *testing.T, name string) *discoveryconfig.DiscoveryConfig {
	t.Helper()

	discoveryConfig, err := discoveryconfig.NewDiscoveryConfig(
		header.Metadata{
			Name: name,
		},
		discoveryconfig.Spec{
			DiscoveryGroup: "discovery-group-01",
			AWS: []types.AWSMatcher{
				{
					Types:   []string{"rds"},
					Regions: []string{"us-west-2"},
				},
				{
					Types:   []string{"ec2"},
					Regions: []string{"eu-west-2"},
				},
			},
		},
	)
	require.NoError(t, err)
	discoveryConfig.Status.State = discoveryconfigv1.DiscoveryConfigState_name[int32(discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_RUNNING)]
	discoveryConfig.Status.DiscoveredResources = 42
	now := time.Now()
	discoveryConfig.Status.LastSyncTime = now
	errMsg := "error message"
	discoveryConfig.Status.ErrorMessage = &errMsg
	discoveryConfig.Status.IntegrationDiscoveredResources = map[string]*discoveryconfig.IntegrationDiscoveredSummary{
		"my-integration": {
			IntegrationDiscoveredSummary: &discoveryconfigv1.IntegrationDiscoveredSummary{
				AwsEc2: &discoveryconfigv1.ResourcesDiscoveredSummary{
					Found:    4,
					Enrolled: 2,
					Failed:   1,
				},
			},
		},
	}
	return discoveryConfig
}
