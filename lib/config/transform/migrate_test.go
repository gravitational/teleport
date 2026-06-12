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

package transform

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestApplyMigration(t *testing.T) {
	t.Parallel()

	input := []byte(`
version: v3
teleport:
  auth_server: source.example.com:3025
  auth_token: SUPERSECRET-A
  data_dir: /var/lib/teleport
  ca_pin: sha256:abc
  log:
    output: /var/log/teleport.log
    severity: INFO
  storage:
    type: dir
    path: /var/lib/teleport/backend
    keep: /srv/teleport-data
ssh_service:
  enabled: yes
  labels:
    env: prod
app_service:
  enabled: yes
auth_service:
  enabled: yes
proxy_service:
  enabled: yes
`)
	doc, err := Load(input)
	require.NoError(t, err)

	result, err := ApplyMigration(doc, MigrateParams{
		InstallSuffix:   "scope",
		ProxyServer:     "target.example.com:443",
		JoinMethod:      types.JoinMethodToken,
		TokenName:       "scope-migrate-ip-10-2-4-17",
		TokenSecretPath: "/var/run/migrate-token-secret",
		DataDir:         "/var/lib/teleport_scope",
		DisableServices: []string{"app"},
		ExtraSSHLabels:  map[string]string{"scope": "target"},
	})
	require.NoError(t, err)

	renderedBytes, err := result.Document.Render()
	require.NoError(t, err)
	rendered := string(renderedBytes)
	require.NotContains(t, rendered, "SUPERSECRET-A")
	require.NotContains(t, rendered, "auth_token")
	require.Contains(t, rendered, "proxy_server: target.example.com:443")
	require.Contains(t, rendered, "data_dir: /var/lib/teleport_scope")
	require.Contains(t, rendered, "token_secret: /var/run/migrate-token-secret")
	require.Contains(t, rendered, "auth_service:\n  enabled: no")
	require.Contains(t, rendered, "proxy_service:\n  enabled: no")
	require.Contains(t, rendered, "app_service:\n  enabled: no")
	require.Contains(t, rendered, "output: /var/log/teleport_scope.log")
	require.Contains(t, rendered, "scope: target")
	require.Contains(t, rendered, "/var/lib/teleport/backend")
	require.Contains(t, rendered, "keep: /srv/teleport-data")
}

func TestRedactedDiff(t *testing.T) {
	t.Parallel()

	input := []byte(`
version: v3
teleport:
  proxy_server: source.example.com:443
  token: SUPERSECRET-A
  join_params:
    method: token
    token_name: legacy-token-secret-name
    token_secret: SUPERSECRET-B
    bound_keypair:
      registration_secret_value: SUPERSECRET-C
ssh_service:
  enabled: yes
auth_service:
  enabled: no
proxy_service:
  enabled: no
`)
	doc, err := Load(input)
	require.NoError(t, err)
	result, err := ApplyMigration(doc, MigrateParams{
		InstallSuffix:   "scope",
		ProxyServer:     "target.example.com:443",
		JoinMethod:      types.JoinMethodToken,
		TokenName:       "scope-migrate-ip-10-2-4-17",
		TokenSecretPath: "/var/run/migrate-token-secret",
		DataDir:         "/var/lib/teleport_scope",
	})
	require.NoError(t, err)

	diff, err := DiffDocuments(
		doc.Redact(DefaultRedactionRules()),
		result.Document.Redact(DefaultRedactionRules()),
		"input",
		"output",
	)
	require.NoError(t, err)
	require.NotContains(t, diff, "SUPERSECRET-A")
	require.NotContains(t, diff, "SUPERSECRET-B")
	require.NotContains(t, diff, "SUPERSECRET-C")
	require.NotContains(t, diff, "scope-migrate-ip-10-2-4-17")
	require.NotContains(t, diff, types.MaskTokenName("scope-migrate-ip-10-2-4-17"))
	require.Contains(t, diff, "<redacted>")
}

