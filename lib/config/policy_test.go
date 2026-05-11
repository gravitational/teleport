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
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/service/servicecfg"
)

const policyYAML = `
version: v3
teleport:
  data_dir: /tmp/data
  nodename: test
  auth_server: "127.0.0.1:3025"
auth_service:
  enabled: false
proxy_service:
  enabled: false
ssh_service:
  enabled: false
app_service:
  enabled: true
  policies:
    - name: read-only
      allow:
        - paths: ["/api/public/**"]
          methods: [GET, HEAD]
    - name: own-subtree
      allow:
        - paths: ["/api/users/{username}/**"]
          where: 'path.username == user.name'
    - name: no-admin
      deny:
        - paths: ["/admin/**"]
          reason_code: admin_blocked
          reason: "Admin endpoints are not exposed via Teleport."
  apps:
    - name: app-with-named-policies
      uri: "http://localhost:18080"
      public_addr: "app.t.tp"
      policies: [read-only, own-subtree, no-admin]
    - name: app-with-inline-policy
      uri: "http://localhost:18081"
      public_addr: "app2.t.tp"
      policies:
        - name: inline-read
          allow:
            - paths: ["/health"]
              methods: [GET]
    - name: app-without-policies
      uri: "http://localhost:18082"
      public_addr: "app3.t.tp"
`

func TestApplyAppsConfig_Policies(t *testing.T) {
	conf, err := ReadConfig(bytes.NewBufferString(policyYAML))
	require.NoError(t, err)

	cfg := servicecfg.MakeDefaultConfig()
	require.NoError(t, ApplyFileConfig(conf, cfg))

	require.True(t, cfg.Apps.Enabled)
	require.Len(t, cfg.Apps.Apps, 3)

	named := cfg.Apps.Apps[0]
	require.Equal(t, "app-with-named-policies", named.Name)
	require.Len(t, named.Policies, 3)
	require.Equal(t, "read-only", named.Policies[0].Name)
	require.Equal(t, "own-subtree", named.Policies[1].Name)
	require.Equal(t, "no-admin", named.Policies[2].Name)
	require.Len(t, named.Policies[2].Deny, 1)
	require.Equal(t, "admin_blocked", named.Policies[2].Deny[0].ReasonCode)

	inline := cfg.Apps.Apps[1]
	require.Equal(t, "app-with-inline-policy", inline.Name)
	require.Len(t, inline.Policies, 1)
	require.Equal(t, "inline-read", inline.Policies[0].Name)

	without := cfg.Apps.Apps[2]
	require.Equal(t, "app-without-policies", without.Name)
	require.Empty(t, without.Policies)
}

const badYAMLReserved = `
version: v3
teleport:
  data_dir: /tmp/data
  nodename: test
  auth_server: "127.0.0.1:3025"
auth_service:
  enabled: false
proxy_service:
  enabled: false
ssh_service:
  enabled: false
app_service:
  enabled: true
  policies:
    - name: bad
      allow:
        - paths: ["/foo"]
          reason_code: teleport_custom
  apps:
    - name: a
      uri: "http://localhost:18080"
      public_addr: "a.t.tp"
      policies: [bad]
`

func TestApplyAppsConfig_RejectsReservedPrefix(t *testing.T) {
	conf, err := ReadConfig(bytes.NewBufferString(badYAMLReserved))
	require.NoError(t, err)

	cfg := servicecfg.MakeDefaultConfig()
	err = ApplyFileConfig(conf, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "teleport_")
}

const mixedYAML = `
version: v3
teleport:
  data_dir: /tmp/data
  nodename: test
  auth_server: "127.0.0.1:3025"
auth_service:
  enabled: false
proxy_service:
  enabled: false
ssh_service:
  enabled: false
app_service:
  enabled: true
  policies:
    - name: mixed
      allow:
        - paths: ["/foo"]
      deny:
        - paths: ["/bar"]
  apps:
    - name: a
      uri: "http://localhost:18080"
      public_addr: "a.t.tp"
      policies: [mixed]
`

func TestApplyAppsConfig_AcceptsMixedKinds(t *testing.T) {
	conf, err := ReadConfig(bytes.NewBufferString(mixedYAML))
	require.NoError(t, err)

	cfg := servicecfg.MakeDefaultConfig()
	require.NoError(t, ApplyFileConfig(conf, cfg))
	require.Len(t, cfg.Apps.Apps, 1)
	pols := cfg.Apps.Apps[0].Policies
	require.Len(t, pols, 1)
	require.Len(t, pols[0].Allow, 1)
	require.Len(t, pols[0].Deny, 1)
}

