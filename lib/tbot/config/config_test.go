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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/bot/onboarding"
	"github.com/gravitational/teleport/lib/tbot/botfs"
	"github.com/gravitational/teleport/lib/tbot/services/application"
	"github.com/gravitational/teleport/lib/tbot/services/example"
	"github.com/gravitational/teleport/lib/tbot/services/identity"
	"github.com/gravitational/teleport/lib/tbot/services/k8s"
	"github.com/gravitational/teleport/lib/tbot/services/ssh"
	"github.com/gravitational/teleport/lib/tbot/services/workloadidentity"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

func TestConfigFile(t *testing.T) {
	configData := fmt.Sprintf(exampleConfigFile, "foo")
	cfg, err := ReadConfig(strings.NewReader(configData), false)
	require.NoError(t, err)
	require.NoError(t, cfg.CheckAndSetDefaults())

	require.Equal(t, "auth.example.com", cfg.AuthServer)
	require.Equal(t, time.Minute*5, cfg.CredentialLifetime.RenewalInterval)

	require.NotNil(t, cfg.Onboarding)

	token, err := cfg.Onboarding.Token()
	require.NoError(t, err)
	require.Equal(t, "foo", token)
	require.ElementsMatch(t, []string{"sha256:abc123"}, cfg.Onboarding.CAPins)

	_, ok := cfg.Storage.Destination.(*destination.Memory)
	require.True(t, ok)

	require.Len(t, cfg.Services, 1)
	output := cfg.Services[0]
	identOutput, ok := output.(*identity.OutputConfig)
	require.True(t, ok)

	destImpl := identOutput.GetDestination()
	destImplReal, ok := destImpl.(*destination.Directory)
	require.True(t, ok)
	require.Equal(t, "/tmp/foo", destImplReal.Path)

	require.True(t, cfg.Debug)
	require.Equal(t, "127.0.0.1:1337", cfg.DiagAddr)
}

func TestLoadTokenFromFile(t *testing.T) {
	tokenDir := t.TempDir()
	tokenFile := filepath.Join(tokenDir, "token")
	require.NoError(t, os.WriteFile(tokenFile, []byte("xxxyyy"), 0660))

	configData := fmt.Sprintf(exampleConfigFile, tokenFile)
	cfg, err := ReadConfig(strings.NewReader(configData), false)
	require.NoError(t, err)

	token, err := cfg.Onboarding.Token()
	require.NoError(t, err)
	require.Equal(t, "xxxyyy", token)
}

const exampleConfigFile = `
version: v2
auth_server: auth.example.com
renewal_interval: 5m
debug: true
diag_addr: 127.0.0.1:1337
onboarding:
  token: %s
  ca_pins:
    - sha256:abc123
storage:
  type: memory
outputs:
  - type: identity
    destination:
      type: directory
      path: /tmp/foo
`

