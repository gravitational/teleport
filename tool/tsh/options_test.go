/*
Copyright 2022 Gravitational, Inc.

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

package main

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/client"
	"github.com/stretchr/testify/require"
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
				require.Equal(t, true, opts.AddKeysToAgent)
			},
		},
		{
			desc:        "Equals Sign Delimited",
			inOptions:   []string{"AddKeysToAgent=yes"},
			assertError: require.NoError,
			assertOptions: func(t *testing.T, opts Options) {
				require.Equal(t, true, opts.AddKeysToAgent)
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
				require.Equal(t, true, opts.ForwardX11)
			},
		},
		{
			desc:        "Forward X11",
			inOptions:   []string{"ForwardX11 no"},
			assertError: require.NoError,
			assertOptions: func(t *testing.T, opts Options) {
				require.Equal(t, false, opts.ForwardX11)
			},
		},
		{
			desc:        "Forward X11 InvalidValue",
			inOptions:   []string{"ForwardX11 potato"},
			assertError: require.Error,
		},
		// ForwardX11Trusted tests
		{
			desc:        "Forward X11 Trusted",
			inOptions:   []string{"ForwardX11Trusted yes"},
			assertError: require.NoError,
			assertOptions: func(t *testing.T, opts Options) {
				require.Equal(t, true, opts.ForwardX11Trusted)
			},
		},
		{
			desc:        "Forward X11 Trusted",
			inOptions:   []string{"ForwardX11Trusted no"},
			assertError: require.NoError,
			assertOptions: func(t *testing.T, opts Options) {
				require.Equal(t, false, opts.ForwardX11Trusted)
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
			options, err := parseOptions(tt.inOptions)
			tt.assertError(t, err)
			if tt.assertOptions != nil {
				tt.assertOptions(t, options)
			}
		})
	}
}
