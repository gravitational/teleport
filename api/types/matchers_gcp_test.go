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
			name: "error if both labels and tags are set",
			in: &GCPMatcher{
				Types:      []string{"gce"},
				ProjectIDs: []string{"project001"},
				Labels: Labels{
					"*": []string{"*"},
				},
				Tags: Labels{
					"*": []string{"*"},
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
