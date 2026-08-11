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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

// Fixtures shared across the test functions below. Single-use fixtures sit
// next to the test function that consumes them.

const reconfigureBaseV3 = `version: v3
teleport:
  nodename: node-04
  data_dir: /var/lib/teleport
  proxy_server: old.example.com:443
  ca_pin: ["sha256:aaa"]
  join_params:
    token_name: old-token
    method: token
proxy_service:
  enabled: no
ssh_service:
  enabled: yes
  labels:
    env: prod
`

const reconfigureLegacyToken = `version: v3
teleport:
  data_dir: /var/lib/teleport
  proxy_server: old.example.com:443
  auth_token: legacy-token
`

func parseTestConfig(t *testing.T, raw string) *FileConfig {
	t.Helper()
	fc, err := ReadConfig(strings.NewReader(raw))
	require.NoError(t, err)
	return fc
}

// requireRoundTrips asserts the mutated config is accepted by the same
// reader the agent uses at start.
func requireRoundTrips(t *testing.T, fc *FileConfig) {
	t.Helper()
	out, err := fc.YAMLString()
	require.NoError(t, err)
	_, err = ReadConfig(strings.NewReader(out))
	require.NoError(t, err)
}

const reconfigureBaseV2 = `version: v2
teleport:
  nodename: node-04
  data_dir: /var/lib/teleport
  auth_servers: ["old.example.com:3025"]
  ca_pin: ["sha256:aaa"]
`

// Old configs often omit version:; ReadConfig's CheckAndSetDefaults stamps an
// absent version as v1, so this exercises the v1/v2 path.
const reconfigureNoVersion = `teleport:
  data_dir: /var/lib/teleport
  auth_servers: ["old.example.com:3025"]
`

const reconfigureProxyServiceEnabled = `version: v3
teleport:
  data_dir: /var/lib/teleport
  proxy_server: old.example.com:443
proxy_service:
  enabled: yes
`

const reconfigureNoProxyServiceSection = `version: v3
teleport:
  data_dir: /var/lib/teleport
  proxy_server: old.example.com:443
`

// proxyEnabledGuardMessage is the error for --proxy-server on a config whose proxy
// service is enabled, explicitly or by default.
const proxyEnabledGuardMessage = "--proxy-server cannot be specified if the proxy_service is enabled"

