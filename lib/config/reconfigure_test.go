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

// proxyEnabledGuardMessage is the error for --proxy on a config whose proxy
// service is enabled, explicitly or by default.
const proxyEnabledGuardMessage = "--proxy requires a config with proxy_service disabled; the agent rejects proxy_server when the proxy service is enabled"

func TestReconfigureRetargeting(t *testing.T) {
	t.Run("proxy retargets v3 and clears stale fields", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3)
		warnings, err := Reconfigure(fc, ReconfigureRequest{Proxy: "new.example.com:443"})
		require.NoError(t, err)
		require.Empty(t, warnings)
		require.Equal(t, "new.example.com:443", fc.ProxyServer)
		require.Empty(t, fc.AuthServer)
		require.Empty(t, fc.AuthServers)
		require.Empty(t, fc.CAPin)
		requireRoundTrips(t, fc)
	})

	t.Run("proxy with ca-pin sets the new pins", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3)
		_, err := Reconfigure(fc, ReconfigureRequest{
			Proxy:  "new.example.com:443",
			CAPins: []string{"sha256:bbb", "sha256:ccc"},
		})
		require.NoError(t, err)
		require.Equal(t, apiutils.Strings{"sha256:bbb", "sha256:ccc"}, fc.CAPin)
		requireRoundTrips(t, fc)
	})

	t.Run("ca-pin alone replaces pins and preserves addressing", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3)
		_, err := Reconfigure(fc, ReconfigureRequest{CAPins: []string{"sha256:bbb"}})
		require.NoError(t, err)
		require.Equal(t, apiutils.Strings{"sha256:bbb"}, fc.CAPin)
		require.Equal(t, "old.example.com:443", fc.ProxyServer)
		requireRoundTrips(t, fc)
	})

	t.Run("auth-server on v3 sets auth_server and clears proxy_server", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3)
		_, err := Reconfigure(fc, ReconfigureRequest{AuthServer: "new.example.com:3025"})
		require.NoError(t, err)
		require.Equal(t, "new.example.com:3025", fc.AuthServer)
		require.Empty(t, fc.ProxyServer)
		require.Empty(t, fc.AuthServers)
		require.Empty(t, fc.CAPin)
		requireRoundTrips(t, fc)
	})

	t.Run("auth-server on v2 replaces auth_servers with the single address", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV2)
		_, err := Reconfigure(fc, ReconfigureRequest{AuthServer: "new.example.com:3025"})
		require.NoError(t, err)
		require.Equal(t, []string{"new.example.com:3025"}, fc.AuthServers)
		require.Empty(t, fc.CAPin)
		requireRoundTrips(t, fc)
	})

	t.Run("config without a version takes auth-server as v1", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureNoVersion)
		_, err := Reconfigure(fc, ReconfigureRequest{AuthServer: "new.example.com:3025"})
		require.NoError(t, err)
		require.Equal(t, []string{"new.example.com:3025"}, fc.AuthServers)
		requireRoundTrips(t, fc)
	})

	t.Run("proxy on v2 errors and points at auth-server", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV2)
		_, err := Reconfigure(fc, ReconfigureRequest{Proxy: "new.example.com:443"})
		require.ErrorContains(t, err, "--proxy requires a v3 config; use --auth-server for v1/v2 configs")
	})

	t.Run("config without a version rejects --proxy", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureNoVersion)
		_, err := Reconfigure(fc, ReconfigureRequest{Proxy: "new.example.com:443"})
		require.ErrorContains(t, err, "--proxy requires a v3 config; use --auth-server for v1/v2 configs")
	})

	t.Run("proxy on a config with proxy_service enabled errors", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureProxyServiceEnabled)
		_, err := Reconfigure(fc, ReconfigureRequest{Proxy: "new.example.com:443"})
		require.ErrorContains(t, err, proxyEnabledGuardMessage)
	})

	t.Run("proxy on a config with no proxy_service section errors (default enabled)", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureNoProxyServiceSection)
		_, err := Reconfigure(fc, ReconfigureRequest{Proxy: "new.example.com:443"})
		require.ErrorContains(t, err, proxyEnabledGuardMessage)
	})

	t.Run("proxy and auth-server are mutually exclusive", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3)
		_, err := Reconfigure(fc, ReconfigureRequest{
			Proxy:      "new.example.com:443",
			AuthServer: "new.example.com:3025",
		})
		require.ErrorContains(t, err, "--proxy and --auth-server are mutually exclusive")
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
	t.Run("token sets token_name, clears legacy auth_token, infers method token", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureLegacyToken)
		_, err := Reconfigure(fc, ReconfigureRequest{Token: "new-token"})
		require.NoError(t, err)
		require.Equal(t, "new-token", fc.JoinParams.TokenName)
		require.Empty(t, fc.AuthToken)
		require.Equal(t, types.JoinMethodToken, fc.JoinParams.Method)
		requireRoundTrips(t, fc)
	})

	t.Run("non-empty method is preserved", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureEC2Method)
		_, err := Reconfigure(fc, ReconfigureRequest{Token: "new-token"})
		require.NoError(t, err)
		require.Equal(t, types.JoinMethodEC2, fc.JoinParams.Method)
		requireRoundTrips(t, fc)
	})

	t.Run("registration-secret sets value and clears path", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBoundKeypairPath)
		_, err := Reconfigure(fc, ReconfigureRequest{RegistrationSecret: "s3cret"})
		require.NoError(t, err)
		require.Equal(t, "s3cret", fc.JoinParams.BoundKeypair.RegistrationSecretValue)
		require.Empty(t, fc.JoinParams.BoundKeypair.RegistrationSecretPath)
		require.Equal(t, types.JoinMethodBoundKeypair, fc.JoinParams.Method)
		requireRoundTrips(t, fc)
	})

	t.Run("registration-secret wins method inference over token", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureNoMethod)
		_, err := Reconfigure(fc, ReconfigureRequest{Token: "bk-token", RegistrationSecret: "s3cret"})
		require.NoError(t, err)
		require.Equal(t, types.JoinMethodBoundKeypair, fc.JoinParams.Method)
		requireRoundTrips(t, fc)
	})

	t.Run("registration-secret-path sets path and clears value", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBoundKeypairValue)
		_, err := Reconfigure(fc, ReconfigureRequest{RegistrationSecretPath: "/etc/teleport-secret"})
		require.NoError(t, err)
		require.Equal(t, "/etc/teleport-secret", fc.JoinParams.BoundKeypair.RegistrationSecretPath)
		require.Empty(t, fc.JoinParams.BoundKeypair.RegistrationSecretValue)
		requireRoundTrips(t, fc)
	})

	t.Run("registration-secret-path infers bound_keypair method", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureNoMethod)
		_, err := Reconfigure(fc, ReconfigureRequest{RegistrationSecretPath: "/etc/teleport-secret"})
		require.NoError(t, err)
		require.Equal(t, types.JoinMethodBoundKeypair, fc.JoinParams.Method)
		requireRoundTrips(t, fc)
	})

	t.Run("registration-secret and registration-secret-path are mutually exclusive", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3)
		_, err := Reconfigure(fc, ReconfigureRequest{
			RegistrationSecret:     "s3cret",
			RegistrationSecretPath: "/etc/teleport-secret",
		})
		require.ErrorContains(t, err, "--registration-secret and --registration-secret-path are mutually exclusive")
	})
}

