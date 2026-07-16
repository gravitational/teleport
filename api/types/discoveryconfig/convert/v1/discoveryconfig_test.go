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
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/types/discoveryconfig/internal/matchertest"
	"github.com/gravitational/teleport/api/types/header"
)

func TestRoundtrip(t *testing.T) {
	discoveryConfig := newDiscoveryConfig(t, "discovery-config-01")

	converted, err := FromProto(ToProto(discoveryConfig))
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(discoveryConfig, converted))
}

// TestRoundtripSubKind ensures the SubKind survives the proto conversion in
// both directions. Synthetic discovery configs rely on it: the subkind is what
// marks them for authorization and consumption filtering.
func TestRoundtripSubKind(t *testing.T) {
	discoveryConfig, err := discoveryconfig.NewSyntheticDiscoveryConfig("server-01", discoveryconfig.SyntheticStatus{
		DiscoveryGroup: "discovery-group-01",
		Matchers: &discoveryconfig.Spec{AWS: []types.AWSMatcher{{
			Types:   []string{types.AWSMatcherEC2},
			Regions: []string{"eu-west-2"},
		}}},
	})
	require.NoError(t, err)
	discoveryConfig.SetExpiry(time.Now().UTC().Truncate(time.Second).Add(time.Hour))

	converted, err := FromProto(ToProto(discoveryConfig))
	require.NoError(t, err)

	require.True(t, converted.IsSynthetic())
	require.Equal(t, discoveryConfig.GetName(), converted.GetName())
	require.Empty(t, converted.Spec.DiscoveryGroup)
	require.Empty(t, converted.Spec.AWS)
	require.Empty(t, converted.Spec.Azure)
	require.Empty(t, converted.Spec.GCP)
	require.Empty(t, converted.Spec.Kube)
	require.Nil(t, converted.Spec.AccessGraph)
	require.Equal(t, discoveryConfig.ConfiguredDiscoveryGroup(), converted.ConfiguredDiscoveryGroup())
	require.Equal(t, discoveryConfig.Status.Synthetic.Matchers, converted.Status.Synthetic.Matchers)
	require.Nil(t, converted.Status.Synthetic.Matchers.AWS[0].Params)
}

// TestRoundtripSyntheticTruncatedCounts verifies that conversion preserves the
// aggregate matcher inventory when detailed matchers have been truncated.
func TestRoundtripSyntheticTruncatedCounts(t *testing.T) {
	discoveryConfig, err := discoveryconfig.NewSyntheticDiscoveryConfig("server-01", discoveryconfig.SyntheticStatus{
		DiscoveryGroup:    "discovery-group-01",
		MatchersTruncated: true,
		MatcherCounts:     &discoveryconfig.StaticMatcherCounts{AWS: 3, Azure: 2, GCP: 1, Kube: 4, AccessGraph: 5},
	})
	require.NoError(t, err)

	converted, err := FromProto(ToProto(discoveryConfig))
	require.NoError(t, err)

	require.Equal(t, discoveryConfig.Status.Synthetic, converted.Status.Synthetic)
}

func TestFromProtoSanitizesSyntheticBeforeValidation(t *testing.T) {
	discoveryConfig, err := discoveryconfig.NewSyntheticDiscoveryConfig("server-01", discoveryconfig.SyntheticStatus{
		Matchers: &discoveryconfig.Spec{AWS: []types.AWSMatcher{{
			Types:   []string{types.AWSMatcherEC2},
			Regions: []string{"eu-west-2"},
		}}},
	})
	require.NoError(t, err)
	msg := ToProto(discoveryConfig)
	msg.Status.Synthetic.Matchers.Aws[0].Params = &types.InstallerParams{
		EnrollMode: types.InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_SCRIPT,
		JoinToken:  "secret-token",
		HTTPProxySettings: &types.HTTPProxySettings{
			HTTPSProxy: "not-a-valid-proxy-with-secret",
		},
	}

	converted, err := FromProto(msg)
	require.NoError(t, err)
	// InstallerParams is excluded in its entirety from synthetic inventory,
	// including non-secret fields such as EnrollMode.
	require.Nil(t, converted.Status.Synthetic.Matchers.AWS[0].Params)
}

