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

package types

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestGCPMatcherCheckAndSetDefaults(t *testing.T) {
	isBadParameterErr := func(tt require.TestingT, err error, i ...any) {
		require.True(tt, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
	}

	for _, tt := range []struct {
		name     string
		in       *GCPMatcher
		errCheck require.ErrorAssertionFunc
		expected *GCPMatcher
	}{
		{
			name: "valid",
			in: &GCPMatcher{
				Types:      []string{"gce"},
				Locations:  []string{"europe-west2"},
				ProjectIDs: []string{"project01"},
				Labels: Labels{
					"x": []string{"y"},
				},
				Params: &InstallerParams{
					JoinMethod: JoinMethodGCP,
					JoinToken:  GCPInviteTokenName,
					ScriptName: DefaultInstallerScriptName,
				},
			},
			errCheck: require.NoError,
			expected: &GCPMatcher{
				Types:      []string{"gce"},
				Locations:  []string{"europe-west2"},
				ProjectIDs: []string{"project01"},
				Labels: Labels{
					"x": []string{"y"},
				},
				Params: &InstallerParams{
					JoinMethod: "gcp",
					JoinToken:  "gcp-discovery-token",
					ScriptName: "default-installer",
				},
			},
		},
		{
			name: "default values",
			in: &GCPMatcher{
				Types:      []string{"gce"},
				ProjectIDs: []string{"project01"},
			},
			errCheck: require.NoError,
			expected: &GCPMatcher{
				Types:      []string{"gce"},
				Locations:  []string{"*"},
				ProjectIDs: []string{"project01"},
				Labels: Labels{
					"*": []string{"*"},
				},
				Params: &InstallerParams{
					JoinMethod: "gcp",
					JoinToken:  "gcp-discovery-token",
					ScriptName: "default-installer",
				},
			},
		},
		{
			name: "wildcard is invalid for types",
			in: &GCPMatcher{
				Types:      []string{"*"},
				ProjectIDs: []string{"project01"},
			},
			errCheck: isBadParameterErr,
		},
		{
			name: "wildcard is valid for project ids",
			in: &GCPMatcher{
				Types:      []string{"gce"},
				ProjectIDs: []string{"*"},
			},
			errCheck: require.NoError,
		},
		{
			name: "invalid type",
			in: &GCPMatcher{
				Types:      []string{"invalid"},
				ProjectIDs: []string{"project01"},
			},
			errCheck: isBadParameterErr,
		},
		{
			name: "invalid join method",
			in: &GCPMatcher{
				Types:      []string{"gce"},
				ProjectIDs: []string{"project01"},
				Params: &InstallerParams{
					JoinMethod: "invalid",
				},
			},
			errCheck: isBadParameterErr,
		},
		{
			name: "no type",
			in: &GCPMatcher{
				Types:      []string{},
				ProjectIDs: []string{"project01"},
			},
			errCheck: isBadParameterErr,
		},
		{
			name: "no project id",
			in: &GCPMatcher{
				Types:      []string{"gce"},
				ProjectIDs: []string{},
			},
			errCheck: isBadParameterErr,
		},
		{
			name: "error if labels and tags are set with different values",
			in: &GCPMatcher{
				Types:      []string{"gce"},
				ProjectIDs: []string{"project001"},
				Labels: Labels{
					"env": []string{"prod"},
				},
				Tags: Labels{
					"env": []string{"dev"},
				},
			},
			errCheck: isBadParameterErr,
		},
		{
			name: "labels and tags with equal values in different order are rejected",
			in: &GCPMatcher{
				Types:      []string{"gce"},
				ProjectIDs: []string{"project001"},
				Labels: Labels{
					"env": []string{"prod", "dev"},
				},
				Tags: Labels{
					"env": []string{"dev", "prod"},
				},
			},
			errCheck: isBadParameterErr,
		},
		{
			name: "labels and tags with equal content are tolerated and normalized",
			in: &GCPMatcher{
				Types:      []string{"gce"},
				ProjectIDs: []string{"project001"},
				Labels: Labels{
					"env": []string{"prod"},
				},
				Tags: Labels{
					"env": []string{"prod"},
				},
			},
			errCheck: require.NoError,
			expected: &GCPMatcher{
				Types:      []string{"gce"},
				Locations:  []string{"*"},
				ProjectIDs: []string{"project001"},
				Labels: Labels{
					"env": []string{"prod"},
				},
				Params: &InstallerParams{
					JoinMethod: "gcp",
					JoinToken:  "gcp-discovery-token",
					ScriptName: "default-installer",
				},
			},
		},
		{
			name: "empty tags map is cleared and labels default",
			in: &GCPMatcher{
				Types:      []string{"gce"},
				ProjectIDs: []string{"project001"},
				Tags:       Labels{},
			},
			errCheck: require.NoError,
			expected: &GCPMatcher{
				Types:      []string{"gce"},
				Locations:  []string{"*"},
				ProjectIDs: []string{"project001"},
				Labels: Labels{
					"*": []string{"*"},
				},
				Params: &InstallerParams{
					JoinMethod: "gcp",
					JoinToken:  "gcp-discovery-token",
					ScriptName: "default-installer",
				},
			},
		},
		{
			name: "empty tags map is cleared and existing labels kept",
			in: &GCPMatcher{
				Types:      []string{"gce"},
				ProjectIDs: []string{"project001"},
				Labels: Labels{
					"env": []string{"prod"},
				},
				Tags: Labels{},
			},
			errCheck: require.NoError,
			expected: &GCPMatcher{
				Types:      []string{"gce"},
				Locations:  []string{"*"},
				ProjectIDs: []string{"project001"},
				Labels: Labels{
					"env": []string{"prod"},
				},
				Params: &InstallerParams{
					JoinMethod: "gcp",
					JoinToken:  "gcp-discovery-token",
					ScriptName: "default-installer",
				},
			},
		},
		{
			name: "tags only is normalized into labels",
			in: &GCPMatcher{
				Types:      []string{"gce"},
				ProjectIDs: []string{"project001"},
				Tags: Labels{
					"env": []string{"prod"},
				},
			},
			errCheck: require.NoError,
			expected: &GCPMatcher{
				Types:      []string{"gce"},
				Locations:  []string{"*"},
				ProjectIDs: []string{"project001"},
				Labels: Labels{
					"env": []string{"prod"},
				},
				Params: &InstallerParams{
					JoinMethod: "gcp",
					JoinToken:  "gcp-discovery-token",
					ScriptName: "default-installer",
				},
			},
		},
		{
			name: "invalid install suffix",
			in: &GCPMatcher{
				Types:      []string{"gce"},
				ProjectIDs: []string{"project001"},
				Params: &InstallerParams{
					Suffix: "$SHELL",
				},
			},
			errCheck: isBadParameterErr,
		},
		{
			name: "invalid update groups",
			in: &GCPMatcher{
				Types:      []string{"gce"},
				ProjectIDs: []string{"project001"},
				Params: &InstallerParams{
					UpdateGroup: "$SHELL",
				},
			},
			errCheck: isBadParameterErr,
		},
		{
			name: "invalid proxy settings",
			in: &GCPMatcher{
				Types:      []string{"gce"},
				ProjectIDs: []string{"project001"},
				Params: &InstallerParams{
					HTTPProxySettings: &HTTPProxySettings{
						HTTPProxy: "not a valid url",
					},
				},
			},
			errCheck: isBadParameterErr,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.in.CheckAndSetDefaults()
			tt.errCheck(t, err)
			if tt.expected != nil {
				require.Equal(t, tt.expected, tt.in)
			}
		})
	}
}

