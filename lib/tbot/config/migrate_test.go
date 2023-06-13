/*
Copyright 2023 Gravitational, Inc.

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
				Version:         V2,
				AuthServer:      "example.teleport.sh:443",
				Oneshot:         true,
				Debug:           true,
				RenewalInterval: time.Minute * 10,
				CertificateTTL:  time.Minute * 30,
				DiagAddr:        "127.0.0.1:621",
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
				Outputs: []Output{
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
						Subtype:     UnspecifiedDatabaseSubtype,
					},
					&DatabaseOutput{
						Destination: &DestinationMemory{},
						Service:     "my-db-service",
						Subtype:     MongoDatabaseSubtype,
					},
					&DatabaseOutput{
						Destination: &DestinationMemory{},
						Service:     "my-db-service",
						Subtype:     TLSDatabaseSubtype,
					},
					&DatabaseOutput{
						Destination: &DestinationMemory{},
						Service:     "my-db-service",
						Subtype:     CockroachDatabaseSubtype,
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
				Outputs: []Output{
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
				Outputs: []Output{
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
				Outputs: []Output{
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
			name: "backwards compat with https://goteleport.com/docs/machine-id/guides/jenkins/",
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
				Outputs: []Output{
					&IdentityOutput{
						Destination: &DestinationDirectory{
							Path: "/opt/machine-id",
						},
					},
				},
			},
		},
		{
			name: "backwards compat with https://goteleport.com/docs/machine-id/guides/databases/",
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
				Outputs: []Output{
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
			name: "backwards compat with https://goteleport.com/docs/machine-id/guides/databases/ - mongo",
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
				Outputs: []Output{
					&DatabaseOutput{
						Destination: &DestinationDirectory{
							Path: "/opt/machine-id",
						},
						Subtype:  MongoDatabaseSubtype,
						Service:  "example-server",
						Username: "alice",
						Database: "example",
					},
				},
			},
		},
		{
			name: "backwards compat with https://goteleport.com/docs/machine-id/guides/databases/ - cockroach",
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
				Outputs: []Output{
					&DatabaseOutput{
						Destination: &DestinationDirectory{
							Path: "/opt/machine-id",
						},
						Subtype:  CockroachDatabaseSubtype,
						Service:  "example-server",
						Username: "alice",
						Database: "example",
					},
				},
			},
		},
		{
			name: "backwards compat with https://goteleport.com/docs/machine-id/guides/databases/ - tls",
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
				Outputs: []Output{
					&DatabaseOutput{
						Destination: &DestinationDirectory{
							Path: "/opt/machine-id",
						},
						Subtype:  TLSDatabaseSubtype,
						Service:  "example-server",
						Username: "alice",
						Database: "example",
					},
				},
			},
		},
		{
			name: "backwards compat with https://goteleport.com/docs/machine-id/guides/host-certificate/",
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
				RenewalInterval: DefaultRenewInterval,
				CertificateTTL:  DefaultCertificateTTL,
				Storage: &StorageConfig{
					Destination: &DestinationDirectory{
						Path:     "/var/lib/teleport/bot",
						Symlinks: "secure",
						ACLs:     "try",
					},
				},
				Outputs: []Output{
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
			name: "backwards compat with https://goteleport.com/docs/machine-id/guides/applications/",
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
				Outputs: []Output{
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
			name: "backwards compat with https://goteleport.com/docs/machine-id/guides/applications/ - with tls config",
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
				Outputs: []Output{
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
			name: "backwards compat with https://goteleport.com/docs/machine-id/guides/kubernetes/",
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
				Outputs: []Output{
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
				Outputs: Outputs{
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