func TestReconfigureRetargeting(t *testing.T) {
	// Retargeting a pinned config requires new pins; see TestReconfigureCAPinGuard.
	t.Run("proxy retargets v3 and clears stale fields", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3)
		err := Reconfigure(fc, ReconfigureRequest{
			ProxyServer: "new.example.com:443",
			CAPins:      []string{"sha256:bbb"},
		})
		require.NoError(t, err)
		require.Equal(t, "new.example.com:443", fc.ProxyServer)
		require.Empty(t, fc.AuthServer)
		require.Empty(t, fc.AuthServers)
		requireRoundTrips(t, fc)
	})

	t.Run("proxy with ca-pin sets the new pins", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3)
		err := Reconfigure(fc, ReconfigureRequest{
			ProxyServer: "new.example.com:443",
			CAPins:      []string{"sha256:bbb", "sha256:ccc"},
		})
		require.NoError(t, err)
		require.Equal(t, apiutils.Strings{"sha256:bbb", "sha256:ccc"}, fc.CAPin)
		requireRoundTrips(t, fc)
	})

	t.Run("ca-pin alone replaces pins and preserves addressing", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3)
		err := Reconfigure(fc, ReconfigureRequest{CAPins: []string{"sha256:bbb"}})
		require.NoError(t, err)
		require.Equal(t, apiutils.Strings{"sha256:bbb"}, fc.CAPin)
		require.Equal(t, "old.example.com:443", fc.ProxyServer)
		requireRoundTrips(t, fc)
	})

	t.Run("auth-server on v3 sets auth_server and clears proxy_server", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3)
		err := Reconfigure(fc, ReconfigureRequest{
			AuthServer: "new.example.com:3025",
			CAPins:     []string{"sha256:bbb"},
		})
		require.NoError(t, err)
		require.Equal(t, "new.example.com:3025", fc.AuthServer)
		require.Empty(t, fc.ProxyServer)
		require.Empty(t, fc.AuthServers)
		requireRoundTrips(t, fc)
	})

	t.Run("auth-server on v2 replaces auth_servers with the single address", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV2)
		err := Reconfigure(fc, ReconfigureRequest{
			AuthServer: "new.example.com:3025",
			CAPins:     []string{"sha256:bbb"},
		})
		require.NoError(t, err)
		require.Equal(t, []string{"new.example.com:3025"}, fc.AuthServers)
		requireRoundTrips(t, fc)
	})

	t.Run("config without a version takes auth-server as v1", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureNoVersion)
		err := Reconfigure(fc, ReconfigureRequest{AuthServer: "new.example.com:3025"})
		require.NoError(t, err)
		require.Equal(t, []string{"new.example.com:3025"}, fc.AuthServers)
		requireRoundTrips(t, fc)
	})

	t.Run("proxy on v2 errors and points at auth-server", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV2)
		err := Reconfigure(fc, ReconfigureRequest{ProxyServer: "new.example.com:443"})
		require.ErrorContains(t, err, "--proxy-server requires a v3 config; use --auth-server for v1/v2 configs")
	})

	t.Run("config without a version rejects --proxy-server", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureNoVersion)
		err := Reconfigure(fc, ReconfigureRequest{ProxyServer: "new.example.com:443"})
		require.ErrorContains(t, err, "--proxy-server requires a v3 config; use --auth-server for v1/v2 configs")
	})

	t.Run("proxy on a config with proxy_service enabled errors", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureProxyServiceEnabled)
		err := Reconfigure(fc, ReconfigureRequest{ProxyServer: "new.example.com:443"})
		require.ErrorContains(t, err, proxyEnabledGuardMessage)
	})

	t.Run("proxy on a config with no proxy_service section errors (default enabled)", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureNoProxyServiceSection)
		err := Reconfigure(fc, ReconfigureRequest{ProxyServer: "new.example.com:443"})
		require.ErrorContains(t, err, proxyEnabledGuardMessage)
	})

	t.Run("proxy and auth-server are mutually exclusive", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3)
		err := Reconfigure(fc, ReconfigureRequest{
			ProxyServer: "new.example.com:443",
			AuthServer:  "new.example.com:3025",
		})
		require.ErrorContains(t, err, "--proxy-server and --auth-server are mutually exclusive")
	})
}

// A pinned input must never come out unpinned by omission: ca_pin is what lets
// a joining agent validate the target cluster's CA, and silently dropping it
// downgrades the next join to unpinned trust.
func TestReconfigureCAPinGuard(t *testing.T) {
	const noVersionWithPin = `teleport:
  data_dir: /var/lib/teleport
  auth_servers: ["old.example.com:3025"]
  ca_pin: ["sha256:aaa"]
`

	t.Run("proxy retarget without ca-pin errors", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3)
		err := Reconfigure(fc, ReconfigureRequest{ProxyServer: "new.example.com:443"})
		require.ErrorContains(t, err, "--ca-pin")
	})

	t.Run("auth-server retarget without ca-pin errors", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3)
		err := Reconfigure(fc, ReconfigureRequest{AuthServer: "new.example.com:3025"})
		require.ErrorContains(t, err, "--ca-pin")
	})

	t.Run("v2 auth-server retarget without ca-pin errors", func(t *testing.T) {
		fc := parseTestConfig(t, noVersionWithPin)
		err := Reconfigure(fc, ReconfigureRequest{AuthServer: "new.example.com:3025"})
		require.ErrorContains(t, err, "--ca-pin")
	})

	// The guard keys off the input's pins, not off retargeting alone.
	t.Run("retarget without ca-pin succeeds when the input has no pins", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureNoVersion)
		err := Reconfigure(fc, ReconfigureRequest{AuthServer: "new.example.com:3025"})
		require.NoError(t, err)
		require.Empty(t, fc.CAPin)
		requireRoundTrips(t, fc)
	})

	t.Run("retarget with ca-pin replaces the input pins", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3)
		err := Reconfigure(fc, ReconfigureRequest{
			ProxyServer: "new.example.com:443",
			CAPins:      []string{"sha256:bbb"},
		})
		require.NoError(t, err)
		require.Equal(t, apiutils.Strings{"sha256:bbb"}, fc.CAPin)
		requireRoundTrips(t, fc)
	})

	// Rotating a token against the same cluster is not a retarget, so it must
	// not demand pins the operator already has in the file.
	t.Run("token change alone preserves the input pins", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3)
		err := Reconfigure(fc, ReconfigureRequest{TokenName: "new-token"})
		require.NoError(t, err)
		require.Equal(t, apiutils.Strings{"sha256:aaa"}, fc.CAPin)
		requireRoundTrips(t, fc)
	})
}

