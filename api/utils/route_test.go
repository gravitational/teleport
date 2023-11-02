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

package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// NOTE: much of the details of the behavior of this type is tested in lib/proxy as part
// of the main router test coverage.

// TestSSHRouteMatcherHostnameMatching verifies the expected behavior of the custom ssh
// hostname matching logic.
func TestSSHRouteMatcherHostnameMatching(t *testing.T) {
	tts := []struct {
		desc        string
		principal   string
		target      string
		insensitive bool
		match       bool
	}{
		{
			desc:        "upper-eq",
			principal:   "Foo",
			target:      "Foo",
			insensitive: true,
			match:       true,
		},
		{
			desc:        "lower-eq",
			principal:   "foo",
			target:      "foo",
			insensitive: true,
			match:       true,
		},
		{
			desc:        "lower-target-match",
			principal:   "Foo",
			target:      "foo",
			insensitive: true,
			match:       true,
		},
		{
			desc:        "upper-target-mismatch",
			principal:   "foo",
			target:      "Foo",
			insensitive: true,
			match:       false,
		},
		{
			desc:        "upper-mismatch",
			principal:   "Foo",
			target:      "fOO",
			insensitive: true,
			match:       false,
		},
		{
			desc:        "non-ascii-match",
			principal:   "🌲",
			target:      "🌲",
			insensitive: true,
			match:       true,
		},
		{
			desc:        "non-ascii-mismatch",
			principal:   "🌲",
			target:      "🔥",
			insensitive: true,
			match:       false,
		},
		{
			desc:        "sensitive-match",
			principal:   "Foo",
			target:      "Foo",
			insensitive: false,
			match:       true,
		},
		{
			desc:        "sensitive-mismatch",
			principal:   "Foo",
			target:      "foo",
			insensitive: false,
			match:       false,
		},
	}

	for _, tt := range tts {
		matcher := NewSSHRouteMatcher(tt.target, "", tt.insensitive)
		require.Equal(t, tt.match, matcher.routeToHostname(tt.principal), "desc=%q", tt.desc)
	}
}