func TestDestinationFromURI(t *testing.T) {
	tests := []struct {
		in      string
		want    destination.Destination
		wantErr bool
	}{
		{
			in: "/absolute/dir",
			want: &destination.Directory{
				Path: "/absolute/dir",
			},
		},
		{
			in: "relative/dir",
			want: &destination.Directory{
				Path: "relative/dir",
			},
		},
		{
			in: "./relative/dir",
			want: &destination.Directory{
				Path: "./relative/dir",
			},
		},
		{
			in: "file:///absolute/dir",
			want: &destination.Directory{
				Path: "/absolute/dir",
			},
		},
		{
			in: "file:/absolute/dir",
			want: &destination.Directory{
				Path: "/absolute/dir",
			},
		},
		{
			in:      "file://host/absolute/dir",
			wantErr: true,
		},
		{
			in:   "memory://",
			want: &destination.Memory{},
		},
		{
			in:      "memory://foo/bar",
			wantErr: true,
		},
		{
			in:      "foobar://",
			wantErr: true,
		},
		{
			in: "kubernetes-secret:///my-secret",
			want: &k8s.SecretDestination{
				Name: "my-secret",
			},
		},
		{
			in:      "kubernetes-secret://my-secret",
			wantErr: true,
		},
		{
			in: "kubernetes-secret://my-namespace/my-secret",
			want: &k8s.SecretDestination{
				Name:      "my-secret",
				Namespace: "my-namespace",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := DestinationFromURI(tt.in)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

// TestBotConfig_YAML ensures that as a whole YAML marshaling and unmarshaling
// of the config works as expected. Avoid testing exhaustive cases here and
// prefer the Output YAML tests for testing the intricacies of marshaling and
// unmarshaling specific objects.
func TestBotConfig_YAML(t *testing.T) {
	tests := []testYAMLCase[BotConfig]{
		{
			name: "standard config",
			in: BotConfig{
				Version: V2,
				Storage: &StorageConfig{
					Destination: &destination.Directory{
						Path:     "/bot/storage",
						ACLs:     botfs.ACLTry,
						Symlinks: botfs.SymlinksSecure,
					},
				},
				Onboarding: onboarding.Config{
					JoinMethod: "gitlab",
					TokenValue: "my-gitlab-token",
					Gitlab: onboarding.GitlabOnboardingConfig{
						TokenEnvVarName: "MY_CUSTOM_ENV_VAR",
					},
				},
				FIPS:       true,
				Debug:      true,
				Oneshot:    true,
				AuthServer: "example.teleport.sh:443",
				DiagAddr:   "127.0.0.1:1337",
				CredentialLifetime: bot.CredentialLifetime{
					TTL:             time.Minute,
					RenewalInterval: time.Second * 30,
				},
				Outputs: ServiceConfigs{
					&identity.OutputConfig{
						Destination: &destination.Directory{
							Path: "/bot/output",
						},
						Roles:   []string{"editor"},
						Cluster: "example.teleport.sh",
					},
					&identity.OutputConfig{
						Destination: &destination.Memory{},
					},
					&identity.OutputConfig{
						Destination: &k8s.SecretDestination{
							Name: "my-secret",
						},
						CredentialLifetime: bot.CredentialLifetime{
							TTL:             30 * time.Second,
							RenewalInterval: 15 * time.Second,
						},
					},
				},
				Services: []ServiceConfig{
					&example.Config{
						Message: "llama",
					},
					&ssh.MultiplexerConfig{
						Destination: &destination.Directory{
							Path: "/bot/output",
						},
						CredentialLifetime: bot.CredentialLifetime{
							TTL:             30 * time.Second,
							RenewalInterval: 15 * time.Second,
						},
					},
					&application.TunnelConfig{
						Listen:  "tcp://127.0.0.1:123",
						Roles:   []string{"access"},
						AppName: "my-app",
						CredentialLifetime: bot.CredentialLifetime{
							TTL:             30 * time.Second,
							RenewalInterval: 15 * time.Second,
						},
					},
					&workloadidentity.X509OutputConfig{
						Destination: &destination.Directory{
							Path: "/an/output/path",
						},
						Selector: bot.WorkloadIdentitySelector{
							Name: "my-workload-identity",
						},
						CredentialLifetime: bot.CredentialLifetime{
							TTL:             30 * time.Second,
							RenewalInterval: 15 * time.Second,
						},
					},
					&workloadidentity.WorkloadAPIConfig{
						Listen: "tcp://127.0.0.1:123",
						Selector: bot.WorkloadIdentitySelector{
							Name: "my-workload-identity",
						},
						CredentialLifetime: bot.CredentialLifetime{
							TTL:             30 * time.Second,
							RenewalInterval: 15 * time.Second,
						},
					},
					&workloadidentity.JWTOutputConfig{
						Destination: &destination.Directory{
							Path: "/an/output/path",
						},
						Selector: bot.WorkloadIdentitySelector{
							Name: "my-workload-identity",
						},
						Audiences: []string{"audience1", "audience2"},
					},
				},
			},
		},
		{
			name: "minimal config",
			in: BotConfig{
				Version:    V2,
				AuthServer: "example.teleport.sh:443",
				CredentialLifetime: bot.CredentialLifetime{
					TTL:             time.Minute,
					RenewalInterval: time.Second * 30,
				},
				Outputs: ServiceConfigs{
					&identity.OutputConfig{
						Destination: &destination.Memory{},
					},
				},
			},
		},
		{
			name: "minimal config using proxy addr",
			in: BotConfig{
				Version:     V2,
				ProxyServer: "example.teleport.sh:443",
				CredentialLifetime: bot.CredentialLifetime{
					TTL:             time.Minute,
					RenewalInterval: time.Second * 30,
				},
				Outputs: ServiceConfigs{
					&identity.OutputConfig{
						Destination: &destination.Memory{},
					},
				},
			},
		},
	}

	testYAML(t, tests)
}

type testYAMLCase[T any] struct {
	name string
	in   T
}

func testYAML[T any](t *testing.T, tests []testYAMLCase[T]) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := bytes.NewBuffer(nil)
			encoder := yaml.NewEncoder(b)
			encoder.SetIndent(2)
			require.NoError(t, encoder.Encode(&tt.in))

			if golden.ShouldSet() {
				golden.Set(t, b.Bytes())
			}
			require.Equal(
				t,
				string(golden.Get(t)),
				b.String(),
				"results of marshal did not match golden file, rerun tests with GOLDEN_UPDATE=1",
			)

			// Now test unmarshalling to see if we get the same object back
			decoder := yaml.NewDecoder(b)
			var unmarshalled T
			require.NoError(t, decoder.Decode(&unmarshalled))
			require.Equal(t, tt.in, unmarshalled, "unmarshalling did not result in same object as input")
		})
	}
}

