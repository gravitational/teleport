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

package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/client"
)

func TestOptions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		desc          string
		inOptions     []string
		assertError   require.ErrorAssertionFunc
		assertOptions func(t *testing.T, opts Options)
	}{
		// Default options
		{
			desc:        "default options",
			assertError: require.NoError,
			assertOptions: func(t *testing.T, opts Options) {
				require.Equal(t, defaultOptions(), opts)
			},
		},
		// AddKeysToAgent Tests and Generic option-parsing tests
		{
			desc:        "Space Delimited",
			inOptions:   []string{"AddKeysToAgent yes"},
			assertError: require.NoError,
			assertOptions: func(t *testing.T, opts Options) {
				require.True(t, opts.AddKeysToAgent)
			},
		},
		{
			desc:        "Equals Sign Delimited",
			inOptions:   []string{"AddKeysToAgent=yes"},
			assertError: require.NoError,
			assertOptions: func(t *testing.T, opts Options) {
				require.True(t, opts.AddKeysToAgent)
			},
		},
		{
			desc:        "Unsupported key",
			inOptions:   []string{"foo foo"},
			assertError: require.Error,
		},
		{
			desc:        "Unsupported option gets skipped",
			inOptions:   []string{"AddressFamily val"},
			assertError: require.NoError,
		},
		{
			desc:        "Incomplete option",
			inOptions:   []string{"AddKeysToAgent"},
			assertError: require.Error,
		},
		{
			desc:        "AddKeysToAgent Invalid Value",
			inOptions:   []string{"AddKeysToAgent foo"},
			assertError: require.Error,
		},
		// ForwardAgent Tests
		{
			desc:        "Forward Agent Yes",
			inOptions:   []string{"ForwardAgent yes"},
			assertError: require.NoError,
			assertOptions: func(t *testing.T, opts Options) {
				require.Equal(t, client.ForwardAgentYes, opts.ForwardAgent)
			},
		},
		{
			desc:        "Forward Agent No",
			inOptions:   []string{"ForwardAgent no"},
			assertError: require.NoError,
			assertOptions: func(t *testing.T, opts Options) {
				require.Equal(t, client.ForwardAgentNo, opts.ForwardAgent)
			},
		},
		{
			desc:        "Forward Agent Local",
			inOptions:   []string{"ForwardAgent local"},
			assertError: require.NoError,
			assertOptions: func(t *testing.T, opts Options) {
				require.Equal(t, client.ForwardAgentLocal, opts.ForwardAgent)
			},
		},
		{
			desc:        "Forward Agent InvalidValue",
			inOptions:   []string{"ForwardAgent potato"},
			assertError: require.Error,
		},
		// ForwardX11 tests
		{
			desc:        "Forward X11",
			inOptions:   []string{"ForwardX11 yes"},
			assertError: require.NoError,
			assertOptions: func(t *testing.T, opts Options) {
				require.True(t, opts.ForwardX11)
				require.True(t, *opts.ForwardX11Trusted)
			},
		},
		{
			desc:        "Forward X11 InvalidValue",
			inOptions:   []string{"ForwardX11 potato"},
			assertError: require.Error,
		},
		// ForwardX11Trusted tests
		{
			desc:        "Forward X11 Trusted yes",
			inOptions:   []string{"ForwardX11Trusted yes"},
			assertError: require.NoError,
			assertOptions: func(t *testing.T, opts Options) {
				require.True(t, *opts.ForwardX11Trusted)
			},
		},
		{
			desc:        "Forward X11 Trusted no",
			inOptions:   []string{"ForwardX11Trusted no"},
			assertError: require.NoError,
			assertOptions: func(t *testing.T, opts Options) {
				require.False(t, *opts.ForwardX11Trusted)
			},
		},
		{
			desc:        "Forward X11 Trusted with Forward X11",
			inOptions:   []string{"ForwardX11 yes", "ForwardX11Trusted no"},
			assertError: require.NoError,
			assertOptions: func(t *testing.T, opts Options) {
				require.True(t, opts.ForwardX11)
				require.False(t, *opts.ForwardX11Trusted)
			},
		},
		{
			desc:        "Forward X11 Trusted InvalidValue",
			inOptions:   []string{"ForwardX11Trusted potato"},
			assertError: require.Error,
		},
		// ForwardX11Trusted tests
		{
			desc:        "Forward X11 Timeout",
			inOptions:   []string{"ForwardX11Timeout 10"},
			assertError: require.NoError,
			assertOptions: func(t *testing.T, opts Options) {
				require.Equal(t, time.Second*10, opts.ForwardX11Timeout)
			},
		},
		{
			desc:        "Forward X11 Timeout 0",
			inOptions:   []string{"ForwardX11Timeout 0"},
			assertError: require.NoError,
			assertOptions: func(t *testing.T, opts Options) {
				require.Equal(t, time.Duration(0), opts.ForwardX11Timeout)
			},
		},
		{
			desc:        "Forward X11 Timeout negative",
			inOptions:   []string{"ForwardX11Timeout -1"},
			assertError: require.Error,
		},
		{
			desc:        "Forward X11 Timeout devimal",
			inOptions:   []string{"ForwardX11Timeout 1.5"},
			assertError: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			options, err := parseOptions(tt.inOptions)
			tt.assertError(t, err)
			if tt.assertOptions != nil {
				tt.assertOptions(t, options)
			}
		})
	}
}
