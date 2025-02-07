/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tbot/botfs"
)

func TestMigrate(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantError  string
		wantOutput *BotConfig
	}{
		{
			name: "very full config",
			input: `
auth_server: example.teleport.sh:443
oneshot: true
debug: true
certificate_ttl: 30m
diag_addr: 127.0.0.1:621
renewal_interval: 10m
onboarding:
  join_method: "token"
  token: "my-token"
  ca_pins:
  - "sha256:my-pin"
storage:
  directory:
    path: /path/storage
    acls: required
    symlinks: secure
destinations:
- directory:
    path: /path/destination
  roles: ["foo"]
  configs:
  - identity: {}
  - ssh_client
  - tls_cas
- memory: true
  app: my-app
  configs:
  - identity: {}
  - tls_cas: {}
  - tls: {}
- memory: true
  app: my-app
- memory: {}
  kubernetes_cluster: my-kubernetes-cluster
  configs:
  - identity
  - tls_cas
  - kubernetes
- memory: {}
  database:
    service: "my-db-service"
    database: "the-db"
    username: "alice"
- memory: {}
  database:
    service: "my-db-service"
  configs:
  - mongo
- memory: {}
  database:
    service: "my-db-service"
  configs:
  - tls
- memory: {}
  database:
    service: "my-db-service"
  configs:
  - cockroach
- memory: {}
  roles: ["foo"]
  configs:
  - ssh_host_cert:
      principals:
      - example.com
      - second.example.com
`,
			wantOutput: &BotConfig{
				Version:    V2,
				AuthServer: "example.teleport.sh:443",
				Oneshot:    true,
				Debug:      true,
				CertificateLifetime: CertificateLifetime{
					RenewalInterval: time.Minute * 10,
					TTL:             time.Minute * 30,
				},
				DiagAddr: "127.0.0.1:621",
				Onboarding: OnboardingConfig{
					JoinMethod: types.JoinMethodToken,
					TokenValue: "my-token",
					CAPins: []string{
						"sha256:my-pin",
					},
				},
				Storage: &StorageConfig{
					Destination: &DestinationDirectory{
						Path:     "/path/storage",
						ACLs:     botfs.ACLRequired,
						Symlinks: botfs.SymlinksSecure,
					},
				},
				Services: ServiceConfigs{
					&IdentityOutput{
						Destination: &DestinationDirectory{
							Path: "/path/destination",
						},
						Roles: []string{"foo"},
					},
					&ApplicationOutput{
						Destination:           &DestinationMemory{},
						AppName:               "my-app",
						SpecificTLSExtensions: true,
					},
					&ApplicationOutput{
						Destination: &DestinationMemory{},
						AppName:     "my-app",
					},
					&KubernetesOutput{
						Destination:       &DestinationMemory{},
						KubernetesCluster: "my-kubernetes-cluster",
					},
					&DatabaseOutput{
						Destination: &DestinationMemory{},
						Service:     "my-db-service",
						Database:    "the-db",
						Username:    "alice",
						Format:      UnspecifiedDatabaseFormat,
					},
					&DatabaseOutput{
						Destination: &DestinationMemory{},
						Service:     "my-db-service",
						Format:      MongoDatabaseFormat,
					},
					&DatabaseOutput{
						Destination: &DestinationMemory{},
						Service:     "my-db-service",
						Format:      TLSDatabaseFormat,
					},
					&DatabaseOutput{
						Destination: &DestinationMemory{},
						Service:     "my-db-service",
						Format:      CockroachDatabaseFormat,
					},
					&SSHHostOutput{
						Destination: &DestinationMemory{},
						Roles:       []string{"foo"},
						Principals:  []string{"example.com", "second.example.com"},
					},
				},
			},
		},
		// Backwards compat with GHA
		{
			name: "backwards compat with @teleport-actions/auth",
			input: `
auth_server: example.teleport.sh:443
oneshot: true
debug: true
onboarding:
  join_method: github
  token: my-token
storage:
  memory: true
destinations:
- directory:
    path: /path/example
    symlinks: try-secure
  roles: []
`,
			wantOutput: &BotConfig{
				Version:    V2,
				AuthServer: "example.teleport.sh:443",
				Oneshot:    true,
				Debug:      true,
				Onboarding: OnboardingConfig{
					JoinMethod: types.JoinMethodGitHub,
					TokenValue: "my-token",
				},
				Storage: &StorageConfig{
					Destination: &DestinationMemory{},
				},
				Services: ServiceConfigs{
					&IdentityOutput{
						Destination: &DestinationDirectory{
							Path:     "/path/example",
							Symlinks: "try-secure",
						},
						Roles: []string{},
					},
				},
			},
		},
		{
			name: "backwards compat with @teleport-actions/auth-k8s",
			input: `
auth_server: example.teleport.sh:443
oneshot: true
debug: true
onboarding:
  join_method: github
  token: my-token
storage:
  memory: true
destinations:
- directory:
    path: /path/example
    symlinks: try-secure
  roles: []
  kubernetes_cluster: my-cluster
`,
			wantOutput: &BotConfig{
				Version:    V2,
				AuthServer: "example.teleport.sh:443",
				Oneshot:    true,
				Debug:      true,
				Onboarding: OnboardingConfig{
					JoinMethod: types.JoinMethodGitHub,
					TokenValue: "my-token",
				},
				Storage: &StorageConfig{
					Destination: &DestinationMemory{},
				},
				Services: ServiceConfigs{
					&KubernetesOutput{
						Destination: &DestinationDirectory{
							Path:     "/path/example",
							Symlinks: "try-secure",
						},
						Roles:             []string{},
						KubernetesCluster: "my-cluster",
					},
				},
			},
		},
		{
			name: "backwards compat with @teleport-actions/auth-application",
			input: `
auth_server: example.teleport.sh:443
oneshot: true
debug: true
onboarding:
  join_method: github
  token: my-token
storage:
  memory: true
destinations:
- directory:
    path: /path/example
    symlinks: try-secure
  roles: []
  app: my-app
`,
			wantOutput: &BotConfig{
				Version:    V2,
				AuthServer: "example.teleport.sh:443",
				Oneshot:    true,
				Debug:      true,
				Onboarding: OnboardingConfig{
					JoinMethod: types.JoinMethodGitHub,
					TokenValue: "my-token",
				},
				Storage: &StorageConfig{
					Destination: &DestinationMemory{},
				},
				Services: ServiceConfigs{
					&ApplicationOutput{
						Destination: &DestinationDirectory{
							Path:     "/path/example",
							Symlinks: "try-secure",
						},
						Roles:   []string{},
						AppName: "my-app",
					},
				},
			},
		},
		// Backwards compat with guides
		{
			name: "backwards compat with https://goteleport.com/docs/enroll-resources/machine-id/deployment/jenkins/",
			input: `
auth_server: "auth.example.com:3025"
onboarding:
  join_method: "token"
  token: "00000000000000000000000000000000"
  ca_pins:
  - "sha256:1111111111111111111111111111111111111111111111111111111111111111"
storage:
  directory: /var/lib/teleport/bot
destinations:
  - directory: /opt/machine-id
`,
			wantOutput: &BotConfig{
				Version:    V2,
				AuthServer: "auth.example.com:3025",
				Onboarding: OnboardingConfig{
					JoinMethod: types.JoinMethodToken,
					TokenValue: "00000000000000000000000000000000",
					CAPins: []string{
						"sha256:1111111111111111111111111111111111111111111111111111111111111111",
					},
				},
				Storage: &StorageConfig{
					Destination: &DestinationDirectory{
						Path: "/var/lib/teleport/bot",
					},
				},
				Services: ServiceConfigs{
					&IdentityOutput{
						Destination: &DestinationDirectory{
							Path: "/opt/machine-id",
						},
					},
				},
			},
		},
		{
			name: "backwards compat with https://goteleport.com/docs/enroll-resources/machine-id/access-guides/databases/",
			input: `
auth_server: "teleport.example.com:443"
onboarding:
  join_method: "token"
  token: "abcd123-insecure-do-not-use-this"
  ca_pins:
  - "sha256:abdc1245efgh5678abdc1245efgh5678abdc1245efgh5678abdc1245efgh5678"
storage:
  directory: /var/lib/teleport/bot
destinations:
  - directory: /opt/machine-id

    database:
      service: example-server
      username: alice
      database: example
`,
			wantOutput: &BotConfig{
				Version:    V2,
				AuthServer: "teleport.example.com:443",
				Onboarding: OnboardingConfig{
					JoinMethod: types.JoinMethodToken,
					TokenValue: "abcd123-insecure-do-not-use-this",
					CAPins: []string{
						"sha256:abdc1245efgh5678abdc1245efgh5678abdc1245efgh5678abdc1245efgh5678",
					},
				},
				Storage: &StorageConfig{
					Destination: &DestinationDirectory{
						Path: "/var/lib/teleport/bot",
					},
				},
				Services: ServiceConfigs{
					&DatabaseOutput{
						Destination: &DestinationDirectory{
							Path: "/opt/machine-id",
						},
						Service:  "example-server",
						Username: "alice",
						Database: "example",
					},
				},
			},
		},
		{
			name: "backwards compat with https://goteleport.com/docs/enroll-resources/machine-id/access-guides/databases/ - mongo",
			input: `
auth_server: "teleport.example.com:443"
onboarding:
  join_method: "token"
  token: "abcd123-insecure-do-not-use-this"
  ca_pins:
  - "sha256:abdc1245efgh5678abdc1245efgh5678abdc1245efgh5678abdc1245efgh5678"
storage:
  directory: /var/lib/teleport/bot
destinations:
  - directory: /opt/machine-id

    database:
      service: example-server
      username: alice
      database: example

    # If using MongoDB, be sure to include the Mongo-formatted certificates:
    configs:
      - mongo
`,
			wantOutput: &BotConfig{
				Version:    V2,
				AuthServer: "teleport.example.com:443",
				Onboarding: OnboardingConfig{
					JoinMethod: types.JoinMethodToken,
					TokenValue: "abcd123-insecure-do-not-use-this",
					CAPins: []string{
						"sha256:abdc1245efgh5678abdc1245efgh5678abdc1245efgh5678abdc1245efgh5678",
					},
				},
				Storage: &StorageConfig{
					Destination: &DestinationDirectory{
						Path: "/var/lib/teleport/bot",
					},
				},
				Services: ServiceConfigs{
					&DatabaseOutput{
						Destination: &DestinationDirectory{
							Path: "/opt/machine-id",
						},
						Format:   MongoDatabaseFormat,
						Service:  "example-server",
						Username: "alice",
						Database: "example",
					},
				},
			},
		},
		{
			name: "backwards compat with https://goteleport.com/docs/enroll-resources/machine-id/access-guides/databases/ - cockroach",
			input: `
auth_server: "teleport.example.com:443"
onboarding:
  join_method: "token"
  token: "abcd123-insecure-do-not-use-this"
  ca_pins:
  - "sha256:abdc1245efgh5678abdc1245efgh5678abdc1245efgh5678abdc1245efgh5678"
storage:
  directory: /var/lib/teleport/bot
destinations:
  - directory: /opt/machine-id

    database:
      service: example-server
      username: alice
      database: example

    configs:
      - cockroach
`,
			wantOutput: &BotConfig{
				Version:    V2,
				AuthServer: "teleport.example.com:443",
				Onboarding: OnboardingConfig{
					JoinMethod: types.JoinMethodToken,
					TokenValue: "abcd123-insecure-do-not-use-this",
					CAPins: []string{
						"sha256:abdc1245efgh5678abdc1245efgh5678abdc1245efgh5678abdc1245efgh5678",
					},
				},
				Storage: &StorageConfig{
					Destination: &DestinationDirectory{
						Path: "/var/lib/teleport/bot",
					},
				},
				Services: ServiceConfigs{
					&DatabaseOutput{
						Destination: &DestinationDirectory{
							Path: "/opt/machine-id",
						},
						Format:   CockroachDatabaseFormat,
						Service:  "example-server",
						Username: "alice",
						Database: "example",
					},
				},
			},
		},
		{
			name: "backwards compat with https://goteleport.com/docs/enroll-resources/machine-id/access-guides/databases/ - tls",
			input: `
auth_server: "teleport.example.com:443"
onboarding:
  join_method: "token"
  token: "abcd123-insecure-do-not-use-this"
  ca_pins:
  - "sha256:abdc1245efgh5678abdc1245efgh5678abdc1245efgh5678abdc1245efgh5678"
storage:
  directory: /var/lib/teleport/bot
destinations:
  - directory: /opt/machine-id

    database:
      service: example-server
      username: alice
      database: example

    configs:
      - tls
`,
			wantOutput: &BotConfig{
				Version:    V2,
				AuthServer: "teleport.example.com:443",
				Onboarding: OnboardingConfig{
					JoinMethod: types.JoinMethodToken,
					TokenValue: "abcd123-insecure-do-not-use-this",
					CAPins: []string{
						"sha256:abdc1245efgh5678abdc1245efgh5678abdc1245efgh5678abdc1245efgh5678",
					},
				},
				Storage: &StorageConfig{
					Destination: &DestinationDirectory{
						Path: "/var/lib/teleport/bot",
					},
				},
				Services: ServiceConfigs{
					&DatabaseOutput{
						Destination: &DestinationDirectory{
							Path: "/opt/machine-id",
						},
						Format:   TLSDatabaseFormat,
						Service:  "example-server",
						Username: "alice",
						Database: "example",
					},
				},
			},
		},
		{
			name: "backwards compat with https://goteleport.com/docs/enroll-resources/machine-id - host-certificate",
			input: `
onboarding:
  token: "1234abcd5678efgh9"
  ca_path: ""
  ca_pins:
  - sha256:1234abcd5678efgh910ijklmnop
  join_method: token
storage:
  directory:
    path: /var/lib/teleport/bot
    symlinks: secure
    acls: try
destinations:
  - directory:
      path: /opt/machine-id
    configs:
      - ssh_host_cert:
          principals: [nodename.my.domain.com]
debug: false
auth_server: example.teleport.sh:443
certificate_ttl: 1h0m0s
renewal_interval: 20m0s
oneshot: false
`,
			wantOutput: &BotConfig{
				Version:    V2,
				AuthServer: "example.teleport.sh:443",
				Onboarding: OnboardingConfig{
					JoinMethod: types.JoinMethodToken,
					TokenValue: "1234abcd5678efgh9",
					CAPins: []string{
						"sha256:1234abcd5678efgh910ijklmnop",
					},
				},
				CertificateLifetime: CertificateLifetime{
					RenewalInterval: DefaultRenewInterval,
					TTL:             DefaultCertificateTTL,
				},
				Storage: &StorageConfig{
					Destination: &DestinationDirectory{
						Path:     "/var/lib/teleport/bot",
						Symlinks: "secure",
						ACLs:     "try",
					},
				},
				Services: ServiceConfigs{
					&SSHHostOutput{
						Destination: &DestinationDirectory{
							Path: "/opt/machine-id",
						},
						Principals: []string{"nodename.my.domain.com"},
					},
				},
			},
		},
		{
			name: "backwards compat with https://goteleport.com/docs/enroll-resources/machine-id/access-guides/applications/",
			input: `
auth_server: "teleport.example.com:443"
onboarding:
  join_method: "token"
  token: "abcd123-insecure-do-not-use-this"
  ca_pins:
  - "sha256:abdc1245efgh5678abdc1245efgh5678abdc1245efgh5678abdc1245efgh5678"
storage:
  directory: /var/lib/teleport/bot
destinations:
  - directory: /opt/machine-id
    app: grafana-example
`,
			wantOutput: &BotConfig{
				Version:    V2,
				AuthServer: "teleport.example.com:443",
				Onboarding: OnboardingConfig{
					JoinMethod: types.JoinMethodToken,
					TokenValue: "abcd123-insecure-do-not-use-this",
					CAPins: []string{
						"sha256:abdc1245efgh5678abdc1245efgh5678abdc1245efgh5678abdc1245efgh5678",
					},
				},
				Storage: &StorageConfig{
					Destination: &DestinationDirectory{
						Path: "/var/lib/teleport/bot",
					},
				},
				Services: ServiceConfigs{
					&ApplicationOutput{
						Destination: &DestinationDirectory{
							Path: "/opt/machine-id",
						},
						AppName: "grafana-example",
					},
				},
			},
		},
		{
			name: "backwards compat with https://goteleport.com/docs/enroll-resources/machine-id/access-guides/applications/ - with tls config",
			input: `
auth_server: "teleport.example.com:443"
onboarding:
  join_method: "token"
  token: "abcd123-insecure-do-not-use-this"
  ca_pins:
  - "sha256:abdc1245efgh5678abdc1245efgh5678abdc1245efgh5678abdc1245efgh5678"
storage:
  directory: /var/lib/teleport/bot
destinations:
  - directory: /opt/machine-id
    app: grafana-example

    configs:
      - tls
`,
			wantOutput: &BotConfig{
				Version:    V2,
				AuthServer: "teleport.example.com:443",
				Onboarding: OnboardingConfig{
					JoinMethod: types.JoinMethodToken,
					TokenValue: "abcd123-insecure-do-not-use-this",
					CAPins: []string{
						"sha256:abdc1245efgh5678abdc1245efgh5678abdc1245efgh5678abdc1245efgh5678",
					},
				},
				Storage: &StorageConfig{
					Destination: &DestinationDirectory{
						Path: "/var/lib/teleport/bot",
					},
				},
				Services: ServiceConfigs{
					&ApplicationOutput{
						Destination: &DestinationDirectory{
							Path: "/opt/machine-id",
						},
						AppName:               "grafana-example",
						SpecificTLSExtensions: true,
					},
				},
			},
		},
		{
			name: "backwards compat with https://goteleport.com/docs/enroll-resources/machine-id/access-guides/kubernetes/",
			input: `
auth_server: "teleport.example.com:443"
onboarding:
  join_method: "token"
  token: "abcd123-insecure-do-not-use-this"
  ca_pins:
  - "sha256:abdc1245efgh5678abdc1245efgh5678abdc1245efgh5678abdc1245efgh5678"
storage:
  directory: /var/lib/teleport/bot
destinations:
  - directory: /opt/machine-id
    kubernetes_cluster: example-k8s-cluster
`,
			wantOutput: &BotConfig{
				Version:    V2,
				AuthServer: "teleport.example.com:443",
				Onboarding: OnboardingConfig{
					JoinMethod: types.JoinMethodToken,
					TokenValue: "abcd123-insecure-do-not-use-this",
					CAPins: []string{
						"sha256:abdc1245efgh5678abdc1245efgh5678abdc1245efgh5678abdc1245efgh5678",
					},
				},
				Storage: &StorageConfig{
					Destination: &DestinationDirectory{
						Path: "/var/lib/teleport/bot",
					},
				},
				Services: ServiceConfigs{
					&KubernetesOutput{
						Destination: &DestinationDirectory{
							Path: "/opt/machine-id",
						},
						KubernetesCluster: "example-k8s-cluster",
					},
				},
			},
		},
		// Niche cases
		{
			name: "no storage config",
			input: `
auth_server: "teleport.example.com:443"
onboarding:
  join_method: "token"
  token: "abcd123-insecure-do-not-use-this"
`,
			wantOutput: &BotConfig{
				Version:    V2,
				AuthServer: "teleport.example.com:443",
				Onboarding: OnboardingConfig{
					JoinMethod: types.JoinMethodToken,
					TokenValue: "abcd123-insecure-do-not-use-this",
				},
				Storage: nil,
				Outputs: nil,
			},
		},
		// Real-world cases
		{
			name: "real world 1",
			input: `
auth_server: "teleport.example.com:443"
onboarding:
  join_method: "iam"
  token: "iam-token-kube"
storage:
  directory:
    path: /var/lib/teleport/bot
    symlinks: insecure
    acls: off
debug: true
destinations:
  - directory:
      path: /opt/machine-id
      symlinks: insecure
      acls: off
  - directory:
      path: /opt/machine-id/tools
      symlinks: insecure
      acls: off
    kubernetes_cluster: "tools"
`,
			wantOutput: &BotConfig{
				Version:    V2,
				AuthServer: "teleport.example.com:443",
				Onboarding: OnboardingConfig{
					JoinMethod: types.JoinMethodIAM,
					TokenValue: "iam-token-kube",
				},
				Storage: &StorageConfig{
					Destination: &DestinationDirectory{
						Path:     "/var/lib/teleport/bot",
						Symlinks: botfs.SymlinksInsecure,
						ACLs:     botfs.ACLOff,
					},
				},
				Debug: true,
				Services: ServiceConfigs{
					&IdentityOutput{
						Destination: &DestinationDirectory{
							Path:     "/opt/machine-id",
							Symlinks: botfs.SymlinksInsecure,
							ACLs:     botfs.ACLOff,
						},
					},
					&KubernetesOutput{
						Destination: &DestinationDirectory{
							Path:     "/opt/machine-id/tools",
							Symlinks: botfs.SymlinksInsecure,
							ACLs:     botfs.ACLOff,
						},
						KubernetesCluster: "tools",
					},
				},
			},
		},
		{
			name: "real world 2",
			input: `
storage:
    directory:
        path: /var/tmp/teleport/bot
        symlinks: insecure

destinations:
    - directory:
          path: /var/tmp/machine-id
          symlinks: insecure
`,
			wantOutput: &BotConfig{
				Version: V2,
				Storage: &StorageConfig{
					Destination: &DestinationDirectory{
						Path:     "/var/tmp/teleport/bot",
						Symlinks: botfs.SymlinksInsecure,
					},
				},
				Services: ServiceConfigs{
					&IdentityOutput{
						Destination: &DestinationDirectory{
							Path:     "/var/tmp/machine-id",
							Symlinks: botfs.SymlinksInsecure,
						},
					},
				},
			},
		},
		{
			name: "real world 3",
			input: `
Machine ID config created at /etc/tbot.yaml:
auth_server: teleportvm.example.com:443
onboarding:
  token: redacted
storage:
  directory: /var/lib/teleport/bot

destinations:
  - directory: /opt/machine-id
    roles: [access]
    database:
      service: self-hosted
      username: alice
      database: Payroll
`,
			wantOutput: &BotConfig{
				Version:    V2,
				AuthServer: "teleportvm.example.com:443",
				Onboarding: OnboardingConfig{
					TokenValue: "redacted",
				},
				Storage: &StorageConfig{
					Destination: &DestinationDirectory{
						Path: "/var/lib/teleport/bot",
					},
				},
				Services: ServiceConfigs{
					&DatabaseOutput{
						Destination: &DestinationDirectory{
							Path: "/opt/machine-id",
						},
						Roles:    []string{"access"},
						Service:  "self-hosted",
						Username: "alice",
						Database: "Payroll",
					},
				},
			},
		},
		{
			name: "real world 4",
			input: `
auth_server: "redacted.teleport.sh:443"
onboarding:
  join_method: "iam"
  token: "redacted-scanner-token"
  ca_pins:
  - "sha256:redacted"
storage:
  directory: /var/lib/teleport/bot
destinations:
  - directory: /opt/machine-id
    kubernetes_cluster: devops
`,
			wantOutput: &BotConfig{
				Version:    V2,
				AuthServer: "redacted.teleport.sh:443",
				Onboarding: OnboardingConfig{
					TokenValue: "redacted-scanner-token",
					JoinMethod: types.JoinMethodIAM,
					CAPins: []string{
						"sha256:redacted",
					},
				},
				Storage: &StorageConfig{
					Destination: &DestinationDirectory{
						Path: "/var/lib/teleport/bot",
					},
				},
				Services: ServiceConfigs{
					&KubernetesOutput{
						Destination: &DestinationDirectory{
							Path: "/opt/machine-id",
						},
						KubernetesCluster: "devops",
					},
				},
			},
		},
		{
			name: "real world 5",
			input: `
auth_server: "redacted.teleport.sh:443"
onboarding:
  join_method: "iam"
  token: "redacted-argocd-token"
  ca_pins:
  - "sha256:redacted"
storage:
  directory: /var/lib/teleport/bot
destinations:
  - directory:
      path: /mount/redacted-prod-global
      acls: off
    kubernetes_cluster: redacted-prod-global
  - directory:
      path: /mount/redacted-prod-au
      acls: off
    kubernetes_cluster: redacted-prod-au
  - directory:
      path: /mount/redacted-prod-eu2
      acls: off
    kubernetes_cluster: redacted-prod-eu2
  - directory:
      path: /mount/redacted-prod-ca
      acls: off
    kubernetes_cluster: redacted-prod-ca
  - directory:
      path: /mount/redacted-prod-us
      acls: off
    kubernetes_cluster: redacted-prod-us
`,
			wantOutput: &BotConfig{
				Version:    V2,
				AuthServer: "redacted.teleport.sh:443",
				Onboarding: OnboardingConfig{
					TokenValue: "redacted-argocd-token",
					JoinMethod: types.JoinMethodIAM,
					CAPins: []string{
						"sha256:redacted",
					},
				},
				Storage: &StorageConfig{
					Destination: &DestinationDirectory{
						Path: "/var/lib/teleport/bot",
					},
				},
				Services: ServiceConfigs{
					&KubernetesOutput{
						Destination: &DestinationDirectory{
							Path: "/mount/redacted-prod-global",
							ACLs: botfs.ACLOff,
						},
						KubernetesCluster: "redacted-prod-global",
					},
					&KubernetesOutput{
						Destination: &DestinationDirectory{
							Path: "/mount/redacted-prod-au",
							ACLs: botfs.ACLOff,
						},
						KubernetesCluster: "redacted-prod-au",
					},
					&KubernetesOutput{
						Destination: &DestinationDirectory{
							Path: "/mount/redacted-prod-eu2",
							ACLs: botfs.ACLOff,
						},
						KubernetesCluster: "redacted-prod-eu2",
					},
					&KubernetesOutput{
						Destination: &DestinationDirectory{
							Path: "/mount/redacted-prod-ca",
							ACLs: botfs.ACLOff,
						},
						KubernetesCluster: "redacted-prod-ca",
					},
					&KubernetesOutput{
						Destination: &DestinationDirectory{
							Path: "/mount/redacted-prod-us",
							ACLs: botfs.ACLOff,
						},
						KubernetesCluster: "redacted-prod-us",
					},
				},
			},
		},
		{
			name: "real world 6",
			// up to 10 roles/destinations depending on the environment
			input: `
auth_server: "redacted.teleport.sh:443"
onboarding:
  join_method: "token"
  token: "redacted"

storage:
  directory: "/var/lib/teleport/tbot"

destinations:
  - directory:
      acls: required
      path: /path/to/role1_creds
    roles:
    - role1
  - directory:
      acls: required
      path: /path/to/role2_creds
    roles:
    - role2
  - directory:
      acls: required
      path: /path/to/roleN_creds
    roles:
    - roleN
`,
			wantOutput: &BotConfig{
				Version:    V2,
				AuthServer: "redacted.teleport.sh:443",
				Onboarding: OnboardingConfig{
					TokenValue: "redacted",
					JoinMethod: types.JoinMethodToken,
				},
				Storage: &StorageConfig{
					Destination: &DestinationDirectory{
						Path: "/var/lib/teleport/tbot",
					},
				},
				Services: ServiceConfigs{
					&IdentityOutput{
						Destination: &DestinationDirectory{
							Path: "/path/to/role1_creds",
							ACLs: botfs.ACLRequired,
						},
						Roles: []string{"role1"},
					},
					&IdentityOutput{
						Destination: &DestinationDirectory{
							Path: "/path/to/role2_creds",
							ACLs: botfs.ACLRequired,
						},
						Roles: []string{"role2"},
					},
					&IdentityOutput{
						Destination: &DestinationDirectory{
							Path: "/path/to/roleN_creds",
							ACLs: botfs.ACLRequired,
						},
						Roles: []string{"roleN"},
					},
				},
			},
		},
		// Error cases
		{
			name:      "storage config with no destination",
			input:     `storage: {}`,
			wantError: "at least one of `memory' and 'directory' must be specified",
		},
		{
			name: "storage config with absurd destination",
			input: `
storage:
  memory: true
  directory:
    path: /opt/machine-id`,
			wantError: "both 'memory' and 'directory' cannot be specified",
		},
		{
			name: "destination with duplicate config types",
			input: `
destinations:
- memory: true
  configs:
  - ssh_client
  - ssh_client: {}`,
			wantError: `multiple config template entries found for "ssh_client"`,
		},
		{
			name: "destination with unsupported config type",
			input: `
destinations:
- memory: true
  app: my-app
  configs:
  - kubernetes`,
			wantError: `config template "kubernetes" unsupported by new output type`,
		},
		{
			name: "destination with empty config type",
			input: `
destinations:
- memory: true
  configs:
  - {}`,
			wantError: `config template must not be empty`,
		},
		{
			name: "destination with indeterminate type",
			input: `
destinations:
- memory: true
  app: my-app
  kubernetes_cluster: my-cluster
`,
			wantError: `multiple potential output types detected, cannot determine correct type`,
		},
		{
			name: "v2 config without version field",
			input: `
outputs:
  - type: identity
    destination:
      type: memory
  - type: identity
    destination:
      type: memory
`,
			wantError: "config has been detected as potentially v1, but includes the v2 outputs field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader([]byte(tt.input))
			out, err := ReadConfig(r, true)
			if tt.wantError != "" {
				require.ErrorContains(t, err, tt.wantError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantOutput, out)

		})
	}
}