func TestApplyMigrationWarnings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		check func(*testing.T, *MigrationResult)
	}{
		{
			name: "diag addr",
			input: `
version: v3
teleport:
  diag_addr: 127.0.0.1:3000
ssh_service:
  enabled: yes
`,
			check: func(t *testing.T, result *MigrationResult) {
				_, ok := result.Document.Get("teleport", "diag_addr")
				require.False(t, ok)
				require.Contains(t, result.Notices[0], "diag_addr removed")
			},
		},
		{
			name: "service listener",
			input: `
version: v3
teleport: {}
ssh_service:
  enabled: yes
  listen_addr: 0.0.0.0:3022
`,
			check: func(t *testing.T, result *MigrationResult) {
				require.Contains(t, result.ListenerWarnings, `ssh_service.listen_addr "0.0.0.0:3022" may be bound by both agents`)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc, err := Load([]byte(tt.input))
			require.NoError(t, err)
			result, err := ApplyMigration(doc, MigrateParams{
				InstallSuffix:   "scope",
				ProxyServer:     "target.example.com:443",
				JoinMethod:      types.JoinMethodToken,
				TokenName:       "scope-migrate-ip-10-2-4-17",
				TokenSecretPath: "/var/run/migrate-token-secret",
				DataDir:         "/var/lib/teleport_scope",
				ExtraSSHLabels:  map[string]string{"scope": "target"},
			})
			require.NoError(t, err)
			tt.check(t, result)
		})
	}
}

func TestApplyMigrationLabelConflict(t *testing.T) {
	t.Parallel()

	doc, err := Load([]byte(`
version: v3
teleport: {}
ssh_service:
  enabled: yes
  labels:
    scope: old
`))
	require.NoError(t, err)

	_, err = ApplyMigration(doc, MigrateParams{
		InstallSuffix:   "scope",
		ProxyServer:     "target.example.com:443",
		JoinMethod:      types.JoinMethodToken,
		TokenName:       "scope-migrate-ip-10-2-4-17",
		TokenSecretPath: "/var/run/migrate-token-secret",
		DataDir:         "/var/lib/teleport_scope",
		ExtraSSHLabels:  map[string]string{"scope": "target"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "ssh_service.labels.scope")
}

func TestApplyMigrationDisablesKubeService(t *testing.T) {
	t.Parallel()

	doc, err := Load([]byte(`
version: v3
teleport: {}
kubernetes_service:
  enabled: yes
`))
	require.NoError(t, err)

	result, err := ApplyMigration(doc, MigrateParams{
		InstallSuffix:   "scope",
		ProxyServer:     "target.example.com:443",
		JoinMethod:      types.JoinMethodToken,
		TokenName:       "scope-migrate-ip-10-2-4-17",
		TokenSecretPath: "/var/run/migrate-token-secret",
		DataDir:         "/var/lib/teleport_scope",
		DisableServices: []string{"kube"},
	})
	require.NoError(t, err)
	renderedBytes, err := result.Document.Render()
	require.NoError(t, err)
	require.Contains(t, string(renderedBytes), "kubernetes_service:\n  enabled: no")
	require.NotContains(t, string(renderedBytes), "kube_service:")
}

func TestApplyMigrationPIDFileIsSuffixed(t *testing.T) {
	t.Parallel()

	doc, err := Load([]byte(`
version: v3
teleport:
  pid_file: /run/teleport.pid
`))
	require.NoError(t, err)

	result, err := ApplyMigration(doc, MigrateParams{
		InstallSuffix:   "scope",
		ProxyServer:     "target.example.com:443",
		JoinMethod:      types.JoinMethodToken,
		TokenName:       "scope-migrate-ip-10-2-4-17",
		TokenSecretPath: "/var/run/migrate-token-secret",
		DataDir:         "/var/lib/teleport_scope",
	})
	require.NoError(t, err)
	require.NotNil(t, result.PIDFileChanged)
	require.Equal(t, "/run/teleport_scope.pid", result.PIDFileChanged.New)
	pidFile, ok := result.Document.Get("teleport", "pid_file")
	require.True(t, ok)
	require.Equal(t, "/run/teleport_scope.pid", pidFile.Value)
}

func TestRedactStdoutRender(t *testing.T) {
	t.Parallel()

	doc, err := Load([]byte(`
teleport:
  join_params:
    token_name: abcdefghijklmnop
    token_secret: SUPERSECRET
`))
	require.NoError(t, err)
	redactedBytes, err := doc.Redact(DefaultRedactionRules()).Render()
	require.NoError(t, err)
	redacted := string(redactedBytes)
	require.NotContains(t, redacted, "SUPERSECRET")
	require.NotContains(t, redacted, "abcdefghijklmnop")
	require.NotContains(t, redacted, "************mnop")
}
