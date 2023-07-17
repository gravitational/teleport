// Copyright 2022 Gravitational, Inc
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

package utils

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestSliceMatchesRegex(t *testing.T) {
	for _, test := range []struct {
		input string
		exprs []string

		matches bool
		assert  require.ErrorAssertionFunc
	}{
		{
			input:   "test|staging",
			exprs:   []string{"test|staging"}, // treated as a literal string
			matches: true,
			assert:  require.NoError,
		},
		{
			input:   "test",
			exprs:   []string{"^test|staging$"}, // treated as a regular expression due to ^ $
			matches: true,
			assert:  require.NoError,
		},
		{
			input:   "staging",
			exprs:   []string{"^test|staging$"}, // treated as a regular expression due to ^ $
			matches: true,
			assert:  require.NoError,
		},
		{
			input:   "test-foo",
			exprs:   []string{"test-*"}, // treated as a glob pattern due to missing ^ $
			matches: true,
			assert:  require.NoError,
		},
		{
			input:   "foo-test",
			exprs:   []string{"test-*"}, // treated as a glob pattern due to missing ^ $
			matches: false,
			assert:  require.NoError,
		},
		{
			input:   "foo",
			exprs:   []string{"^[$"}, // invalid regex, should error
			matches: false,
			assert:  require.Error,
		},
	} {
		t.Run(test.input, func(t *testing.T) {
			matches, err := SliceMatchesRegex(test.input, test.exprs)
			test.assert(t, err)
			require.Equal(t, test.matches, matches)
		})
	}
}

func TestRegexMatchesAny(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		desc        string
		inputs      []string
		expr        string
		expectError string
		expectMatch bool
	}{
		{
			desc:        "empty",
			expectMatch: false,
		},
		{
			desc:        "exact match",
			expr:        "test",
			inputs:      []string{"test"},
			expectMatch: true,
		},
		{
			desc:        "no exact match",
			expr:        "test",
			inputs:      []string{"first", "last"},
			expectMatch: false,
		},
		{
			desc:        "must match full string",
			expr:        "test",
			inputs:      []string{"pretest", "tempest", "testpost"},
			expectMatch: false,
		},
		{
			desc:        "glob match",
			expr:        "env-*-staging",
			inputs:      []string{"env-app-staging"},
			expectMatch: true,
		},
		{
			desc:        "glob must match full string",
			expr:        "env-*-staging",
			inputs:      []string{"pre-env-app-staging", "env-app-staging-post"},
			expectMatch: false,
		},
		{
			desc:        "regexp match",
			expr:        "^env-[a-zA-Z0-9]{3,12}-staging$",
			inputs:      []string{"env-app-staging"},
			expectMatch: true,
		},
		{
			desc:        "regexp no match",
			expr:        "^env-[a-zA-Z0-9]{3,12}-staging$",
			inputs:      []string{"env-~-staging", "env-🚀-staging", "env-reallylongname-staging"},
			expectMatch: false,
		},
		{
			desc:        "regexp must match full string",
			expr:        "^env-[a-zA-Z0-9]{3,12}-staging$",
			inputs:      []string{"pre-env-app-staging", "env-app-staging-post"},
			expectMatch: false,
		},
		{
			desc:        "bad regexp",
			expr:        "^env-(?!prod)$",
			expectError: "error parsing regexp",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			match, err := RegexMatchesAny(tc.inputs, tc.expr)
			if msg := tc.expectError; msg != "" {
				require.ErrorContains(t, err, msg)
				return
			}
			require.Equal(t, tc.expectMatch, match)
		})
	}
}

