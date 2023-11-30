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

package accesslist

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInclusionMarshalling(t *testing.T) {
	testCases := []struct {
		in       Inclusion
		require  require.ErrorAssertionFunc
		expected []byte
	}{
		{
			in:       InclusionUnspecified,
			require:  require.NoError,
			expected: []byte(`""`),
		},
		{
			in:       InclusionExplicit,
			require:  require.NoError,
			expected: []byte(`"explicit"`),
		},
		{
			in:       InclusionImplicit,
			require:  require.NoError,
			expected: []byte(`"implicit"`),
		},
		{
			in:       Inclusion(42),
			require:  require.Error,
			expected: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.in.String(), func(t *testing.T) {
			data, err := json.Marshal(testCase.in)
			testCase.require(t, err)
			require.Equal(t, testCase.expected, data)
		})
	}
}

func TestInclusionUnMarshalling(t *testing.T) {
	testCases := []struct {
		in       []byte
		require  require.ErrorAssertionFunc
		expected Inclusion
	}{
		{
			in:       []byte(`""`),
			expected: InclusionUnspecified,
			require:  require.NoError,
		},
		{
			in:       []byte(`"explicit"`),
			expected: InclusionExplicit,
			require:  require.NoError,
		},
		{
			in:       []byte(`"implicit"`),
			expected: InclusionImplicit,
			require:  require.NoError,
		},
		{
			in:       []byte(`"potato"`),
			expected: InclusionUnspecified,
			require:  require.Error,
		},
		{
			in:       []byte(`42`),
			expected: InclusionUnspecified,
			require:  require.Error,
		},
	}

	for _, testCase := range testCases {
		t.Run(string(testCase.in), func(t *testing.T) {
			var out Inclusion
			err := json.Unmarshal(testCase.in, &out)
			testCase.require(t, err)
			require.Equal(t, testCase.expected, out)
		})
	}
}
