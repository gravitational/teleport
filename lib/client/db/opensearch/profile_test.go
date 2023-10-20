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

package opensearch

import (
	"os"
	"path"
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
	require.Equal(t, path.Join(tmp, "opensearch-cli", "150502df.yml"), fn)
	bytes, err := os.ReadFile(fn)
	require.NoError(t, err)
	require.Equal(t, `profiles:
- endpoint: http://host.example.com:8080/
  max_retry: 3
  name: teleport
  timeout: 10
`, string(bytes))
}