const reconfigureBoundKeypairPath = `version: v3
teleport:
  data_dir: /var/lib/teleport
  proxy_server: old.example.com:443
  join_params:
    token_name: bk-token
    method: bound_keypair
    bound_keypair:
      registration_secret_path: /etc/secret
`

const reconfigureBoundKeypairValue = `version: v3
teleport:
  data_dir: /var/lib/teleport
  proxy_server: old.example.com:443
  join_params:
    token_name: bk-token
    method: bound_keypair
    bound_keypair:
      registration_secret_value: oldsecret
`

const reconfigureNoMethod = `version: v3
teleport:
  data_dir: /var/lib/teleport
  proxy_server: old.example.com:443
  join_params:
    token_name: some-token
`

const reconfigureEC2Method = `version: v3
teleport:
  data_dir: /var/lib/teleport
  proxy_server: old.example.com:443
  join_params:
    token_name: ec2-token
    method: ec2
`

func TestReconfigureJoinParams(t *testing.T) {
	t.Run("token with join-method replaces legacy auth_token", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureLegacyToken)
		err := Reconfigure(fc, ReconfigureRequest{TokenName: "new-token", JoinMethod: "token"})
		require.NoError(t, err)
		require.Equal(t, "new-token", fc.JoinParams.TokenName)
		require.Empty(t, fc.AuthToken)
		require.Equal(t, types.JoinMethodToken, fc.JoinParams.Method)
		requireRoundTrips(t, fc)
	})

	t.Run("token without a method to keep or set errors", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureLegacyToken)
		err := Reconfigure(fc, ReconfigureRequest{TokenName: "new-token"})
		require.ErrorContains(t, err, "--join-method")
	})

	t.Run("join-method omitted keeps the existing method", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureEC2Method)
		err := Reconfigure(fc, ReconfigureRequest{TokenName: "new-token"})
		require.NoError(t, err)
		require.Equal(t, types.JoinMethodEC2, fc.JoinParams.Method)
		require.Equal(t, "new-token", fc.JoinParams.TokenName)
		requireRoundTrips(t, fc)
	})

	t.Run("join-method replaces the existing method", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureEC2Method)
		err := Reconfigure(fc, ReconfigureRequest{TokenName: "new-token", JoinMethod: "token"})
		require.NoError(t, err)
		require.Equal(t, types.JoinMethodToken, fc.JoinParams.Method)
		requireRoundTrips(t, fc)
	})

	t.Run("join-method alone moves legacy auth_token into join_params", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureLegacyToken)
		err := Reconfigure(fc, ReconfigureRequest{JoinMethod: "token"})
		require.NoError(t, err)
		require.Equal(t, "legacy-token", fc.JoinParams.TokenName)
		require.Empty(t, fc.AuthToken)
		require.Equal(t, types.JoinMethodToken, fc.JoinParams.Method)
		requireRoundTrips(t, fc)
	})

	t.Run("join-method on a config with nothing to join with errors", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureNoProxyServiceSection) // no join_params, no auth_token
		err := Reconfigure(fc, ReconfigureRequest{JoinMethod: "ec2"})
		require.ErrorContains(t, err, "--token")
	})

	// A method change away from bound_keypair must not carry the bound_keypair
	// block, and possibly a live secret, into the output.
	t.Run("join-method away from bound_keypair clears the bound_keypair block", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBoundKeypairValue)
		err := Reconfigure(fc, ReconfigureRequest{JoinMethod: "token"})
		require.NoError(t, err)
		require.Equal(t, types.JoinMethodToken, fc.JoinParams.Method)
		require.Equal(t, BoundKeypairParams{}, fc.JoinParams.BoundKeypair)
		require.Equal(t, "bk-token", fc.JoinParams.TokenName)
		requireRoundTrips(t, fc)
	})

	t.Run("unknown join-method errors", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3)
		err := Reconfigure(fc, ReconfigureRequest{JoinMethod: "bogus"})
		require.ErrorContains(t, err, "parsing --join-method")
	})

	t.Run("registration-secret sets value and clears path", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBoundKeypairPath)
		err := Reconfigure(fc, ReconfigureRequest{BoundKeypairRegistrationSecret: "s3cret"})
		require.NoError(t, err)
		require.Equal(t, "s3cret", fc.JoinParams.BoundKeypair.RegistrationSecretValue)
		require.Empty(t, fc.JoinParams.BoundKeypair.RegistrationSecretPath)
		require.Equal(t, types.JoinMethodBoundKeypair, fc.JoinParams.Method)
		requireRoundTrips(t, fc)
	})

	t.Run("registration-secret-path sets path and clears value", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBoundKeypairValue)
		err := Reconfigure(fc, ReconfigureRequest{BoundKeypairRegistrationSecretPath: "/etc/teleport-secret"})
		require.NoError(t, err)
		require.Equal(t, "/etc/teleport-secret", fc.JoinParams.BoundKeypair.RegistrationSecretPath)
		require.Empty(t, fc.JoinParams.BoundKeypair.RegistrationSecretValue)
		requireRoundTrips(t, fc)
	})

	t.Run("registration-secret on a token-method config errors", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3)
		err := Reconfigure(fc, ReconfigureRequest{BoundKeypairRegistrationSecret: "s3cret"})
		require.ErrorContains(t, err, "--join-method bound_keypair")
	})

	t.Run("registration-secret-path on a method-less config errors", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureNoMethod)
		err := Reconfigure(fc, ReconfigureRequest{BoundKeypairRegistrationSecretPath: "/etc/teleport-secret"})
		require.ErrorContains(t, err, "--join-method bound_keypair")
	})

	t.Run("registration-secret with join-method bound_keypair retargets a token config", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3)
		err := Reconfigure(fc, ReconfigureRequest{
			TokenName:                      "bk-token",
			BoundKeypairRegistrationSecret: "s3cret",
			JoinMethod:                     "bound_keypair",
		})
		require.NoError(t, err)
		require.Equal(t, types.JoinMethodBoundKeypair, fc.JoinParams.Method)
		require.Equal(t, "bk-token", fc.JoinParams.TokenName)
		require.Equal(t, "s3cret", fc.JoinParams.BoundKeypair.RegistrationSecretValue)
		requireRoundTrips(t, fc)
	})

	// The agent rejects join_params without a method, so the tool must not
	// write one even when the request does not touch joining.
	t.Run("input join_params without a method errors on any edit", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureNoMethod)
		err := Reconfigure(fc, ReconfigureRequest{DataDir: "/var/lib/teleport_new"})
		require.ErrorContains(t, err, "--join-method")
	})

	t.Run("registration-secret and registration-secret-path are mutually exclusive", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3)
		err := Reconfigure(fc, ReconfigureRequest{
			BoundKeypairRegistrationSecret:     "s3cret",
			BoundKeypairRegistrationSecretPath: "/etc/teleport-secret",
		})
		require.ErrorContains(t, err, "--registration-secret and --registration-secret-path are mutually exclusive")
	})
}