// TestGCPMatcherCheckAndSetDefaultsIdempotent verifies that re-running
// defaulting on an already-defaulted matcher succeeds and changes nothing.
// Defaulting normalizes the deprecated tags alias into labels and clears
// tags, so its own output must validate cleanly on every later run: stored
// or republished matchers (for example static snapshot publication) run
// defaulting again on its own prior output. The both-fields-equal shape
// written by older releases and older clients must normalize to the same
// result instead of being rejected.
func TestGCPMatcherCheckAndSetDefaultsIdempotent(t *testing.T) {
	t.Parallel()

	newMatcher := func() *GCPMatcher {
		return &GCPMatcher{
			Types:      []string{"gce"},
			ProjectIDs: []string{"project01"},
			Tags: Labels{
				"env": []string{"prod"},
			},
		}
	}

	once := newMatcher()
	require.NoError(t, once.CheckAndSetDefaults())
	require.Equal(t, Labels{"env": []string{"prod"}}, once.Labels)
	require.Nil(t, once.Tags)

	twice := newMatcher()
	require.NoError(t, twice.CheckAndSetDefaults())
	require.NoError(t, twice.CheckAndSetDefaults())
	require.Equal(t, once, twice)

	// The shape produced by older releases and by older clients during a
	// rolling upgrade: both fields populated with equal content. It must
	// be accepted and normalize to the same result as a fresh tags-only
	// matcher.
	legacy := &GCPMatcher{
		Types:      []string{"gce"},
		ProjectIDs: []string{"project01"},
		Labels: Labels{
			"env": []string{"prod"},
		},
		Tags: Labels{
			"env": []string{"prod"},
		},
	}
	require.NoError(t, legacy.CheckAndSetDefaults())
	require.Equal(t, once, legacy)

	// The normalized legacy matcher is itself a fixed point:
	// a further run doesn't change anything.
	require.NoError(t, legacy.CheckAndSetDefaults())
	require.Equal(t, once, legacy)
}
