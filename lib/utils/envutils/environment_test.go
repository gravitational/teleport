/*
Copyright 2017 Gravitational, Inc.

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
package envutils

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestReadEnvironmentFile(t *testing.T) {
	t.Parallel()

	// contents of environment file
	rawenv := []byte(`
foo=bar
# comment
foo=bar=baz
    # comment 2
=
foo=

=bar
LD_PRELOAD=attack
`)

	// create a temp file with an environment in it
	f, err := os.CreateTemp(t.TempDir(), "teleport-environment-")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	_, err = f.Write(rawenv)
	require.NoError(t, err)
	err = f.Close()
	require.NoError(t, err)

	// read in the temp file
	env, err := ReadEnvironmentFile(f.Name())
	require.NoError(t, err)

	// check we parsed it correctly
	require.Empty(t, cmp.Diff(env, []string{"foo=bar", "foo=bar=baz", "foo="}))
}

func TestSafeEnvAdd(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		keys     []string
		values   []string
		expected []string
	}{
		{
			name:     "normal add",
			keys:     []string{"foo"},
			values:   []string{"bar"},
			expected: []string{"foo=bar"},
		},
		{
			name:     "double add",
			keys:     []string{"one", "two"},
			values:   []string{"v1", "v2"},
			expected: []string{"one=v1", "two=v2"},
		},
		{
			name:     "whitespace trim",
			keys:     []string{" foo "},
			values:   []string{" bar "},
			expected: []string{"foo=bar"},
		},
		{
			name:     "skip dangerous exact",
			keys:     []string{"foo", "LD_PRELOAD"},
			values:   []string{"bar", "ignored"},
			expected: []string{"foo=bar"},
		},
		{
			name:     "skip dangerous lowercase",
			keys:     []string{"foo", "ld_preload"},
			values:   []string{"bar", "ignored"},
			expected: []string{"foo=bar"},
		},
		{
			name:     "skip dangerous with whitespace",
			keys:     []string{"foo", "  LD_PRELOAD"},
			values:   []string{"bar", "ignored"},
			expected: []string{"foo=bar"},
		},
	}

	for _, tc := range testCases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			require.Len(t, tc.keys, len(tc.values))

			env := &SafeEnv{}
			for i := range tc.keys {
				env.Add(tc.keys[i], tc.values[i])
			}
			result := []string(*env)

			require.Equal(t, tc.expected, result)
		})
	}
}

func TestSafeEnvAddFull(t *testing.T) {
	testCases := []struct {
		name       string
		fullValues []string
		expected   []string
	}{
		{
			name:       "normal add",
			fullValues: []string{"foo=bar"},
			expected:   []string{"foo=bar"},
		},
		{
			name:       "double add",
			fullValues: []string{"one=v1", "two=v2"},
			expected:   []string{"one=v1", "two=v2"},
		},
		{
			name:       "whitespace trim",
			fullValues: []string{" foo=bar "},
			expected:   []string{"foo=bar"},
		},
		{
			name:       "skip dangerous exact",
			fullValues: []string{"foo=bar", "LD_PRELOAD=ignored"},
			expected:   []string{"foo=bar"},
		},
		{
			name:       "skip dangerous lowercase",
			fullValues: []string{"foo=bar", "ld_preload=ignored"},
			expected:   []string{"foo=bar"},
		},
		{
			name:       "skip dangerous with whitespace",
			fullValues: []string{"foo=bar", "  LD_PRELOAD=ignored"},
			expected:   []string{"foo=bar"},
		},
	}

	for _, tc := range testCases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			env := &SafeEnv{}
			env.AddFull(tc.fullValues...)
			result := []string(*env)

			require.Equal(t, tc.expected, result)
		})
	}
}
