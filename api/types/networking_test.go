/*
Copyright 2021 Gravitational, Inc.

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

package types

import (
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/gravitational/teleport/api/defaults"
)

func TestProxyListenerModeMarshalYAML(t *testing.T) {
	tt := []struct {
		name string
		in   ProxyListenerMode
		want string
	}{
		{
			name: "default value",
			want: "separate",
		},
		{
			name: "multiplex mode",
			in:   ProxyListenerMode_Multiplex,
			want: "multiplex",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			buff, err := yaml.Marshal(&tc.in)
			require.NoError(t, err)
			require.Equal(t, tc.want, strings.TrimRight(string(buff), "\n"))
		})
	}
}

func TestProxyListenerModeUnmarshalYAML(t *testing.T) {
	tt := []struct {
		name    string
		in      string
		want    ProxyListenerMode
		wantErr func(*testing.T, error)
	}{
		{
			name: "default value",
			in:   "",
			want: ProxyListenerMode_Separate,
		},
		{
			name: "multiplex",
			in:   "multiplex",
			want: ProxyListenerMode_Multiplex,
		},
		{
			name: "invalid value",
			in:   "unknown value",
			wantErr: func(t *testing.T, err error) {
				require.True(t, trace.IsBadParameter(err))
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var got ProxyListenerMode
			err := yaml.Unmarshal([]byte(tc.in), &got)
			if tc.wantErr != nil {
				tc.wantErr(t, err)
				return
			}
			require.Equal(t, tc.want, got)
		})
	}
}

func TestSSHDialTimeout(t *testing.T) {
	cfg := DefaultClusterNetworkingConfig()

	// Validate that the default config, which has no dial timeout set
	// returns the default value.
	require.Equal(t, defaults.DefaultIOTimeout, cfg.GetSSHDialTimeout())

	// A zero value can be set, but retrieving a zero value will result
	// in the default value since we cannot distinguish between an
	// old config prior to the addition of the ssh dial timeout being added
	// and an explicitly set value of zero.
	cfg.SetSSHDialTimeout(0)
	require.Equal(t, defaults.DefaultIOTimeout, cfg.GetSSHDialTimeout())

	// Validate that a non-zero value is honored.
	cfg.SetSSHDialTimeout(time.Minute)
	require.Equal(t, time.Minute, cfg.GetSSHDialTimeout())

	// Validate that unmarshaling a config without a timeout set
	// returns the default value.
	raw, err := yaml.Marshal(DefaultClusterNetworkingConfig())
	require.NoError(t, err)

	var cnc ClusterNetworkingConfigV2
	require.NoError(t, yaml.Unmarshal(raw, &cnc))
	require.Equal(t, defaults.DefaultIOTimeout, cnc.GetSSHDialTimeout())

	// Validate that unmarshaling a config with a valid timeout
	// set returns the correct duration.
	raw, err = yaml.Marshal(cfg)
	require.NoError(t, err)

	require.NoError(t, yaml.Unmarshal(raw, &cnc))
	require.Equal(t, time.Minute, cnc.GetSSHDialTimeout())
}
