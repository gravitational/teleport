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

package opensearch

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigNoTLS(t *testing.T) {
	want := Config{
		Profiles: []Profile{
			{
				Name:     "teleport",
				Endpoint: "http://host.example.com:8080/",
				MaxRetry: 3,
				Timeout:  10,
			},
		},
	}

	got := ConfigNoTLS("host.example.com", 8080)
	require.Equal(t, want, got)
}

func TestConfigTLS(t *testing.T) {
	want := Config{
		Profiles: []Profile{
			{
				Name:     "teleport",
				Endpoint: "https://host.example.com:8080/",
				Certificate: &Certificate{
					CACert: "/foo/bar/ca.cert",
					Cert:   "/priv/clt.cert",
					Key:    "/priv/clt.key",
				},
				MaxRetry: 3,
				Timeout:  10,
			},
		},
	}

	got := ConfigTLS("host.example.com", 8080, "/foo/bar/ca.cert", "/priv/clt.cert", "/priv/clt.key")
	require.Equal(t, want, got)
}

func TestWriteConfig(t *testing.T) {
	tmp := t.TempDir()
	fn, err := WriteConfig(tmp, ConfigNoTLS("host.example.com", 8080))
	require.NoError(t, err)
	require.Equal(t, filepath.Join(tmp, "opensearch-cli", "150502df.yml"), fn)
	bytes, err := os.ReadFile(fn)
	require.NoError(t, err)
	require.Equal(t, `profiles:
- endpoint: http://host.example.com:8080/
  max_retry: 3
  name: teleport
  timeout: 10
`, string(bytes))
}
