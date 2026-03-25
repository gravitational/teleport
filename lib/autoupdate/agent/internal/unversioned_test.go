/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package internal_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/autoupdate/agent/internal"
	"github.com/gravitational/teleport/lib/config"
	tbotconfig "github.com/gravitational/teleport/lib/tbot/config"
)

// In the future, the latest version of the updater may need to read a version of teleport.yaml that has
// an unsupported version which is supported by the updater-managed version of Teleport.
// This test will break if Teleport removes a field that the updater reads.
func TestUnversionedTeleportConfig(t *testing.T) {
	for _, tt := range []struct {
		name    string
		version string
		in      internal.UnversionedTeleport
		err     bool
	}{
		{
			name: "empty",
			in: internal.UnversionedTeleport{
				Teleport: internal.UnversionedConfig{
					ProxyServer: "proxy.example.com",
					AuthServer:  "auth.example.com",
					AuthServers: []string{"auth1.example.com", "auth2.example.com"},
					DataDir:     "example_dir",
				},
			},
		},
		{
			name:    "v1",
			version: string(tbotconfig.V1),
			in: internal.UnversionedTeleport{
				Teleport: internal.UnversionedConfig{
					ProxyServer: "proxy.example.com",
					AuthServer:  "auth.example.com",
					AuthServers: []string{"auth1.example.com", "auth2.example.com"},
					DataDir:     "example_dir",
				},
			},
		},
		{
			name:    "v2",
			version: string(tbotconfig.V2),
			in: internal.UnversionedTeleport{
				Teleport: internal.UnversionedConfig{
					ProxyServer: "proxy.example.com",
					AuthServer:  "auth.example.com",
					AuthServers: []string{"auth1.example.com", "auth2.example.com"},
					DataDir:     "example_dir",
				},
			},
		},
		{
			name:    "v3", // if this fails, add any new fields to the unversioned config
			version: "v3",
			in: internal.UnversionedTeleport{
				Teleport: internal.UnversionedConfig{
					ProxyServer: "proxy.example.com",
					AuthServer:  "auth.example.com",
					AuthServers: []string{"auth1.example.com", "auth2.example.com"},
					DataDir:     "example_dir",
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			in := struct {
				Version                      string `yaml:"version"`
				internal.UnversionedTeleport `yaml:",inline"`
			}{
				Version:             tt.version,
				UnversionedTeleport: tt.in,
			}
			var inB bytes.Buffer
			err := yaml.NewEncoder(&inB).Encode(in)
			require.NoError(t, err)
			fc, err := config.ReadConfig(&inB)
			if tt.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			var outB bytes.Buffer
			err = yaml.NewEncoder(&outB).Encode(fc)
			require.NoError(t, err)

			var out internal.UnversionedTeleport
			err = yaml.NewDecoder(&outB).Decode(&out)
			require.NoError(t, err)
			require.Equal(t, tt.in, out)
		})
	}

}

// In the future, the latest version of the updater may need to read a version of tbot.yaml that has
// an unsupported version which is supported by the updater-managed version of tbot.
// This test will break if tbot removes a field that the updater reads.
func TestUnversionedTbotConfig(t *testing.T) {
	for _, tt := range []struct {
		name    string
		version string
		in      internal.UnversionedConfig
		err     bool
	}{
		{
			name: "empty",
			in: internal.UnversionedConfig{
				AuthServer: "auth.example.com",
			},
			err: true,
		},
		{
			name:    "v1",
			version: string(tbotconfig.V1),
			in: internal.UnversionedConfig{
				AuthServer: "auth.example.com",
			},
			err: true,
		},
		{
			name:    "v2",
			version: string(tbotconfig.V2),
			in: internal.UnversionedConfig{
				AuthServer:  "auth.example.com",
				ProxyServer: "proxy.example.com",
			},
		},
		{
			name:    "v3", // when this fails, add any new fields to the unversioned config
			version: "v3",
			in:      internal.UnversionedConfig{},
			err:     true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			in := struct {
				Version                    string `yaml:"version"`
				internal.UnversionedConfig `yaml:",inline"`
			}{
				Version:           tt.version,
				UnversionedConfig: tt.in,
			}
			var inB bytes.Buffer
			err := yaml.NewEncoder(&inB).Encode(in)
			require.NoError(t, err)
			fc, err := tbotconfig.ReadConfig(bytes.NewReader(inB.Bytes()), false)
			if tt.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			var outB bytes.Buffer
			err = yaml.NewEncoder(&outB).Encode(fc)
			require.NoError(t, err)

			var out internal.UnversionedConfig
			err = yaml.NewDecoder(&outB).Decode(&out)
			require.NoError(t, err)
			require.Equal(t, tt.in, out)
		})
	}
}
