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

func TestAzureMatcherCheckAndSetDefaults(t *testing.T) {
	isBadParameterErr := func(tt require.TestingT, err error, i ...any) {
		require.True(tt, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
	}

	for _, tt := range []struct {
		name     string
		in       *AzureMatcher
		errCheck require.ErrorAssertionFunc
		expected *AzureMatcher
	}{
		{
			name: "valid",
			in: &AzureMatcher{
				Types:          []string{"vm"},
				Regions:        []string{"westeurope"},
				Subscriptions:  []string{"s1", "s2"},
				ResourceGroups: []string{"rg1"},
				ResourceTags: Labels{
					"x": []string{"y"},
				},
				Params: &InstallerParams{
					JoinMethod: JoinMethodAzure,
					JoinToken:  AzureInviteTokenName,
					ScriptName: DefaultInstallerScriptName,
				},
			},
			errCheck: require.NoError,
			expected: &AzureMatcher{
				Types:          []string{"vm"},
				Regions:        []string{"westeurope"},
				Subscriptions:  []string{"s1", "s2"},
				ResourceGroups: []string{"rg1"},
				ResourceTags: Labels{
					"x": []string{"y"},
				},
				Params: &InstallerParams{
					JoinMethod: "azure",
					JoinToken:  "azure-discovery-token",
					ScriptName: "default-installer",
					Azure:      &AzureInstallerParams{},
				},
			},
		},
		{
			name: "default values",
			in: &AzureMatcher{
				Types: []string{"vm"},
			},
			errCheck: require.NoError,
			expected: &AzureMatcher{
				Types:          []string{"vm"},
				Regions:        []string{"*"},
				Subscriptions:  []string{"*"},
				ResourceGroups: []string{"*"},
				ResourceTags: Labels{
					"*": []string{"*"},
				},
				Params: &InstallerParams{
					JoinMethod: "azure",
					JoinToken:  "azure-discovery-token",
					ScriptName: "default-installer",
					Azure:      &AzureInstallerParams{},
				},
			},
		},
		{
			name: "wildcard is invalid for types",
			in: &AzureMatcher{
				Types:   []string{"*"},
				Regions: []string{"eu-west-2"},
			},
			errCheck: isBadParameterErr,
		},
		{
			name: "invalid type",
			in: &AzureMatcher{
				Types: []string{"ec2"},
			},
			errCheck: isBadParameterErr,
		},
		{
			name: "no type",
			in: &AzureMatcher{
				Types: []string{},
			},
			errCheck: isBadParameterErr,
		},
		{
			name: "invalid join method",
			in: &AzureMatcher{
				Types: []string{"vm"},
				Params: &InstallerParams{
					JoinMethod: "invalid",
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