func TestBotConfig_InsecureWithCAPins(t *testing.T) {
	cfg := &BotConfig{
		Insecure: true,
		Onboarding: onboarding.Config{
			CAPins: []string{"123"},
		},
	}

	require.ErrorContains(t, cfg.CheckAndSetDefaults(), "ca-pin")
}

func TestBotConfig_InsecureWithCAPath(t *testing.T) {
	cfg := &BotConfig{
		Insecure: true,
		Onboarding: onboarding.Config{
			CAPath: "/tmp/invalid-path/some.crt",
		},
	}

	require.ErrorContains(t, cfg.CheckAndSetDefaults(), "ca-path")
}

func TestBotConfig_WithCAPathAndCAPins(t *testing.T) {
	cfg := &BotConfig{
		Insecure: false,
		Onboarding: onboarding.Config{
			CAPath: "/tmp/invalid-path/some.crt",
			CAPins: []string{"123"},
		},
	}

	require.ErrorContains(t, cfg.CheckAndSetDefaults(), "mutually exclusive")
}

func TestBotConfig_ServicePartialCredentialLifetime(t *testing.T) {
	cfg := &BotConfig{
		Version:    V2,
		AuthServer: "example.teleport.sh:443",
		Services: []ServiceConfig{
			&identity.OutputConfig{
				CredentialLifetime: bot.CredentialLifetime{TTL: 5 * time.Minute},
				Destination:        &destination.Memory{},
			},
		},
	}
	require.ErrorContains(t, cfg.CheckAndSetDefaults(), "credential_ttl and renewal_interval")
}

func TestBotConfig_ServiceInvalidCredentialLifetime(t *testing.T) {
	cfg := &BotConfig{
		Version:    V2,
		AuthServer: "example.teleport.sh:443",
		Services: []ServiceConfig{
			&identity.OutputConfig{
				CredentialLifetime: bot.CredentialLifetime{TTL: 5 * time.Minute},
				Destination:        &destination.Memory{},
			},
		},
	}
	require.ErrorContains(t, cfg.CheckAndSetDefaults(), "credential_ttl and renewal_interval")
}

func TestBotConfig_DeprecatedCertificateTTL(t *testing.T) {
	t.Run("just deprecated option", func(t *testing.T) {
		const config = `
version: v2
certificate_ttl: 5m
`

		cfg, err := ReadConfig(strings.NewReader(config), false)
		require.NoError(t, err)
		require.Equal(t, 5*time.Minute, cfg.CredentialLifetime.TTL)
	})

	t.Run("both options", func(t *testing.T) {
		const config = `
version: v2
certificate_ttl: 5m
credential_ttl: 10m
`

		cfg, err := ReadConfig(strings.NewReader(config), false)
		require.NoError(t, err)
		require.Equal(t, 10*time.Minute, cfg.CredentialLifetime.TTL)
	})
}