func TestReconfigureLabelsAndFields(t *testing.T) {
	t.Run("node-labels merge additively, new value wins", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3) // ssh_service has labels: {env: prod}
		_, err := Reconfigure(fc, ReconfigureRequest{NodeLabels: "team=a,env=dev"})
		require.NoError(t, err)
		require.Equal(t, map[string]string{"env": "dev", "team": "a"}, fc.SSH.Labels)
		requireRoundTrips(t, fc)
	})

	t.Run("node-labels merge into a config with no labels", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureLegacyToken) // no ssh_service section
		_, err := Reconfigure(fc, ReconfigureRequest{NodeLabels: "team=a"})
		require.NoError(t, err)
		require.Equal(t, map[string]string{"team": "a"}, fc.SSH.Labels)
		requireRoundTrips(t, fc)
	})

	t.Run("malformed node-labels error", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3)
		_, err := Reconfigure(fc, ReconfigureRequest{NodeLabels: "not-a-pair"})
		require.Error(t, err)
	})

	t.Run("dynamic labels are rejected", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3)
		_, err := Reconfigure(fc, ReconfigureRequest{NodeLabels: `up=[1m:"uptime"]`})
		require.ErrorContains(t, err, "--node-labels only accepts static labels")
	})

	t.Run("plain fields set only when requested", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3)
		_, err := Reconfigure(fc, ReconfigureRequest{
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
		_, err := Reconfigure(fc, ReconfigureRequest{Proxy: "new.example.com:443"})
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
		warnings, err := Reconfigure(fc, ReconfigureRequest{})
		require.NoError(t, err)
		require.Empty(t, warnings)
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
			Proxy:      "new.example.com:443",
			Token:      "new-token",
			NodeLabels: "team=a",
			DataDir:    "/var/lib/teleport_new",
			PIDFile:    "/run/teleport_new.pid",
		}
		fc := parseTestConfig(t, reconfigureBaseV3)
		warnings1, err := Reconfigure(fc, req)
		require.NoError(t, err)
		out1, err := fc.YAMLString()
		require.NoError(t, err)

		fc2 := parseTestConfig(t, out1)
		warnings2, err := Reconfigure(fc2, req)
		require.NoError(t, err)
		out2, err := fc2.YAMLString()
		require.NoError(t, err)

		require.Equal(t, out1, out2)
		require.Equal(t, warnings1, warnings2)
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

func TestReconfigureCollisionWarnings(t *testing.T) {
	t.Run("all colliding fields kept from input warn", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureCollisions)
		warnings, err := Reconfigure(fc, ReconfigureRequest{Proxy: "new.example.com:443"})
		require.NoError(t, err)
		require.Equal(t, []string{
			`output keeps pid_file "/var/run/teleport.pid" from the input; two agents on one host will collide — pass --pid-file to change it`,
			`output keeps diag_addr "127.0.0.1:3000" from the input; two agents on one host will collide — pass --diag-addr to change it`,
			`output keeps ssh_service.listen_addr "0.0.0.0:3022" from the input; two agents on one host will collide — pass --ssh-listen-addr to change it`,
			`output keeps kubernetes_service.listen_addr "0.0.0.0:3027" from the input; two agents on one host will collide — pass --kube-listen-addr to change it`,
			`output keeps metrics_service.listen_addr "127.0.0.1:3080" from the input; two agents on one host will collide — pass --metrics-listen-addr to change it`,
		}, warnings)
	})

	t.Run("overridden fields do not warn", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureCollisions)
		warnings, err := Reconfigure(fc, ReconfigureRequest{
			PIDFile:           "/run/teleport_new.pid",
			DiagAddr:          "127.0.0.1:3001",
			SSHListenAddr:     "0.0.0.0:3122",
			KubeListenAddr:    "0.0.0.0:3127",
			MetricsListenAddr: "127.0.0.1:3081",
		})
		require.NoError(t, err)
		require.Empty(t, warnings)
	})

	t.Run("explicitly disabled service does not warn on listen_addr", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureDisabledSSHListen)
		warnings, err := Reconfigure(fc, ReconfigureRequest{Proxy: "new.example.com:443"})
		require.NoError(t, err)
		require.Empty(t, warnings)
	})

	t.Run("default-disabled service does not warn on listen_addr", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureDefaultDisabledKubeListen)
		warnings, err := Reconfigure(fc, ReconfigureRequest{Proxy: "new.example.com:443"})
		require.NoError(t, err)
		require.Empty(t, warnings)
	})

	t.Run("config with no colliding fields does not warn", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3)
		warnings, err := Reconfigure(fc, ReconfigureRequest{Proxy: "new.example.com:443"})
		require.NoError(t, err)
		require.Empty(t, warnings)
	})

	// Exact warning texts are pinned above; here we only care that the kind
	// of mutation does not gate the warnings.
	t.Run("warnings fire when only unrelated fields change", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureCollisions)
		warnings, err := Reconfigure(fc, ReconfigureRequest{DataDir: "/var/lib/teleport_new"})
		require.NoError(t, err)
		require.Len(t, warnings, 5)
		for i, field := range []string{
			"pid_file",
			"diag_addr",
			"ssh_service.listen_addr",
			"kubernetes_service.listen_addr",
			"metrics_service.listen_addr",
		} {
			require.Contains(t, warnings[i], field)
		}
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
			req:     ReconfigureRequest{Proxy: "http://bad::addr::443"},
			wantErr: "parsing --proxy",
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
			_, err := Reconfigure(fc, tc.req)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}

	t.Run("diag-addr without a port is accepted", func(t *testing.T) {
		fc := parseTestConfig(t, reconfigureBaseV3)
		_, err := Reconfigure(fc, ReconfigureRequest{DiagAddr: "127.0.0.1"})
		require.NoError(t, err)
		require.Equal(t, "127.0.0.1", fc.DiagAddr)
		requireRoundTrips(t, fc)
	})
}
