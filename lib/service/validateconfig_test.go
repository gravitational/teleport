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
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
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
			err: fmt.Sprintf("version must be one of %s", strings.Join(defaults.TeleportConfigVersions, ", ")),
		},
		{
			desc: "no service enabled",
			config: &Config{
				Version: defaults.TeleportConfigVersionV2,
			},
			err: "config: enable at least one of auth_service, ssh_service, proxy_service, app_service, database_service, kubernetes_service, windows_desktop_service or discover_service",
		},
		{
			desc: "no auth_servers or proxy_server specified",
			config: &Config{
				Version: defaults.TeleportConfigVersionV3,
				Auth: AuthConfig{
					Enabled: true,
				},
			},
			err: "config: auth_server or proxy_server is required",
		},
		{
			desc: "no auth_servers specified",
			config: &Config{
				Version: defaults.TeleportConfigVersionV2,
				Auth: AuthConfig{
					Enabled: true,
				},
			},
			err: "config: auth_servers is required",
		},
		{
			desc: "specifying proxy_server with the wrong config version",
			config: &Config{
				Version: defaults.TeleportConfigVersionV2,
				Auth: AuthConfig{
					Enabled: true,
				},
				ProxyServer: *utils.MustParseAddr("0.0.0.0"),
			},
			err: "config: proxy_server is supported from config version v3 onwards",
		},
		{
			desc: "specifying auth_server when app_service is enabled",
			config: &Config{
				Version: defaults.TeleportConfigVersionV3,
				Apps: AppsConfig{
					Enabled: true,
				},
				DataDir:     "/",
				authServers: []utils.NetAddr{*utils.MustParseAddr("0.0.0.0")},
			},
			err: "config: when app_service is enabled, proxy_server must be specified instead of auth_server",
		},
		{
			desc: "specifying auth_server when db_service is enabled",
			config: &Config{
				Version: defaults.TeleportConfigVersionV3,
				Databases: DatabasesConfig{
					Enabled: true,
				},
				DataDir:     "/",
				authServers: []utils.NetAddr{*utils.MustParseAddr("0.0.0.0")},
			},
			err: "config: when db_service is enabled, proxy_server must be specified instead of auth_server",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := validateConfig(test.config)
			if test.err == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, test.err)
			}
		})
	}
}

func TestVerifyEnabledService(t *testing.T) {
	tests := []struct {
		desc             string
		config           *Config
		errAssertionFunc require.ErrorAssertionFunc
	}{
		{
			desc:             "auth enabled",
			config:           &Config{Auth: AuthConfig{Enabled: true}},
			errAssertionFunc: require.NoError,
		},
		{
			desc:             "ssh enabled",
			config:           &Config{SSH: SSHConfig{Enabled: true}},
			errAssertionFunc: require.NoError,
		},
		{
			desc:             "proxy enabled",
			config:           &Config{Proxy: ProxyConfig{Enabled: true}},
			errAssertionFunc: require.NoError,
		},
		{
			desc:             "kube enabled",
			config:           &Config{Kube: KubeConfig{Enabled: true}},
			errAssertionFunc: require.NoError,
		},
		{
			desc:             "apps enabled",
			config:           &Config{Apps: AppsConfig{Enabled: true}},
			errAssertionFunc: require.NoError,
		},
		{
			desc:             "databases enabled",
			config:           &Config{Databases: DatabasesConfig{Enabled: true}},
			errAssertionFunc: require.NoError,
		},
		{
			desc:             "windows desktop enabled",
			config:           &Config{WindowsDesktop: WindowsDesktopConfig{Enabled: true}},
			errAssertionFunc: require.NoError,
		},
		{
			desc:             "discovery enabled",
			config:           &Config{Discovery: DiscoveryConfig{Enabled: true}},
			errAssertionFunc: require.NoError,
		},
		{
			desc:             "okta enabled",
			config:           &Config{Okta: OktaConfig{Enabled: true}},
			errAssertionFunc: require.NoError,
		},
		{
			desc:   "nothing enabled",
			config: &Config{},
			errAssertionFunc: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			test.errAssertionFunc(t, verifyEnabledService(test.config))
		})
	}
}