// TestBotConfig_Base64 ensures that config can be read from bas64 encoded YAML
func TestBotConfig_Base64(t *testing.T) {
	tests := []struct {
		name         string
		configBase64 string
		expected     BotConfig
	}{
		{
			name:         "minimal config, proxy server",
			configBase64: "dmVyc2lvbjogdjIKcHJveHlfc2VydmVyOiAiZXhhbXBsZS50ZWxlcG9ydC5zaDo0NDMiCm9uYm9hcmRpbmc6CiAgdG9rZW46ICJteS10b2tlbiIKICBqb2luX21ldGhvZDogInRva2VuIgpzZXJ2aWNlczoKLSB0eXBlOiBhcHBsaWNhdGlvbi10dW5uZWwKICBhcHBfbmFtZTogdGVzdGFwcAogIGxpc3RlbjogdGNwOi8vMTI3LjAuMC4xOjgwODA=",
			expected: BotConfig{
				Version:     V2,
				ProxyServer: "example.teleport.sh:443",
				Onboarding: onboarding.Config{
					JoinMethod: "token",
					TokenValue: "my-token",
				},
				Services: []ServiceConfig{
					&application.TunnelConfig{
						Listen:  "tcp://127.0.0.1:8080",
						AppName: "testapp",
					},
				},
			},
		},
		{
			name:         "minimal config, auth server",
			configBase64: "dmVyc2lvbjogdjIKYXV0aF9zZXJ2ZXI6ICJleGFtcGxlLnRlbGVwb3J0LnNoOjQ0MyIKb25ib2FyZGluZzoKICB0b2tlbjogIm15LXRva2VuIgogIGpvaW5fbWV0aG9kOiAidG9rZW4iCnNlcnZpY2VzOgotIHR5cGU6IGFwcGxpY2F0aW9uLXR1bm5lbAogIGFwcF9uYW1lOiB0ZXN0YXBwCiAgbGlzdGVuOiB0Y3A6Ly8xMjcuMC4wLjE6ODA4MA==",
			expected: BotConfig{
				Version:    V2,
				AuthServer: "example.teleport.sh:443",
				Onboarding: onboarding.Config{
					JoinMethod: "token",
					TokenValue: "my-token",
				},
				Services: []ServiceConfig{
					&application.TunnelConfig{
						Listen:  "tcp://127.0.0.1:8080",
						AppName: "testapp",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ReadConfigFromBase64String(tt.configBase64, false)
			require.NoError(t, err)
			require.Equal(t, tt.expected, *cfg)
		})
	}
}

func TestBotConfig_NameValidation(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		cfg *BotConfig
		err string
	}{
		"duplicate names": {
			cfg: &BotConfig{
				Version: V2,
				Services: ServiceConfigs{
					&identity.OutputConfig{
						Name:        "foo",
						Destination: &destination.Memory{},
					},
					&identity.OutputConfig{
						Name:        "foo",
						Destination: &destination.Memory{},
					},
				},
			},
			err: `duplicate name: "foo`,
		},
		"reserved name": {
			cfg: &BotConfig{
				Version: V2,
				Services: ServiceConfigs{
					&identity.OutputConfig{
						Name:        "identity",
						Destination: &destination.Memory{},
					},
				},
			},
			err: `service name "identity" is reserved for internal use`,
		},
		"invalid name": {
			cfg: &BotConfig{
				Version: V2,
				Services: ServiceConfigs{
					&identity.OutputConfig{
						Name:        "hello, world!",
						Destination: &destination.Memory{},
					},
				},
			},
			err: `may only contain lowercase letters`,
		},
	}
	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			t.Parallel()
			require.ErrorContains(t, tc.cfg.CheckAndSetDefaults(), tc.err)
		})
	}
}
