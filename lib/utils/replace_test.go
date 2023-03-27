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

func TestKubeResourceMatchesRegex(t *testing.T) {
	tests := []struct {
		name      string
		input     types.KubernetesResource
		resources []types.KubernetesResource
		matches   bool
		assert    require.ErrorAssertionFunc
	}{
		{
			name: "input matches single resource",
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
			assert:  require.NoError,
			matches: true,
		},
		{
			name: "input matches last resource",
			input: types.KubernetesResource{
				Kind:      types.KindKubePod,
				Namespace: "default",
				Name:      "podname",
			},
			resources: []types.KubernetesResource{
				{
					Kind:      types.KindKubePod,
					Namespace: "other_namespace",
					Name:      "podname",
				},
				{
					Kind:      types.KindKubePod,
					Namespace: "default",
					Name:      "other_pod",
				},
				{
					Kind:      types.KindKubePod,
					Namespace: "default",
					Name:      "podname",
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
			},
			resources: []types.KubernetesResource{
				{
					Kind:      types.KindKubePod,
					Namespace: "defa*",
					Name:      "^podname-[0-9]+$",
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
			},
			resources: []types.KubernetesResource{
				{
					Kind:      types.KindKubePod,
					Namespace: "default",
					Name:      "^pod-[0-9]+$",
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
			},
			resources: []types.KubernetesResource{
				{
					Kind:      types.KindKubePod,
					Namespace: "defa*",
					Name:      "^podname-[0-+$",
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
