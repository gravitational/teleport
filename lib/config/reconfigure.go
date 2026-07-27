/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package config

import (
	"fmt"
	"maps"
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

// ReconfigureRequest describes the changes Reconfigure applies to a parsed
// FileConfig. A zero-valued field leaves the corresponding setting untouched;
// none of the fields has a meaningful empty value.
type ReconfigureRequest struct {
	// Proxy sets proxy_server. Valid only for v3 configs.
	Proxy string
	// AuthServer sets auth_server (v3) or replaces auth_servers with the
	// single given address (v1/v2). Mutually exclusive with Proxy.
	AuthServer string
	// CAPins replaces ca_pin with the target cluster's pins.
	CAPins []string
	// Token sets join_params.token_name and clears the legacy auth_token.
	Token string
	// RegistrationSecret sets the bound_keypair registration secret value
	// and clears the incompatible registration_secret_path.
	RegistrationSecret string
	// RegistrationSecretPath sets the bound_keypair registration secret path
	// and clears the incompatible registration_secret_value. Mutually
	// exclusive with RegistrationSecret.
	RegistrationSecretPath string
	// NodeLabels is a comma-separated k=v list merged additively into
	// ssh_service labels; on a key conflict the new value wins.
	NodeLabels string
	// DataDir sets teleport.data_dir.
	DataDir string
	// PIDFile sets teleport.pid_file.
	PIDFile string
	// DiagAddr sets teleport.diag_addr.
	DiagAddr string
	// SSHListenAddr sets ssh_service.listen_addr.
	SSHListenAddr string
	// KubeListenAddr sets kubernetes_service.listen_addr.
	KubeListenAddr string
	// MetricsListenAddr sets metrics_service.listen_addr.
	MetricsListenAddr string
}

// Reconfigure applies req to fc in place, leaving everything not named by a
// request field untouched. It returns warnings for values kept from the
// input that would collide with a second agent on the same host.
func Reconfigure(fc *FileConfig, req ReconfigureRequest) ([]string, error) {
	if req.Proxy != "" && req.AuthServer != "" {
		return nil, trace.BadParameter("--proxy and --auth-server are mutually exclusive")
	}
	if req.RegistrationSecret != "" && req.RegistrationSecretPath != "" {
		return nil, trace.BadParameter("--registration-secret and --registration-secret-path are mutually exclusive")
	}
	if req.Proxy != "" && fc.Version != defaults.TeleportConfigVersionV3 {
		return nil, trace.BadParameter("--proxy requires a v3 config; use --auth-server for v1/v2 configs")
	}
	if req.Proxy != "" && fc.Proxy.Enabled() {
		return nil, trace.BadParameter("--proxy requires a config with proxy_service disabled; the agent rejects proxy_server when the proxy service is enabled")
	}
	// Mirror the parsers the agent runs at start (applyConfig and the
	// per-service apply functions), or the tool writes a config the agent rejects.
	if req.Proxy != "" {
		if _, err := utils.ParseHostPortAddr(req.Proxy, defaults.HTTPListenPort); err != nil {
			return nil, trace.Wrap(err, "parsing --proxy")
		}
	}
	if req.AuthServer != "" {
		if _, err := utils.ParseHostPortAddr(req.AuthServer, defaults.AuthListenPort); err != nil {
			return nil, trace.Wrap(err, "parsing --auth-server")
		}
	}
	if len(req.CAPins) > 0 {
		// Nil certs: format check only; the pins are verified against the
		// target cluster's CA when the agent joins.
		if err := utils.CheckSPKI(req.CAPins, nil); err != nil {
			return nil, trace.Wrap(err, "parsing --ca-pin")
		}
	}
	if req.DiagAddr != "" {
		if _, err := utils.ParseAddr(req.DiagAddr); err != nil {
			return nil, trace.Wrap(err, "parsing --diag-addr")
		}
	}
	if req.SSHListenAddr != "" {
		if _, err := utils.ParseHostPortAddr(req.SSHListenAddr, defaults.SSHServerListenPort); err != nil {
			return nil, trace.Wrap(err, "parsing --ssh-listen-addr")
		}
	}
	if req.KubeListenAddr != "" {
		// Not a typo: applyKubeConfig uses the SSH proxy port as kube's default.
		if _, err := utils.ParseHostPortAddr(req.KubeListenAddr, defaults.SSHProxyListenPort); err != nil {
			return nil, trace.Wrap(err, "parsing --kube-listen-addr")
		}
	}
	if req.MetricsListenAddr != "" {
		if _, err := utils.ParseHostPortAddr(req.MetricsListenAddr, defaults.MetricsListenPort); err != nil {
			return nil, trace.Wrap(err, "parsing --metrics-listen-addr")
		}
	}

	if req.Proxy != "" {
		fc.ProxyServer = req.Proxy
		fc.AuthServer = ""
		fc.AuthServers = nil
		fc.CAPin = nil
	}
	if req.AuthServer != "" {
		if fc.Version == defaults.TeleportConfigVersionV3 {
			fc.AuthServer = req.AuthServer
			fc.ProxyServer = ""
			fc.AuthServers = nil
		} else {
			fc.AuthServers = []string{req.AuthServer}
		}
		fc.CAPin = nil
	}
	if len(req.CAPins) > 0 {
		fc.CAPin = apiutils.Strings(slices.Clone(req.CAPins))
	}

	if req.Token != "" {
		fc.JoinParams.TokenName = req.Token
		fc.AuthToken = ""
	}
	if req.RegistrationSecret != "" {
		fc.JoinParams.BoundKeypair.RegistrationSecretValue = req.RegistrationSecret
		fc.JoinParams.BoundKeypair.RegistrationSecretPath = ""
	}
	if req.RegistrationSecretPath != "" {
		fc.JoinParams.BoundKeypair.RegistrationSecretPath = req.RegistrationSecretPath
		fc.JoinParams.BoundKeypair.RegistrationSecretValue = ""
	}
	if fc.JoinParams.Method == "" {
		switch {
		case req.RegistrationSecret != "" || req.RegistrationSecretPath != "":
			fc.JoinParams.Method = types.JoinMethodBoundKeypair
		case req.Token != "":
			fc.JoinParams.Method = types.JoinMethodToken
		}
	}

	if req.NodeLabels != "" {
		static, dynamic, err := parseLabels(req.NodeLabels)
		if err != nil {
			return nil, trace.Wrap(err, "parsing --node-labels")
		}
		if len(dynamic) > 0 {
			return nil, trace.BadParameter("--node-labels only accepts static labels")
		}
		if fc.SSH.Labels == nil {
			fc.SSH.Labels = make(map[string]string, len(static))
		}
		maps.Copy(fc.SSH.Labels, static)
	}

	if req.DataDir != "" {
		fc.DataDir = req.DataDir
	}
	if req.PIDFile != "" {
		fc.PIDFile = req.PIDFile
	}
	if req.DiagAddr != "" {
		fc.DiagAddr = req.DiagAddr
	}
	if req.SSHListenAddr != "" {
		fc.SSH.ListenAddress = req.SSHListenAddr
	}
	if req.KubeListenAddr != "" {
		fc.Kube.ListenAddress = req.KubeListenAddr
	}
	if req.MetricsListenAddr != "" {
		fc.Metrics.ListenAddress = req.MetricsListenAddr
	}

	return collisionWarnings(fc, req), nil
}

// collisionWarnings reports input values the caller kept that a second agent
// on the same host would contend for. Only explicitly set values are checked;
// implicit defaults (like SSH's 3022) cannot be detected here.
func collisionWarnings(fc *FileConfig, req ReconfigureRequest) []string {
	var warnings []string
	warn := func(field, value, flag string) {
		warnings = append(warnings, fmt.Sprintf(
			"output keeps %s %q from the input; two agents on one host will collide — pass %s to change it",
			field, value, flag))
	}
	if req.PIDFile == "" && fc.PIDFile != "" {
		warn("pid_file", fc.PIDFile, "--pid-file")
	}
	if req.DiagAddr == "" && fc.DiagAddr != "" {
		warn("diag_addr", fc.DiagAddr, "--diag-addr")
	}
	if req.SSHListenAddr == "" && listenAddrCollides(fc.SSH.Service) {
		warn("ssh_service.listen_addr", fc.SSH.ListenAddress, "--ssh-listen-addr")
	}
	if req.KubeListenAddr == "" && listenAddrCollides(fc.Kube.Service) {
		warn("kubernetes_service.listen_addr", fc.Kube.ListenAddress, "--kube-listen-addr")
	}
	if req.MetricsListenAddr == "" && listenAddrCollides(fc.Metrics.Service) {
		warn("metrics_service.listen_addr", fc.Metrics.ListenAddress, "--metrics-listen-addr")
	}
	return warnings
}

// listenAddrCollides reports whether a service pins a listen address a second
// agent would contend for: explicitly set, on an enabled service.
func listenAddrCollides(s Service) bool {
	return s.ListenAddress != "" && s.Enabled()
}