func TestReconfigureLabelsAndFields(t *testing.T) {
	t.Run("node-labels merge additively, new value wins", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3) // ssh_service has labels: {env: prod}
		err := Reconfigure(fc, ReconfigureRequest{NodeLabels: "team=a,env=dev"})
		require.NoError(t, err)
		require.Equal(t, map[string]string{"env": "dev", "team": "a"}, fc.SSH.Labels)
		requireRoundTrips(t, fc)
	})

	t.Run("node-labels merge into a config with no labels", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureLegacyToken) // no ssh_service section
		err := Reconfigure(fc, ReconfigureRequest{NodeLabels: "team=a"})
		require.NoError(t, err)
		require.Equal(t, map[string]string{"team": "a"}, fc.SSH.Labels)
		requireRoundTrips(t, fc)
	})

	t.Run("malformed node-labels error", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3)
		err := Reconfigure(fc, ReconfigureRequest{NodeLabels: "not-a-pair"})
		require.Error(t, err)
	})

	t.Run("dynamic labels are rejected", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3)
		err := Reconfigure(fc, ReconfigureRequest{NodeLabels: `up=[1m:"uptime"]`})
		require.ErrorContains(t, err, "--node-labels only accepts static labels")
	})

	t.Run("plain fields set only when requested", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3)
		err := Reconfigure(fc, ReconfigureRequest{
			DataDir:           "/var/lib/teleport_new",
			PIDFile:           "/run/teleport_new.pid",
			DiagAddr:          "127.0.0.1:3001",
			SSHListenAddr:     "0.0.0.0:3122",
			KubeListenAddr:    "0.0.0.0:3127",
			MetricsListenAddr: "127.0.0.1:3081",
		})
		require.NoError(t, err)
		require.Equal(t, "/var/lib/teleport_new", fc.DataDir)
		require.Equal(t, "/run/teleport_new.pid", fc.PIDFile)
		require.Equal(t, "127.0.0.1:3001", fc.DiagAddr)
		require.Equal(t, "0.0.0.0:3122", fc.SSH.ListenAddress)
		require.Equal(t, "0.0.0.0:3127", fc.Kube.ListenAddress)
		require.Equal(t, "127.0.0.1:3081", fc.Metrics.ListenAddress)
		requireRoundTrips(t, fc)
	})

	t.Run("untouched fields carry through unchanged", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3)
		err := Reconfigure(fc, ReconfigureRequest{
			ProxyServer: "new.example.com:443",
			CAPins:      []string{"sha256:bbb"},
		})
		require.NoError(t, err)
		require.Equal(t, "node-04", fc.NodeName)
		require.Equal(t, "/var/lib/teleport", fc.DataDir)
		require.Equal(t, map[string]string{"env": "prod"}, fc.SSH.Labels)
		require.Equal(t, "old-token", fc.JoinParams.TokenName)
	})
}

