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
