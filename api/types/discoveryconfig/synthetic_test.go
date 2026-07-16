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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig/internal/matchertest"
	"github.com/gravitational/teleport/api/types/header"
)

func TestNewSyntheticDiscoveryConfig(t *testing.T) {
	const serverID = "00000000-0000-0000-0000-000000000001"
	input := SyntheticStatus{DiscoveryGroup: "group", Matchers: &Spec{AWS: []types.AWSMatcher{{
		Types: []string{types.AWSMatcherEC2}, Regions: []string{"us-east-1"},
	}}}}
	dc, err := NewSyntheticDiscoveryConfig(serverID, input)
	require.NoError(t, err)
	require.True(t, dc.IsSynthetic())
	require.Equal(t, SyntheticName(serverID), dc.GetName())
	require.Equal(t, types.OriginConfigFile, dc.Origin())
	require.Empty(t, dc.Spec.DiscoveryGroup)
	require.Empty(t, dc.Spec.AWS)
	require.Empty(t, dc.Spec.Azure)
	require.Empty(t, dc.Spec.GCP)
	require.Empty(t, dc.Spec.Kube)
	require.Nil(t, dc.Spec.AccessGraph)
	require.Equal(t, "group", dc.ConfiguredDiscoveryGroup())
	require.Len(t, dc.Status.Synthetic.Matchers.AWS, 1)

	// The status is copied on construction: mutating the input afterward must
	// not reach the resource.
	input.Matchers.AWS[0].Regions[0] = "mutated"
	require.Equal(t, "us-east-1", dc.Status.Synthetic.Matchers.AWS[0].Regions[0])

	// The status must already satisfy the publication contract: unsanitized
	// installer params are rejected rather than silently stripped.
	_, err = NewSyntheticDiscoveryConfig(serverID, SyntheticStatus{Matchers: &Spec{AWS: []types.AWSMatcher{{
		Types: []string{types.AWSMatcherEC2}, Regions: []string{"us-east-1"},
		Params: &types.InstallerParams{JoinToken: "secret"},
	}}}})
	requireBadParameter(t, err)

	zero, err := NewSyntheticDiscoveryConfig(serverID, SyntheticStatus{Matchers: &Spec{}})
	require.NoError(t, err)
	require.NotNil(t, zero.Status.Synthetic.Matchers)
	require.Empty(t, *zero.Status.Synthetic.Matchers)

	_, err = NewSyntheticDiscoveryConfig("", SyntheticStatus{Matchers: &Spec{}})
	requireBadParameter(t, err)
}

func TestNewDiscoveryConfigWithSubKindDoesNotMutateSyntheticSpec(t *testing.T) {
	params := &types.InstallerParams{JoinToken: "secret"}
	input := Spec{AWS: []types.AWSMatcher{{
		Types:   []string{types.AWSMatcherEC2},
		Regions: []string{"us-east-1"},
		Params:  params,
	}}}

	dc, err := NewDiscoveryConfigWithSubKind(header.Metadata{Name: "synthetic"}, input, SubKindSynthetic)
	require.NoError(t, err)
	require.Nil(t, dc.Spec.AWS[0].Params)
	require.Same(t, params, input.AWS[0].Params)
	require.Equal(t, "secret", input.AWS[0].Params.JoinToken)
}

// TestEachInstallerParamsCoversAllFamilies enforces the eachInstallerParams
// contract by reflection: every Spec matcher family whose elements carry
// installer params must be visited, so synthetic sanitization and validation
// cannot silently miss a family added later.
func TestEachInstallerParamsCoversAllFamilies(t *testing.T) {
	var spec Spec
	want := matchertest.PopulateSentinelInstallerParams(&spec)
	require.NotZero(t, want, "expected at least one matcher family with installer params")

	visited := 0
	spec.eachInstallerParams(func(p **types.InstallerParams) {
		if *p != nil && (*p).JoinToken == matchertest.SentinelJoinToken {
			visited++
		}
	})
	require.Equal(t, want, visited,
		"eachInstallerParams must visit every matcher family carrying installer params; update it (and convert/v1.SanitizeSyntheticDiscoveryConfig) for the new family")

	SanitizeSyntheticDiscoveryConfigSpec(&spec)
	require.Empty(t, matchertest.FamiliesWithInstallerParams(&spec), "sanitization left installer params behind")
}

func TestSyntheticNames(t *testing.T) {
	uuid := "00000000-0000-0000-0000-000000000001"
	require.Equal(t, "synthetic-"+uuid, SyntheticName(uuid))
	require.True(t, IsReservedSyntheticName(SyntheticName(uuid)))
	hashed := SyntheticName("legacy-server")
	require.True(t, strings.HasPrefix(hashed, syntheticHashedNamePrefix))
	require.Equal(t, hashed, SyntheticName("legacy-server"))
	require.True(t, IsReservedSyntheticName(hashed))
	require.False(t, IsReservedSyntheticName("synthetic-aws-prod"))
}

func TestCheckSyntheticDiscoveryConfigRepresentations(t *testing.T) {
	const serverID = "00000000-0000-0000-0000-000000000001"
	complete, err := NewSyntheticDiscoveryConfig(serverID, SyntheticStatus{Matchers: &Spec{}})
	require.NoError(t, err)
	require.NoError(t, CheckSyntheticDiscoveryConfig(complete, serverID))

	truncated := complete.Clone()
	truncated.Status.Synthetic.Matchers = nil
	truncated.Status.Synthetic.MatchersTruncated = true
	truncated.Status.Synthetic.MatcherCounts = &StaticMatcherCounts{AWS: 2, Azure: 1}
	require.NoError(t, CheckSyntheticDiscoveryConfig(truncated, serverID))

	for name, mutate := range map[string]func(*DiscoveryConfig){
		"nil inventory":          func(dc *DiscoveryConfig) { dc.Status.Synthetic = nil },
		"both representations":   func(dc *DiscoveryConfig) { dc.Status.Synthetic.MatcherCounts = &StaticMatcherCounts{} },
		"neither representation": func(dc *DiscoveryConfig) { dc.Status.Synthetic.Matchers = nil },
		"nested discovery group": func(dc *DiscoveryConfig) { dc.Status.Synthetic.Matchers.DiscoveryGroup = "bad" },
		"installer params": func(dc *DiscoveryConfig) {
			dc.Status.Synthetic.Matchers.AWS = []types.AWSMatcher{{Params: &types.InstallerParams{JoinToken: "secret"}}}
		},
	} {
		t.Run(name, func(t *testing.T) {
			dc := complete.Clone()
			mutate(dc)
			requireBadParameter(t, CheckSyntheticDiscoveryConfig(dc, serverID))
		})
	}
}

func TestConfiguredDiscoveryGroupRegular(t *testing.T) {
	dc, err := NewDiscoveryConfig(header.Metadata{Name: "regular"}, Spec{DiscoveryGroup: "regular-group"})
	require.NoError(t, err)
	require.Equal(t, "regular-group", dc.ConfiguredDiscoveryGroup())
}
