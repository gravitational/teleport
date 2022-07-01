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
	"io"
	"strings"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
)

func validateConfig(cfg *Config) error {
	applyDefaults(cfg)

	if err := validateVersion(cfg); err != nil {
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
	haveAuthServers := len(cfg.AuthServers) > 0
	haveProxyServers := len(cfg.ProxyServers) > 0

	fmt.Printf("has proxy", haveProxyServers)
	fmt.Printf("cfg version", cfg.Version)

	if haveProxyServers && cfg.Version != defaults.TeleportConfigVersionV3 {
		return trace.BadParameter("config: proxy_servers is supported from config version v3 onwards")
	}

	if haveAuthServers && haveProxyServers {
		return trace.BadParameter("config: cannot use both auth_servers and proxy_servers")
	}

	if !haveAuthServers && !haveProxyServers {
		return trace.BadParameter("config: auth_servers or proxy_servers is required")
	}

	return nil
}

func validateVersion(cfg *Config) error {
	has := false

	for _, version := range defaults.TeleportVersions {
		if cfg.Version == version {
			has = true

			break
		}
	}

	if !has {
		return trace.BadParameter("config: version must be one of %s", strings.Join(defaults.TeleportVersions, ", "))
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
	}

	has := false
	for _, item := range enabled {
		if item {
			has = true

			break
		}
	}

	if !has {
		return trace.BadParameter(
			"config: enable at least one of auth_service, ssh_service, proxy_service, app_service, database_service, kubernetes_service or windows_desktop_service")
	}

	return nil
}
