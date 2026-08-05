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
	"strings"

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
	// CAPins replaces ca_pin with the target cluster's pins. Required when
	// Proxy or AuthServer retargets a config that already pins a CA.
	CAPins []string
	// Token sets join_params.token_name and clears the legacy auth_token.
	Token string
	// JoinMethod sets join_params.method. It is never inferred from the
	// other join fields.
	JoinMethod string
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
// request field untouched. It rejects cases where an unspecified request
// field would cause a collision with a second agent on the same host.
func Reconfigure(fc *FileConfig, req ReconfigureRequest) error {
	if req.Proxy != "" && req.AuthServer != "" {
		return trace.BadParameter("--proxy and --auth-server are mutually exclusive")
	}
	if req.RegistrationSecret != "" && req.RegistrationSecretPath != "" {
		return trace.BadParameter("--registration-secret and --registration-secret-path are mutually exclusive")
	}
	if req.Proxy != "" && fc.Version != defaults.TeleportConfigVersionV3 {
		return trace.BadParameter("--proxy requires a v3 config; use --auth-server for v1/v2 configs")
	}
	if req.Proxy != "" && fc.Proxy.Enabled() {
		return trace.BadParameter("--proxy requires a config with proxy_service disabled; the agent rejects proxy_server when the proxy service is enabled")
	}
	// Mirror the parsers the agent runs at start (applyConfig and the
	// per-service apply functions), or the tool writes a config the agent rejects.
	if req.Proxy != "" {
		if _, err := utils.ParseHostPortAddr(req.Proxy, defaults.HTTPListenPort); err != nil {
			return trace.Wrap(err, "parsing --proxy")
		}
	}
	if req.AuthServer != "" {
		if _, err := utils.ParseHostPortAddr(req.AuthServer, defaults.AuthListenPort); err != nil {
			return trace.Wrap(err, "parsing --auth-server")
		}
	}
	if len(req.CAPins) > 0 {
		// Nil certs: format check only; the pins are verified against the
		// target cluster's CA when the agent joins.
		if err := utils.CheckSPKI(req.CAPins, nil); err != nil {
			return trace.Wrap(err, "parsing --ca-pin")
		}
	}
	if req.JoinMethod != "" {
		if err := types.ValidateJoinMethod(types.JoinMethod(req.JoinMethod)); err != nil {
			return trace.Wrap(err, "parsing --join-method")
		}
	}
	if req.DiagAddr != "" {
		if _, err := utils.ParseAddr(req.DiagAddr); err != nil {
			return trace.Wrap(err, "parsing --diag-addr")
		}
	}
	if req.SSHListenAddr != "" {
		if _, err := utils.ParseHostPortAddr(req.SSHListenAddr, defaults.SSHServerListenPort); err != nil {
			return trace.Wrap(err, "parsing --ssh-listen-addr")
		}
	}
	if req.KubeListenAddr != "" {
		// Not a typo: applyKubeConfig uses the SSH proxy port as kube's default.
		if _, err := utils.ParseHostPortAddr(req.KubeListenAddr, defaults.SSHProxyListenPort); err != nil {
			return trace.Wrap(err, "parsing --kube-listen-addr")
		}
	}
	if req.MetricsListenAddr != "" {
		if _, err := utils.ParseHostPortAddr(req.MetricsListenAddr, defaults.MetricsListenPort); err != nil {
			return trace.Wrap(err, "parsing --metrics-listen-addr")
		}
	}

	// The join method comes from the request or the config, never inference.
	// Reject what the agent would reject at startup (join_params without a
	// method) or silently ignore (a registration secret without bound_keypair).
	joinMethod := fc.JoinParams.Method
	if req.JoinMethod != "" {
		joinMethod = types.JoinMethod(req.JoinMethod)
	}
	if (req.RegistrationSecret != "" || req.RegistrationSecretPath != "") && joinMethod != types.JoinMethodBoundKeypair {
		flag := "--registration-secret"
		if req.RegistrationSecretPath != "" {
			flag = "--registration-secret-path"
		}
		return trace.BadParameter("%s requires the %s join method; pass --join-method %s", flag, types.JoinMethodBoundKeypair, types.JoinMethodBoundKeypair)
	}
	joinParamsInUse := fc.JoinParams != (JoinParams{}) ||
		req.Token != "" || req.RegistrationSecret != "" || req.RegistrationSecretPath != ""
	if joinParamsInUse && joinMethod == "" {
		return trace.BadParameter("the output config would set join_params without a join method, which the agent rejects at startup; pass --join-method")
	}
	if req.JoinMethod != "" && !joinParamsInUse && fc.AuthToken == "" {
		return trace.BadParameter("--join-method would create join_params without a token name; pass --token as well")
	}
	// ca_pin validates the CA of the cluster the agent joins, so pins carried
	// over from the old cluster cannot be reused after a retarget. Requiring
	// new ones keeps a pinned config from silently becoming an unpinned one.
	if (req.Proxy != "" || req.AuthServer != "") && len(fc.CAPin) > 0 && len(req.CAPins) == 0 {
		return trace.BadParameter("the input config sets ca_pin, and retargeting would leave the next join unpinned; pass --ca-pin with the target cluster's pins, which `tctl status` prints")
	}

	if req.Proxy != "" {
		fc.ProxyServer = req.Proxy
		fc.AuthServer = ""
		fc.AuthServers = nil
	}
	if req.AuthServer != "" {
		if fc.Version == defaults.TeleportConfigVersionV3 {
			fc.AuthServer = req.AuthServer
			fc.ProxyServer = ""
			fc.AuthServers = nil
		} else {
			fc.AuthServers = []string{req.AuthServer}
		}
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
	if req.JoinMethod != "" {
		fc.JoinParams.Method = types.JoinMethod(req.JoinMethod)
		// Leaving the bound_keypair block under another method would carry a
		// possibly live registration secret into the output.
		if fc.JoinParams.Method != types.JoinMethodBoundKeypair {
			fc.JoinParams.BoundKeypair = BoundKeypairParams{}
		}
	}
	// The agent rejects a config that sets both join_params and the legacy
	// auth_token.
	if fc.JoinParams != (JoinParams{}) && fc.AuthToken != "" {
		if fc.JoinParams.TokenName == "" {
			fc.JoinParams.TokenName = fc.AuthToken
		}
		fc.AuthToken = ""
	}

	if req.NodeLabels != "" {
		static, dynamic, err := parseLabels(req.NodeLabels)
		if err != nil {
			return trace.Wrap(err, "parsing --node-labels")
		}
		if len(dynamic) > 0 {
			return trace.BadParameter("--node-labels only accepts static labels")
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

	return trace.Wrap(collisionError(fc, req))
}

// collisionError rejects values kept from the input that a second agent on
// the same host would contend for; any explicitly requested value passes.
// Only values the input sets are checked; implicit defaults (like SSH's
// 3022) cannot be detected here.
func collisionError(fc *FileConfig, req ReconfigureRequest) error {
	var collisions []string
	add := func(field, value, flag string) {
		collisions = append(collisions, fmt.Sprintf("%s %q (%s)", field, value, flag))
	}
	if req.PIDFile == "" && fc.PIDFile != "" {
		add("pid_file", fc.PIDFile, "--pid-file")
	}
	if req.DiagAddr == "" && fc.DiagAddr != "" {
		add("diag_addr", fc.DiagAddr, "--diag-addr")
	}
	if req.SSHListenAddr == "" && listenAddrCollides(fc.SSH.Service) {
		add("ssh_service.listen_addr", fc.SSH.ListenAddress, "--ssh-listen-addr")
	}
	if req.KubeListenAddr == "" && listenAddrCollides(fc.Kube.Service) {
		add("kubernetes_service.listen_addr", fc.Kube.ListenAddress, "--kube-listen-addr")
	}
	if req.MetricsListenAddr == "" && listenAddrCollides(fc.Metrics.Service) {
		add("metrics_service.listen_addr", fc.Metrics.ListenAddress, "--metrics-listen-addr")
	}
	if len(collisions) == 0 {
		return nil
	}
	return trace.BadParameter(
		"the output would keep these values from the input, and two agents on one host would collide on them:\n\t%s\npass each listed flag with a new value, or repeat the current value to keep it",
		strings.Join(collisions, "\n\t"))
}

// listenAddrCollides reports whether a service pins a listen address a second
// agent would contend for: explicitly set, on an enabled service.
func listenAddrCollides(s Service) bool {
	return s.ListenAddress != "" && s.Enabled()
}