func TestReconfigureNoOpAndIdempotency(t *testing.T) {
	t.Run("empty request is a no-op", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3)
		err := Reconfigure(fc, ReconfigureRequest{})
		require.NoError(t, err)
		require.Equal(t, "old.example.com:443", fc.ProxyServer)
		require.Equal(t, apiutils.Strings{"sha256:aaa"}, fc.CAPin)
		require.Equal(t, "old-token", fc.JoinParams.TokenName)
		require.Equal(t, map[string]string{"env": "prod"}, fc.SSH.Labels)
		requireRoundTrips(t, fc)
	})

	// The migration tool re-runs apply on failure, so re-applying the same
	// request to an already-reconfigured config must change nothing.
	t.Run("reconfigure is idempotent", func(t *testing.T) {
		req := ReconfigureRequest{
			ProxyServer: "new.example.com:443",
			CAPins:      []string{"sha256:bbb"},
			TokenName:   "new-token",
			NodeLabels:  "team=a",
			DataDir:     "/var/lib/teleport_new",
			PIDFile:     "/run/teleport_new.pid",
		}
		fc := parseTestConfig(t, reconfigureBaseV3)
		require.NoError(t, Reconfigure(fc, req))
		out1, err := fc.YAMLString()
		require.NoError(t, err)

		fc2 := parseTestConfig(t, out1)
		require.NoError(t, Reconfigure(fc2, req))
		out2, err := fc2.YAMLString()
		require.NoError(t, err)

		require.Equal(t, out1, out2)
	})
}

