/*
Copyright 2023 Gravitational, Inc.

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

package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsAllowedDomain(t *testing.T) {
	t.Parallel()
	allowedDomains := []string{
		"example.com",
		"alt.example.com",
	}
	tests := []struct {
		name   string
		url    string
		assert require.BoolAssertionFunc
	}{
		{
			name:   "domain match",
			url:    "example.com",
			assert: require.True,
		},
		{
			name:   "subdomain match",
			url:    "somewhere.example.com",
			assert: require.True,
		},
		{
			name:   "no match",
			url:    "fake.example2.com",
			assert: require.False,
		},
		{
			name:   "empty",
			url:    "",
			assert: require.False,
		},
		{
			name:   "not a url",
			url:    "$$$$",
			assert: require.False,
		},
		{
			name:   "suffix matches, but not as subdomain",
			url:    "fakeexample.com",
			assert: require.False,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.assert(t, isAllowedDomain(tc.url, allowedDomains))
		})
	}
}
