/*
 *
 * Copyright 2015-2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 *
 */

package service

import (
	"fmt"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"

	"github.com/gravitational/teleport/lib/defaults"
)

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		desc   string
		config *Config
		err    string
	}{
		{
			desc: "invalid version",
			config: &Config{
				Version: "v1.1",
			},
			err: fmt.Sprintf("config: version must be one of %s", strings.Join(defaults.TeleportVersions, ", ")),
		},
		{
			desc: "no service enabled",
			config: &Config{
				Version: defaults.TeleportConfigVersionV2,
			},
			err: "config: enable at least one of auth_service, ssh_service, proxy_service, app_service, database_service, kubernetes_service or windows_desktop_service",
		},
		{
			desc: "no auth_servers or proxy_servers specified",
			config: &Config{
				Version: defaults.TeleportConfigVersionV2,
				Auth: AuthConfig{
					Enabled: true,
				},
			},
			err: "config: auth_servers or proxy_servers is required",
		},
		{
			desc: "specifying proxy_servers with the wrong config version",
			config: &Config{
				Version: defaults.TeleportConfigVersionV2,
				Auth: AuthConfig{
					Enabled: true,
				},
				ProxyServers: []utils.NetAddr{
					*utils.MustParseAddr("0.0.0.0"),
				},
			},
			err: "config: proxy_servers is supported from config version v3 onwards",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			if err := validateConfig(test.config); err != nil {
				require.Equal(t, test.err, err)
			}
		})
	}
}