const reconfigureCollisions = `version: v3
teleport:
  data_dir: /var/lib/teleport
  proxy_server: old.example.com:443
  pid_file: /var/run/teleport.pid
  diag_addr: 127.0.0.1:3000
proxy_service:
  enabled: no
ssh_service:
  enabled: yes
  listen_addr: 0.0.0.0:3022
kubernetes_service:
  enabled: yes
  listen_addr: 0.0.0.0:3027
  kube_cluster_name: prod
metrics_service:
  enabled: yes
  listen_addr: 127.0.0.1:3080
`

const reconfigureDisabledSSHListen = `version: v3
teleport:
  data_dir: /var/lib/teleport
  proxy_server: old.example.com:443
proxy_service:
  enabled: no
ssh_service:
  enabled: no
  listen_addr: 0.0.0.0:3022
`

// kubernetes_service is disabled by default, so a section that pins a
// listen_addr without an enabled flag never binds the port and must not warn.
const reconfigureDefaultDisabledKubeListen = `version: v3
teleport:
  data_dir: /var/lib/teleport
  proxy_server: old.example.com:443
proxy_service:
  enabled: no
kubernetes_service:
  listen_addr: 0.0.0.0:3027
  kube_cluster_name: prod
`

func TestReconfigureCollisionErrors(t *testing.T) {
	t.Run("colliding fields kept from the input error, all listed", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureCollisions)
		err := Reconfigure(fc, ReconfigureRequest{ProxyServer: "new.example.com:443"})
		require.ErrorContains(t, err, "two agents on one host would collide")
		for _, want := range []string{
			`pid_file "/var/run/teleport.pid" (--pid-file)`,
			`diag_addr "127.0.0.1:3000" (--diag-addr)`,
			`ssh_service.listen_addr "0.0.0.0:3022" (--ssh-listen-addr)`,
			`kubernetes_service.listen_addr "0.0.0.0:3027" (--kube-listen-addr)`,
			`metrics_service.listen_addr "127.0.0.1:3080" (--metrics-listen-addr)`,
		} {
			require.ErrorContains(t, err, want)
		}
	})

	t.Run("new values for every colliding field clear the error", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureCollisions)
		err := Reconfigure(fc, ReconfigureRequest{
			PIDFile:           "/run/teleport_new.pid",
			DiagAddr:          "127.0.0.1:3001",
			SSHListenAddr:     "0.0.0.0:3122",
			KubeListenAddr:    "0.0.0.0:3127",
			MetricsListenAddr: "127.0.0.1:3081",
		})
		require.NoError(t, err)
	})

	// Repeating the input's value keeps it on purpose, for configs that
	// replace the input agent rather than run beside it.
	t.Run("repeating the current values clears the error", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureCollisions)
		err := Reconfigure(fc, ReconfigureRequest{
			PIDFile:           "/var/run/teleport.pid",
			DiagAddr:          "127.0.0.1:3000",
			SSHListenAddr:     "0.0.0.0:3022",
			KubeListenAddr:    "0.0.0.0:3027",
			MetricsListenAddr: "127.0.0.1:3080",
		})
		require.NoError(t, err)
	})

	t.Run("explicitly disabled service does not collide on listen_addr", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureDisabledSSHListen)
		err := Reconfigure(fc, ReconfigureRequest{ProxyServer: "new.example.com:443"})
		require.NoError(t, err)
	})

	t.Run("default-disabled service does not collide on listen_addr", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureDefaultDisabledKubeListen)
		err := Reconfigure(fc, ReconfigureRequest{ProxyServer: "new.example.com:443"})
		require.NoError(t, err)
	})

	t.Run("config with no colliding fields does not error", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3)
		err := Reconfigure(fc, ReconfigureRequest{
			ProxyServer: "new.example.com:443",
			CAPins:      []string{"sha256:bbb"},
		})
		require.NoError(t, err)
	})

	// The kind of mutation must not gate the collision check.
	t.Run("collisions error when only unrelated fields change", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureCollisions)
		err := Reconfigure(fc, ReconfigureRequest{DataDir: "/var/lib/teleport_new"})
		require.ErrorContains(t, err, "pid_file")
	})
}

