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

package service

import (
	"io"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
)

func validateConfig(cfg *Config) error {
	applyDefaults(cfg)

	if err := defaults.ValidateConfigVersion(cfg.Version); err != nil {
		return err
	}

	if err := verifyEnabledService(cfg); err != nil {
		return err
	}

	if err := validateAuthOrProxyServices(cfg); err != nil {
		return err
	}

	if cfg.DataDir == "" {
		return trace.BadParameter("config: please supply data directory")
	}

	for i := range cfg.Auth.Authorities {
		if err := services.ValidateCertAuthority(cfg.Auth.Authorities[i]); err != nil {
			return trace.Wrap(err)
		}
	}

	for _, tun := range cfg.ReverseTunnels {
		if err := services.ValidateReverseTunnel(tun); err != nil {
			return trace.Wrap(err)
		}
	}

	cfg.SSH.Namespace = types.ProcessNamespace(cfg.SSH.Namespace)

	return nil
}

func applyDefaults(cfg *Config) {
	if cfg.Version == "" {
		cfg.Version = defaults.TeleportConfigVersionV1
	}

	if cfg.Console == nil {
		cfg.Console = io.Discard
	}

	if cfg.Log == nil {
		cfg.Log = logrus.StandardLogger()
	}

	if cfg.PollingPeriod == 0 {
		cfg.PollingPeriod = defaults.LowResPollingPeriod
	}
}

func validateAuthOrProxyServices(cfg *Config) error {
	haveAuthServers := len(cfg.authServers) > 0
	haveProxyServer := !cfg.ProxyServer.IsEmpty()

	if cfg.Version == defaults.TeleportConfigVersionV3 {
		if haveAuthServers && haveProxyServer {
			return trace.BadParameter("config: cannot use both auth_server and proxy_server")
		}

		if !haveAuthServers && !haveProxyServer {
			return trace.BadParameter("config: auth_server or proxy_server is required")
		}

		if !cfg.Auth.Enabled {
			if haveAuthServers && cfg.Apps.Enabled {
				return trace.BadParameter("config: when app_service is enabled, proxy_server must be specified instead of auth_server")
			}

			if haveAuthServers && cfg.Databases.Enabled {
				return trace.BadParameter("config: when db_service is enabled, proxy_server must be specified instead of auth_server")
			}
		}

		if haveProxyServer {
			port := cfg.ProxyServer.Port(0)
			if port == defaults.AuthListenPort {
				cfg.Log.Warnf("config: proxy_server is pointing to port %d, is this the auth server address?", defaults.AuthListenPort)
			}
		}

		if haveAuthServers {
			authServerPort := cfg.authServers[0].Port(0)
			checkPorts := []int{defaults.HTTPListenPort, teleport.StandardHTTPSPort}
			for _, port := range checkPorts {
				if authServerPort == port {
					cfg.Log.Warnf("config: auth_server is pointing to port %d, is this the proxy server address?", port)
				}
			}
		}

		return nil
	}

	if haveProxyServer {
		return trace.BadParameter("config: proxy_server is supported from config version v3 onwards")
	}

	if !haveAuthServers {
		return trace.BadParameter("config: auth_servers is required")
	}

	return nil
}

func verifyEnabledService(cfg *Config) error {
	enabled := []bool{
		cfg.Auth.Enabled,
		cfg.SSH.Enabled,
		cfg.Proxy.Enabled,
		cfg.Kube.Enabled,
		cfg.Apps.Enabled,
		cfg.Databases.Enabled,
		cfg.WindowsDesktop.Enabled,
		cfg.Discovery.Enabled,
		cfg.Okta.Enabled,
	}

	for _, item := range enabled {
		if item {
			return nil
		}
	}

	return trace.BadParameter(
		"config: enable at least one of auth_service, ssh_service, proxy_service, app_service, database_service, kubernetes_service, windows_desktop_service or discover_service")
}
