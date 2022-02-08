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

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
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
				require.IsType(t, &trace.BadParameterError{}, err.(*trace.TraceErr).OrigError())
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

func TestProxyPeeringMarshalYAML(t *testing.T) {
	tt := []struct {
		name string
		in   ProxyPeering
		want string
	}{
		{
			name: "default value",
			want: "disabled",
		},
		{
			name: "multiplex mode",
			in:   ProxyPeering_Enabled,
			want: "enabled",
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

func TestProxyPeeringUnmarshalYAML(t *testing.T) {
	tt := []struct {
		name    string
		in      string
		want    ProxyPeering
		wantErr func(*testing.T, error)
	}{
		{
			name: "default value",
			in:   "",
			want: ProxyPeering_Disabled,
		},
		{
			name: "enabled",
			in:   "enabled",
			want: ProxyPeering_Enabled,
		},
		{
			name: "disabled",
			in:   "disabled",
			want: ProxyPeering_Disabled,
		},
		{
			name: "invalid value",
			in:   "unknown value",
			wantErr: func(t *testing.T, err error) {
				require.IsType(t, &trace.BadParameterError{}, err.(*trace.TraceErr).OrigError())
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var got ProxyPeering
			err := yaml.Unmarshal([]byte(tc.in), &got)
			if tc.wantErr != nil {
				tc.wantErr(t, err)
				return
			}
			require.Equal(t, tc.want, got)
		})
	}
}
