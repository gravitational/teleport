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
			name: "error if both labels and tags are set with different values",
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
			name: "error if both labels and tags are set with equal keys but different values",
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
			name: "equal labels and tags are accepted and preserved (legacy stored shape)",
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
				Tags: Labels{
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
			name: "tags only is preserved and not copied into labels",
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
				Tags: Labels{
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
// Defaulting does not mutate the deprecated tags alias: it neither copies
// tags into labels nor clears either field, so a tags-only matcher stays
// tags-only and re-running defaulting on its own output changes nothing.
// This is what lets stored or republished matchers (for example static
// snapshot publication) be re-validated repeatedly without failing.
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
	// The spec is preserved exactly: tags kept, labels not synthesized.
	require.Equal(t, Labels{"env": []string{"prod"}}, once.Tags)
	require.Empty(t, once.Labels)
	// The alias still resolves through GetLabels for matching.
	require.Equal(t, Labels{"env": []string{"prod"}}, once.GetLabels())

	twice := newMatcher()
	require.NoError(t, twice.CheckAndSetDefaults())
	require.NoError(t, twice.CheckAndSetDefaults())
	require.Equal(t, once, twice)

	// A legacy stored shape (older versions copied tags into labels
	// without clearing tags) must validate, stay unmutated, and be a
	// fixed point across further runs.
	newLegacy := func() *GCPMatcher {
		return &GCPMatcher{
			Types:      []string{"gce"},
			ProjectIDs: []string{"project01"},
			Labels:     Labels{"env": []string{"prod"}},
			Tags:       Labels{"env": []string{"prod"}},
		}
	}
	legacyOnce := newLegacy()
	require.NoError(t, legacyOnce.CheckAndSetDefaults())
	require.Equal(t, Labels{"env": []string{"prod"}}, legacyOnce.Labels)
	require.Equal(t, Labels{"env": []string{"prod"}}, legacyOnce.Tags)

	legacyTwice := newLegacy()
	require.NoError(t, legacyTwice.CheckAndSetDefaults())
	require.NoError(t, legacyTwice.CheckAndSetDefaults())
	require.Equal(t, legacyOnce, legacyTwice)
}