const badYAMLUnknownRef = `
version: v3
teleport:
  data_dir: /tmp/data
  nodename: test
  auth_server: "127.0.0.1:3025"
auth_service:
  enabled: false
proxy_service:
  enabled: false
ssh_service:
  enabled: false
app_service:
  enabled: true
  apps:
    - name: a
      uri: "http://localhost:18080"
      public_addr: "a.t.tp"
      policies: [missing]
`

func TestApplyAppsConfig_RejectsUnknownPolicyRef(t *testing.T) {
	conf, err := ReadConfig(bytes.NewBufferString(badYAMLUnknownRef))
	require.NoError(t, err)

	cfg := servicecfg.MakeDefaultConfig()
	err = ApplyFileConfig(conf, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown policy")
}

const tcpAppYAML = `
version: v3
teleport:
  data_dir: /tmp/data
  nodename: test
  auth_server: "127.0.0.1:3025"
auth_service:
  enabled: false
proxy_service:
  enabled: false
ssh_service:
  enabled: false
app_service:
  enabled: true
  policies:
    - name: p
      allow:
        - paths: ["/foo"]
  apps:
    - name: a
      uri: "tcp://localhost"
      public_addr: "a.t.tp"
      tcp_ports:
        - port: 5432
      policies: [p]
`

func TestApplyAppsConfig_RejectsPoliciesOnTCPApp(t *testing.T) {
	conf, err := ReadConfig(bytes.NewBufferString(tcpAppYAML))
	require.NoError(t, err)

	cfg := servicecfg.MakeDefaultConfig()
	err = ApplyFileConfig(conf, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "TCP apps")
}

const mcpAppYAML = `
version: v3
teleport:
  data_dir: /tmp/data
  nodename: test
  auth_server: "127.0.0.1:3025"
auth_service:
  enabled: false
proxy_service:
  enabled: false
ssh_service:
  enabled: false
app_service:
  enabled: true
  policies:
    - name: p
      allow:
        - paths: ["/foo"]
  apps:
    - name: a
      uri: "mcp+stdio://"
      public_addr: "a.t.tp"
      mcp:
        command: "/usr/bin/true"
        run_as_host_user: nobody
      policies: [p]
`

func TestApplyAppsConfig_RejectsPoliciesOnMCPApp(t *testing.T) {
	conf, err := ReadConfig(bytes.NewBufferString(mcpAppYAML))
	require.NoError(t, err)

	cfg := servicecfg.MakeDefaultConfig()
	err = ApplyFileConfig(conf, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "MCP apps")
}

const llmAppYAML = `
version: v3
teleport:
  data_dir: /tmp/data
  nodename: test
  auth_server: "127.0.0.1:3025"
auth_service:
  enabled: false
proxy_service:
  enabled: false
ssh_service:
  enabled: false
app_service:
  enabled: true
  policies:
    - name: p
      allow:
        - paths: ["/foo"]
  apps:
    - name: a
      uri: "https://api.openai.com"
      public_addr: "a.t.tp"
      inference:
        provider: openai
        format: openai
      policies: [p]
`

func TestApplyAppsConfig_RejectsPoliciesOnLLMApp(t *testing.T) {
	conf, err := ReadConfig(bytes.NewBufferString(llmAppYAML))
	require.NoError(t, err)

	cfg := servicecfg.MakeDefaultConfig()
	err = ApplyFileConfig(conf, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "LLM apps")
}

const noPublicAddrYAML = `
version: v3
teleport:
  data_dir: /tmp/data
  nodename: test
  auth_server: "127.0.0.1:3025"
auth_service:
  enabled: false
proxy_service:
  enabled: false
ssh_service:
  enabled: false
app_service:
  enabled: true
  policies:
    - name: p
      allow:
        - paths: ["/foo"]
  apps:
    - name: a
      uri: "http://localhost:8080"
      policies: [p]
`

// TestApplyAppsConfig_RejectsPoliciesWithoutPublicAddr locks in the
// fix for the bot-flagged bypass: public_addr is the lookup key the
// connections handler uses at request time. If it is empty here,
// resolution happens later on the agent side and the policy map
// keyed on `app.PublicAddr` silently misses, skipping the gate.
func TestApplyAppsConfig_RejectsPoliciesWithoutPublicAddr(t *testing.T) {
	conf, err := ReadConfig(bytes.NewBufferString(noPublicAddrYAML))
	require.NoError(t, err)

	cfg := servicecfg.MakeDefaultConfig()
	err = ApplyFileConfig(conf, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "public_addr")
}
