/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package config

import (
	"testing"

	"github.com/goccy/go-yaml/parser"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tbot/bot"
)

type checkAndSetDefaulter interface {
	CheckAndSetDefaults() error
}

type testCheckAndSetDefaultsCase[T checkAndSetDefaulter] struct {
	name string
	in   func() T

	// want specifies the desired state of the checkAndSetDefaulter after
	// check and set defaults has been run. If want is nil, the Output is
	// compared to its initial state.
	want    checkAndSetDefaulter
	wantErr string
}

func memoryDestForTest() bot.Destination {
	return &DestinationMemory{store: map[string][]byte{}}
}

func testCheckAndSetDefaults[T checkAndSetDefaulter](t *testing.T, tests []testCheckAndSetDefaultsCase[T]) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.in()
			err := got.CheckAndSetDefaults()
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}

			want := tt.want
			if want == nil {
				want = tt.in()
			}
			require.NoError(t, err)
			require.Equal(t, want, got)
		})
	}
}

func TestExtractOutputDestination(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		yaml              string
		assert            assert.ErrorAssertionFunc
		expectDestination bot.Destination
	}{
		{
			name: "ok destination",
			yaml: `ignore: this
type: identity
destination:
  type: directory
  path: /tmp/foo
red: herring`,
			assert: assert.NoError,
			expectDestination: &DestinationDirectory{
				Path: "/tmp/foo",
			},
		},
		{
			name: "destination missing",
			yaml: `ignore: this
type: identity
red: herring`,
			assert: assert.NoError,
		},
		{
			name: "multiple documents",
			yaml: `ignore: this
type: identity
destination:
  type: directory
  path: /tmp/foo
red: herring
---
ignore: this
type: identity
destination:
  type: directory
  path: /tmp/foo
red: herring`,
			assert: assert.Error,
		},
		{
			name:   "bad format",
			yaml:   `["destination", "type", "directory"]`,
			assert: assert.Error,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := parser.ParseBytes([]byte(tc.yaml), parser.ParseComments)
			require.NoError(t, err)
			dest, err := extractOutputDestination(parsed)
			tc.assert(t, err)
			assert.Equal(t, tc.expectDestination, dest)
		})
	}
}