func TestReconfigureAddressValidation(t *testing.T) {
	// Each address flag must reject a malformed value up front, mirroring the
	// parser the agent runs at start.
	for _, tc := range []struct {
		name    string
		req     ReconfigureRequest
		wantErr string
	}{
		{
			name:    "proxy",
			req:     ReconfigureRequest{ProxyServer: "http://bad::addr::443"},
			wantErr: "parsing --proxy-server",
		},
		{
			name:    "auth-server",
			req:     ReconfigureRequest{AuthServer: "http://bad::addr::443"},
			wantErr: "parsing --auth-server",
		},
		{
			name:    "ca-pin",
			req:     ReconfigureRequest{CAPins: []string{"not-a-pin"}},
			wantErr: "parsing --ca-pin",
		},
		{
			name:    "diag-addr",
			req:     ReconfigureRequest{DiagAddr: "http://bad::addr::443"},
			wantErr: "parsing --diag-addr",
		},
		{
			name:    "ssh-listen-addr",
			req:     ReconfigureRequest{SSHListenAddr: "http://bad::addr::443"},
			wantErr: "parsing --ssh-listen-addr",
		},
		{
			name:    "kube-listen-addr",
			req:     ReconfigureRequest{KubeListenAddr: "http://bad::addr::443"},
			wantErr: "parsing --kube-listen-addr",
		},
		{
			name:    "metrics-listen-addr",
			req:     ReconfigureRequest{MetricsListenAddr: "http://bad::addr::443"},
			wantErr: "parsing --metrics-listen-addr",
		},
	} {
		t.Run("invalid "+tc.name+" errors", func(t *testing.T) {
			fc := parseTestConfig(t, reconfigureBaseV3)
			err := Reconfigure(fc, tc.req)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}

	t.Run("diag-addr without a port is accepted", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3)
		err := Reconfigure(fc, ReconfigureRequest{DiagAddr: "127.0.0.1"})
		require.NoError(t, err)
		require.Equal(t, "127.0.0.1", fc.DiagAddr)
		requireRoundTrips(t, fc)
	})
}

func TestReconfigureGoldenOutput(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		req   ReconfigureRequest
	}{
		{
			name:  "v3 proxy retarget",
			input: reconfigureBaseV3,
			req: ReconfigureRequest{
				ProxyServer: "new.example.com:443",
				CAPins:      []string{"sha256:bbb"},
				TokenName:   "new-token",
				NodeLabels:  "env=dev,team=a",
				DataDir:     "/var/lib/teleport_new",
			},
		},
		{
			name:  "v2 auth server retarget",
			input: reconfigureBaseV2,
			req: ReconfigureRequest{
				AuthServer: "new.example.com:3025",
				CAPins:     []string{"sha256:bbb"},
				TokenName:  "new-token",
				JoinMethod: "token",
			},
		},
		{
			// The output must carry no trace of the old bound_keypair block,
			// registration secret included.
			name:  "join method away from bound_keypair",
			input: reconfigureBoundKeypairValue,
			req:   ReconfigureRequest{JoinMethod: "token"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fc := parseTestConfig(t, tc.input)
			require.NoError(t, Reconfigure(fc, tc.req))
			out, err := fc.YAMLString()
			require.NoError(t, err)
			if golden.ShouldSet() {
				golden.Set(t, []byte(out))
			}
			require.Equal(t, string(golden.Get(t)), out)
		})
	}
}
