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

func TestKubeMatcherCheckAndSetDefaults(t *testing.T) {
	isBadParameterErr := func(tt require.TestingT, err error, i ...any) {
		require.True(tt, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
	}

	for _, tt := range []struct {
		name     string
		in       *KubernetesMatcher
		errCheck require.ErrorAssertionFunc
		expected *KubernetesMatcher
	}{
		{
			name: "valid",
			in: &KubernetesMatcher{
				Types:      []string{"app"},
				Namespaces: []string{"default"},
				Labels: Labels{
					"x": []string{"y"},
				},
			},
			errCheck: require.NoError,
			expected: &KubernetesMatcher{
				Types:      []string{"app"},
				Namespaces: []string{"default"},
				Labels: Labels{
					"x": []string{"y"},
				},
			},
		},
		{
			name:     "default values",
			in:       &KubernetesMatcher{},
			errCheck: require.NoError,
			expected: &KubernetesMatcher{
				Types:      []string{"app"},
				Namespaces: []string{"*"},
				Labels: Labels{
					"*": []string{"*"},
				},
			},
		},
		{
			name: "wildcard is invalid for types",
			in: &KubernetesMatcher{
				Types: []string{"*"},
			},
			errCheck: isBadParameterErr,
		},
		{
			name: "invalid type",
			in: &KubernetesMatcher{
				Types: []string{"db"},
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