func TestKubeResourceMatchesRegex(t *testing.T) {
	tests := []struct {
		name      string
		input     types.KubernetesResource
		resources []types.KubernetesResource
		matches   bool
		assert    require.ErrorAssertionFunc
	}{
		{
			name: "input misses verb",
			input: types.KubernetesResource{
				Kind:      types.KindKubePod,
				Namespace: "default",
				Name:      "podname",
			},
			resources: []types.KubernetesResource{
				{
					Kind:      types.KindKubePod,
					Namespace: "default",
					Name:      "podname",
				},
			},
			assert:  require.Error,
			matches: false,
		},
		{
			name: "input matches single resource with wildcard verb",
			input: types.KubernetesResource{
				Kind:      types.KindKubePod,
				Namespace: "default",
				Name:      "podname",
				Verbs:     []string{types.KubeVerbGet},
			},
			resources: []types.KubernetesResource{
				{
					Kind:      types.KindKubePod,
					Namespace: "default",
					Name:      "podname",
					Verbs:     []string{types.Wildcard},
				},
			},
			assert:  require.NoError,
			matches: true,
		},
		{
			name: "input matches single resource with matching verb",
			input: types.KubernetesResource{
				Kind:      types.KindKubePod,
				Namespace: "default",
				Name:      "podname",
				Verbs:     []string{types.KubeVerbGet},
			},
			resources: []types.KubernetesResource{
				{
					Kind:      types.KindKubePod,
					Namespace: "default",
					Name:      "podname",
					Verbs:     []string{types.KubeVerbCreate, types.KubeVerbGet},
				},
			},
			assert:  require.NoError,
			matches: true,
		},
		{
			name: "input matches single resource with unmatching verb",
			input: types.KubernetesResource{
				Kind:      types.KindKubePod,
				Namespace: "default",
				Name:      "podname",
				Verbs:     []string{types.KubeVerbPatch},
			},
			resources: []types.KubernetesResource{
				{
					Kind:      types.KindKubePod,
					Namespace: "default",
					Name:      "podname",
					Verbs:     []string{types.KubeVerbGet, types.KubeVerbGet},
				},
			},
			assert:  require.NoError,
			matches: false,
		},
		{
			name: "input does not match single resource because missing verb",
			input: types.KubernetesResource{
				Kind:      types.KindKubePod,
				Namespace: "default",
				Name:      "podname",
				Verbs:     []string{types.KubeVerbGet},
			},
			resources: []types.KubernetesResource{
				{
					Kind:      types.KindKubePod,
					Namespace: "default",
					Name:      "podname",
				},
			},
			assert:  require.NoError,
			matches: false,
		},
		{
			name: "input matches last resource",
			input: types.KubernetesResource{
				Kind:      types.KindKubePod,
				Namespace: "default",
				Name:      "podname",
				Verbs:     []string{types.KubeVerbGet},
			},
			resources: []types.KubernetesResource{
				{
					Kind:      types.KindKubePod,
					Namespace: "other_namespace",
					Name:      "podname",
					Verbs:     []string{types.Wildcard},
				},
				{
					Kind:      types.KindKubePod,
					Namespace: "default",
					Name:      "other_pod",
					Verbs:     []string{types.Wildcard},
				},
				{
					Kind:      types.KindKubePod,
					Namespace: "default",
					Name:      "podname",
					Verbs:     []string{types.Wildcard},
				},
			},
			assert:  require.NoError,
			matches: true,
		},
		{
			name: "input matches regex expression",
			input: types.KubernetesResource{
				Kind:      types.KindKubePod,
				Namespace: "default-5",
				Name:      "podname-5",
				Verbs:     []string{types.KubeVerbGet},
			},
			resources: []types.KubernetesResource{
				{
					Kind:      types.KindKubePod,
					Namespace: "defa*",
					Name:      "^podname-[0-9]+$",
					Verbs:     []string{types.Wildcard},
				},
			},
			assert:  require.NoError,
			matches: true,
		},
		{
			name: "input has no matchers",
			input: types.KubernetesResource{
				Kind:      types.KindKubePod,
				Namespace: "default",
				Name:      "pod-name",
				Verbs:     []string{types.KubeVerbGet},
			},
			resources: []types.KubernetesResource{
				{
					Kind:      types.KindKubePod,
					Namespace: "default",
					Name:      "^pod-[0-9]+$",
					Verbs:     []string{types.Wildcard},
				},
			},
			assert:  require.NoError,
			matches: false,
		},
		{
			name: "invalid regex expression",
			input: types.KubernetesResource{
				Kind:      types.KindKubePod,
				Namespace: "default-5",
				Name:      "podname-5",
				Verbs:     []string{types.KubeVerbGet},
			},
			resources: []types.KubernetesResource{
				{
					Kind:      types.KindKubePod,
					Namespace: "defa*",
					Name:      "^podname-[0-+$",
					Verbs:     []string{types.Wildcard},
				},
			},
			assert: require.Error,
		},
		{
			name: "resource with different kind",
			input: types.KubernetesResource{
				Kind:      types.KindKubePod,
				Namespace: "default",
				Name:      "podname",
				Verbs:     []string{types.KubeVerbGet},
			},
			resources: []types.KubernetesResource{
				{
					Kind:      "other_type",
					Namespace: "default",
					Name:      "podname",
				},
			},
			assert: require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := KubeResourceMatchesRegex(tt.input, tt.resources)
			tt.assert(t, err)
			require.Equal(t, tt.matches, got)
		})
	}
}