func TestSanitizeSyntheticDiscoveryConfig(t *testing.T) {
	dc, err := discoveryconfig.NewSyntheticDiscoveryConfig("server-01", discoveryconfig.SyntheticStatus{
		Matchers: &discoveryconfig.Spec{
			AWS:   []types.AWSMatcher{{Types: []string{types.AWSMatcherEC2}, Regions: []string{"us-east-1"}}},
			Azure: []types.AzureMatcher{{Types: []string{types.AzureMatcherVM}, Regions: []string{"eastus"}}},
			GCP:   []types.GCPMatcher{{Types: []string{types.GCPMatcherCompute}, Locations: []string{"us-central1-a"}, ProjectIDs: []string{"project"}}},
		},
	})
	require.NoError(t, err)
	msg := ToProto(dc)
	msg.Status.Synthetic.Matchers.Aws[0].Params = &types.InstallerParams{JoinToken: "secret"}
	for _, params := range []*types.InstallerParams{
		msg.Status.Synthetic.Matchers.Aws[0].Params,
		msg.Status.Synthetic.Matchers.Azure[0].Params,
		msg.Status.Synthetic.Matchers.Gcp[0].Params,
	} {
		if params == nil {
			continue
		}
		params.JoinToken = "secret"
	}
	if msg.Status.Synthetic.Matchers.Azure[0].Params == nil {
		msg.Status.Synthetic.Matchers.Azure[0].Params = &types.InstallerParams{JoinToken: "secret"}
	}
	if msg.Status.Synthetic.Matchers.Gcp[0].Params == nil {
		msg.Status.Synthetic.Matchers.Gcp[0].Params = &types.InstallerParams{JoinToken: "secret"}
	}

	sanitized := SanitizeSyntheticDiscoveryConfig(msg)
	require.False(t, proto.Equal(msg, sanitized))
	require.Equal(t, "secret", msg.Status.Synthetic.Matchers.Aws[0].Params.JoinToken, "sanitization must not mutate its input")
	require.Nil(t, sanitized.Status.Synthetic.Matchers.Aws[0].Params)
	require.Nil(t, sanitized.Status.Synthetic.Matchers.Azure[0].Params)
	require.Nil(t, sanitized.Status.Synthetic.Matchers.Gcp[0].Params)
}

// TestFromProtoPreservesUnknownSubKind ensures conversion preserves unknown
// subkinds without weakening regular DiscoveryConfig validation.
func TestFromProtoPreservesUnknownSubKind(t *testing.T) {
	msg := ToProto(newDiscoveryConfig(t, "discovery-config-01"))
	msg.Header.SubKind = "some-future-subkind"

	converted, err := FromProto(msg)
	require.NoError(t, err)
	require.Equal(t, "some-future-subkind", converted.GetSubKind())
	require.Equal(t, "discovery-group-01", converted.Spec.DiscoveryGroup)

	msg.Spec.DiscoveryGroup = ""
	_, err = FromProto(msg)
	require.True(t, trace.IsBadParameter(err), "got %v", err)
}

// TestSanitizeSyntheticDiscoveryConfigCoversAllFamilies enforces by reflection
// that the proto-side sanitizer strips installer params from every matcher
// family the native Spec carries them in, so the family list cannot silently
// drift from Spec.eachInstallerParams when a new family is added.
func TestSanitizeSyntheticDiscoveryConfigCoversAllFamilies(t *testing.T) {
	var spec discoveryconfig.Spec
	want := matchertest.PopulateSentinelInstallerParams(&spec)
	require.NotZero(t, want, "expected at least one matcher family with installer params")
	msg := &discoveryconfigv1.DiscoveryConfig{
		Header: &headerv1.ResourceHeader{SubKind: discoveryconfig.SubKindSynthetic},
		Spec:   &discoveryconfigv1.DiscoveryConfigSpec{},
		Status: StatusToProto(discoveryconfig.Status{Synthetic: &discoveryconfig.SyntheticStatus{Matchers: &spec}}),
	}
	require.Len(t, matchertest.FamiliesWithInstallerParams(StatusFromProto(msg.GetStatus()).Synthetic.Matchers), want,
		"fixture must carry one sentinel per family before sanitization")

	sanitized := SanitizeSyntheticDiscoveryConfig(msg)
	got := StatusFromProto(sanitized.GetStatus()).Synthetic.Matchers
	require.Empty(t, matchertest.FamiliesWithInstallerParams(got),
		"SanitizeSyntheticDiscoveryConfig missed a matcher family; update it alongside Spec.eachInstallerParams")
}

// Make sure that we don't panic if any of the message fields are missing.
func TestFromProtoNils(t *testing.T) {
	// Spec is nil
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
