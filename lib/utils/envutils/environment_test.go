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

package envutils

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadEnvironment(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// contents of environment file
	rawenv := []byte(`
foo=bar
# comment
foo=bar=baz
    # comment 2
=
foo=

=bar
bar=foo
LD_PRELOAD=attack
`)

	env, err := ReadEnvironment(ctx, bytes.NewReader(rawenv))
	require.NoError(t, err)

	// check we parsed it correctly
	require.Equal(t, []string{"foo=bar", "foo=bar=baz", "foo=", "bar=foo"}, env)
}

func TestSafeEnvAdd(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		excludeDuplicate bool
		keys             []string
		values           []string
		expected         []string
	}{
		{
			name:             "normal add",
			excludeDuplicate: true,
			keys:             []string{"foo"},
			values:           []string{"bar"},
			expected:         []string{"foo=bar"},
		},
		{
			name:             "double add",
			excludeDuplicate: true,
			keys:             []string{"one", "two"},
			values:           []string{"v1", "v2"},
			expected:         []string{"one=v1", "two=v2"},
		},
		{
			name:             "whitespace trim",
			excludeDuplicate: true,
			keys:             []string{" foo "},
			values:           []string{" bar "},
			expected:         []string{"foo=bar"},
		},
		{
			name:             "duplicate ignore",
			excludeDuplicate: true,
			keys:             []string{"one", "one"},
			values:           []string{"v1", "v2"},
			expected:         []string{"one=v1"},
		},
		{
			name:             "duplicate different case ignore",
			excludeDuplicate: true,
			keys:             []string{"one", "ONE"},
			values:           []string{"v1", "v2"},
			expected:         []string{"one=v1"},
		},
		{
			name:             "duplicate allowed",
			excludeDuplicate: false,
			keys:             []string{"one", "one"},
			values:           []string{"v1", "v2"},
			expected:         []string{"one=v1", "one=v2"},
		},
		{
			name:             "skip dangerous exact",
			excludeDuplicate: true,
			keys:             []string{"foo", "LD_PRELOAD"},
			values:           []string{"bar", "ignored"},
			expected:         []string{"foo=bar"},
		},
		{
			name:             "skip dangerous lowercase",
			excludeDuplicate: true,
			keys:             []string{"foo", "ld_preload"},
			values:           []string{"bar", "ignored"},
			expected:         []string{"foo=bar"},
		},
		{
			name:             "skip dangerous with whitespace",
			excludeDuplicate: true,
			keys:             []string{"foo", "  LD_PRELOAD"},
			values:           []string{"bar", "ignored"},
			expected:         []string{"foo=bar"},
		},
	}

	for _, tc := range testCases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			require.Len(t, tc.keys, len(tc.values))

			env := &SafeEnv{}
			for i := range tc.keys {
				env.add(tc.excludeDuplicate, tc.keys[i], tc.values[i])
			}
			result := []string(*env)

			require.Equal(t, tc.expected, result)
		})
	}
}

func TestSafeEnvAddFull(t *testing.T) {
	testCases := []struct {
		name             string
		excludeDuplicate bool
		fullValues       []string
		expected         []string
	}{
		{
			name:             "normal add",
			excludeDuplicate: true,
			fullValues:       []string{"foo=bar"},
			expected:         []string{"foo=bar"},
		},
		{
			name:             "double add",
			excludeDuplicate: true,
			fullValues:       []string{"one=v1", "two=v2"},
			expected:         []string{"one=v1", "two=v2"},
		},
		{
			name:             "whitespace trim",
			excludeDuplicate: true,
			fullValues:       []string{" foo=bar "},
			expected:         []string{"foo=bar"},
		},
		{
			name:             "duplicate ignore",
			excludeDuplicate: true,
			fullValues:       []string{"one=v1", "one=v2"},
			expected:         []string{"one=v1"},
		},
		{
			name:             "duplicate ignore different case",
			excludeDuplicate: true,
			fullValues:       []string{"one=v1", "ONE=v2"},
			expected:         []string{"one=v1"},
		},
		{
			name:             "duplicate allowed",
			excludeDuplicate: false,
			fullValues:       []string{"one=v1", "one=v2"},
			expected:         []string{"one=v1", "one=v2"},
		},
		{
			name:             "double equal value",
			excludeDuplicate: true,
			fullValues:       []string{"foo=bar=badvalue"},
			expected:         []string{"foo=bar=badvalue"},
		},
		{
			name:             "skip dangerous exact",
			excludeDuplicate: true,
			fullValues:       []string{"foo=bar", "LD_PRELOAD=ignored"},
			expected:         []string{"foo=bar"},
		},
		{
			name:             "skip dangerous lowercase",
			excludeDuplicate: true,
			fullValues:       []string{"foo=bar", "ld_preload=ignored"},
			expected:         []string{"foo=bar"},
		},
		{
			name:             "skip dangerous with whitespace",
			excludeDuplicate: true,
			fullValues:       []string{"foo=bar", "  LD_PRELOAD=ignored"},
			expected:         []string{"foo=bar"},
		},
	}

	for _, tc := range testCases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			env := &SafeEnv{}
			env.addFull(tc.excludeDuplicate, tc.fullValues)
			result := []string(*env)

			require.Equal(t, tc.expected, result)
		})
	}
}
