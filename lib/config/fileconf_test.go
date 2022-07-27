/*
Copyright 2015 Gravitational, Inc.

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
	"math"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/sshutils/x11"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// minimalConfigFile is a minimal subset of a teleport config file that can be
// mutated programatically by test cases and then re-serialized to test the
// config file loader
const minimalConfigFile string = `
teleport:
  nodename: testing

auth_service:
  enabled: yes

ssh_service:
  enabled: yes
`

// cfgMap is a shorthand for a type that can hold the nested key-value
// representation of a parsed YAML file.
type cfgMap map[interface{}]interface{}

// editConfig takes the minimal YAML configuration file, de-serializes it into a
// nested key-value dictionary suitable for manipulation by a test case,
// passes that dictionary to the caller-supplied mutator and then re-serializes
// it ready to be injected into the config loader.
func editConfig(t *testing.T, mutate func(cfg cfgMap)) []byte {
	var cfg cfgMap
	require.NoError(t, yaml.Unmarshal([]byte(minimalConfigFile), &cfg))
	mutate(cfg)

	text, err := yaml.Marshal(cfg)
	require.NoError(t, err)

	return text
}

// requireEqual creates an assertion function with a bound `expected` value
// for use with table-driven tests
func requireEqual(expected interface{}) require.ValueAssertionFunc {
	return func(t require.TestingT, actual interface{}, msgAndArgs ...interface{}) {
		require.Equal(t, expected, actual, msgAndArgs...)
	}
}

// TestAuthSection tests the config parser for the `auth_service` config block
func TestAuthSection(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc                 string
		mutate               func(cfgMap)
		expectError          require.ErrorAssertionFunc
		expectEnabled        require.BoolAssertionFunc
		expectIdleMsg        require.ValueAssertionFunc
		expectMotd           require.ValueAssertionFunc
		expectWebIdleTimeout require.ValueAssertionFunc
	}{
		{
			desc:                 "Default",
			mutate:               func(cfg cfgMap) {},
			expectError:          require.NoError,
			expectEnabled:        require.True,
			expectIdleMsg:        require.Empty,
			expectMotd:           require.Empty,
			expectWebIdleTimeout: require.Empty,
		}, {
			desc: "Enabled",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["enabled"] = "yes"
			},
			expectError:   require.NoError,
			expectEnabled: require.True,
		}, {
			desc: "Disabled",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["enabled"] = "no"
			},
			expectError:   require.NoError,
			expectEnabled: require.False,
		}, {
			desc: "Idle timeout message",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["client_idle_timeout_message"] = "Are you pondering what I'm pondering?"
			},
			expectError:   require.NoError,
			expectIdleMsg: requireEqual("Are you pondering what I'm pondering?"),
		}, {
			desc: "Message of the Day",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["message_of_the_day"] = "Are you pondering what I'm pondering?"
			},
			expectError: require.NoError,
			expectMotd:  requireEqual("Are you pondering what I'm pondering?"),
		}, {
			desc: "Web idle timeout",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["web_idle_timeout"] = "10m"
			},
			expectError:          require.NoError,
			expectWebIdleTimeout: requireEqual(types.Duration(10 * time.Minute)),
		}, {
			desc: "Web idle timeout (invalid)",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["web_idle_timeout"] = "potato"
			},
			expectError: require.Error,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			text := bytes.NewBuffer(editConfig(t, tt.mutate))

			cfg, err := ReadConfig(text)
			tt.expectError(t, err)

			if tt.expectEnabled != nil {
				tt.expectEnabled(t, cfg.Auth.Enabled())
			}

			if tt.expectIdleMsg != nil {
				tt.expectIdleMsg(t, cfg.Auth.ClientIdleTimeoutMessage)
			}

			if tt.expectMotd != nil {
				tt.expectMotd(t, cfg.Auth.MessageOfTheDay)
			}

			if tt.expectWebIdleTimeout != nil {
				tt.expectWebIdleTimeout(t, cfg.Auth.WebIdleTimeout)
			}
		})
	}
}

func TestAuthenticationSection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc        string
		mutate      func(cfgMap)
		expectError require.ErrorAssertionFunc
		expected    *AuthenticationConfig
	}{
		{
			desc: "Local auth with OTP",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"type":          "local",
					"second_factor": "otp",
				}
			},
			expectError: require.NoError,
			expected: &AuthenticationConfig{
				Type:         "local",
				SecondFactor: "otp",
			},
		}, {
			desc: "Local auth without OTP",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"type":          "local",
					"second_factor": "off",
				}
			},
			expectError: require.NoError,
			expected: &AuthenticationConfig{
				Type:         "local",
				SecondFactor: "off",
			},
		}, {
			desc: "Local auth with u2f",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"type":          "local",
					"second_factor": "u2f",
					"u2f": cfgMap{
						"app_id": "https://graviton:3080",
						"facets": []interface{}{"https://graviton:3080"},
						"device_attestation_cas": []interface{}{
							"testdata/u2f_attestation_ca.pam",
							"-----BEGIN CERTIFICATE-----\nfake certificate\n-----END CERTIFICATE-----",
						},
					},
				}
			},
			expectError: require.NoError,
			expected: &AuthenticationConfig{
				Type:         "local",
				SecondFactor: "u2f",
				U2F: &UniversalSecondFactor{
					AppID: "https://graviton:3080",
					Facets: []string{
						"https://graviton:3080",
					},
					DeviceAttestationCAs: []string{
						"testdata/u2f_attestation_ca.pam",
						"-----BEGIN CERTIFICATE-----\nfake certificate\n-----END CERTIFICATE-----",
					},
				},
			},
		}, {
			desc: "Local auth with Webauthn",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"type":          "local",
					"second_factor": "webauthn",
					"webauthn": cfgMap{
						"rp_id": "example.com",
						"attestation_allowed_cas": []interface{}{
							"testdata/u2f_attestation_ca.pam",
							"-----BEGIN CERTIFICATE-----\nfake certificate1\n-----END CERTIFICATE-----",
						},
						"attestation_denied_cas": []interface{}{
							"-----BEGIN CERTIFICATE-----\nfake certificate2\n-----END CERTIFICATE-----",
							"testdata/u2f_attestation_ca.pam",
						},
					},
				}
			},
			expectError: require.NoError,
			expected: &AuthenticationConfig{
				Type:         "local",
				SecondFactor: "webauthn",
				Webauthn: &Webauthn{
					RPID: "example.com",
					AttestationAllowedCAs: []string{
						"testdata/u2f_attestation_ca.pam",
						"-----BEGIN CERTIFICATE-----\nfake certificate1\n-----END CERTIFICATE-----",
					},
					AttestationDeniedCAs: []string{
						"-----BEGIN CERTIFICATE-----\nfake certificate2\n-----END CERTIFICATE-----",
						"testdata/u2f_attestation_ca.pam",
					},
				},
			},
		}, {
			desc: "Local auth with disabled Webauthn",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"type":          "local",
					"second_factor": "on",
					"u2f": cfgMap{
						"app_id": "https://example.com",
						"facets": []interface{}{
							"https://example.com",
						},
					},
					"webauthn": cfgMap{
						"disabled": true, // Kept for backwards compatibility, has no effect.
					},
				}
			},
			expectError: require.NoError,
			expected: &AuthenticationConfig{
				Type:         "local",
				SecondFactor: "on",
				U2F: &UniversalSecondFactor{
					AppID:  "https://example.com",
					Facets: []string{"https://example.com"},
				},
				Webauthn: &Webauthn{
					Disabled: true,
				},
			},
		}, {
			desc: "Local auth with passwordless connector",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"type":          "local",
					"second_factor": "on",
					"webauthn": cfgMap{
						"rp_id": "example.com",
					},
					"passwordless":   "true",
					"connector_name": "passwordless",
				}
			},
			expectError: require.NoError,
			expected: &AuthenticationConfig{
				Type:         "local",
				SecondFactor: "on",
				Webauthn: &Webauthn{
					RPID: "example.com",
				},
				Passwordless:  types.NewBoolOption(true),
				ConnectorName: "passwordless",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			text := bytes.NewBuffer(editConfig(t, tt.mutate))

			cfg, err := ReadConfig(text)
			tt.expectError(t, err)

			require.Empty(t, cmp.Diff(cfg.Auth.Authentication, tt.expected))
		})
	}
}

func TestAuthenticationConfig_Parse_StaticToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc  string
		token string
	}{
		{desc: "file path on windows", token: `C:\path\to\some\file`},
		{desc: "literal string", token: "some-literal-token"},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			staticToken := StaticToken("Auth,Node,Proxy:" + tt.token)
			provisionTokens, err := staticToken.Parse()
			require.NoError(t, err)

			require.Len(t, provisionTokens, 1)
			provisionToken := provisionTokens[0]

			want := types.ProvisionTokenV1{
				Roles: []types.SystemRole{
					types.RoleAuth, types.RoleNode, types.RoleProxy,
				},
				Token:   tt.token,
				Expires: provisionToken.Expires,
			}
			require.Equal(t, provisionToken, want)
		})
	}
}

func TestAuthenticationConfig_Parse_nilU2F(t *testing.T) {
	// An absent U2F section should be reflected as a nil U2F object.
	// The config below is a valid config without U2F, but other than that we
	// don't care about its specifics for this test.
	text := editConfig(t, func(cfg cfgMap) {
		cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
			"type":          "local",
			"second_factor": "on",
			"webauthn": cfgMap{
				"rp_id": "localhost",
			},
		}
	})
	cfg, err := ReadConfig(bytes.NewBuffer(text))
	require.NoError(t, err)

	cap, err := cfg.Auth.Authentication.Parse()
	require.NoError(t, err, "failed parsing cap")

	_, u2fErr := cap.GetU2F()
	require.Error(t, u2fErr, "U2F configuration present")
	require.True(t, trace.IsNotFound(u2fErr), "uxpected U2F error")

	_, webErr := cap.GetWebauthn()
	require.NoError(t, webErr, "unexpected webauthn error")
}

// TestSSHSection tests the config parser for the SSH config block
func TestSSHSection(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc                      string
		mutate                    func(cfgMap)
		expectError               require.ErrorAssertionFunc
		expectEnabled             require.BoolAssertionFunc
		expectAllowsTCPForwarding require.BoolAssertionFunc
	}{
		{
			desc:                      "default",
			mutate:                    func(cfgMap) {},
			expectError:               require.NoError,
			expectEnabled:             require.True,
			expectAllowsTCPForwarding: require.True,
		}, {
			desc: "explicitly enabled",
			mutate: func(cfg cfgMap) {
				cfg["ssh_service"].(cfgMap)["enabled"] = "yes"
			},
			expectError:               require.NoError,
			expectEnabled:             require.True,
			expectAllowsTCPForwarding: require.True,
		}, {
			desc: "diasbled",
			mutate: func(cfg cfgMap) {
				cfg["ssh_service"].(cfgMap)["enabled"] = "no"
			},
			expectError:               require.NoError,
			expectEnabled:             require.False,
			expectAllowsTCPForwarding: require.True,
		}, {
			desc: "Port forwarding is enabled",
			mutate: func(cfg cfgMap) {
				cfg["ssh_service"].(cfgMap)["port_forwarding"] = true
			},
			expectError:               require.NoError,
			expectEnabled:             require.True,
			expectAllowsTCPForwarding: require.True,
		}, {
			desc: "Port forwarding is disabled",
			mutate: func(cfg cfgMap) {
				cfg["ssh_service"].(cfgMap)["port_forwarding"] = false
			},
			expectError:               require.NoError,
			expectEnabled:             require.True,
			expectAllowsTCPForwarding: require.False,
		}, {
			desc: "Port forwarding invalid value",
			mutate: func(cfg cfgMap) {
				cfg["ssh_service"].(cfgMap)["port_forwarding"] = "banana"
			},
			expectError: require.Error,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.desc, func(t *testing.T) {
			text := bytes.NewBuffer(editConfig(t, testCase.mutate))

			cfg, err := ReadConfig(text)
			testCase.expectError(t, err)

			if testCase.expectEnabled != nil {
				testCase.expectEnabled(t, cfg.SSH.Enabled())
			}

			if testCase.expectAllowsTCPForwarding != nil {
				testCase.expectAllowsTCPForwarding(t, cfg.SSH.AllowTCPForwarding())
			}
		})
	}
}

func TestX11Config(t *testing.T) {
	testCases := []struct {
		desc              string
		mutate            func(cfgMap)
		expectReadError   require.ErrorAssertionFunc
		expectConfigError require.ErrorAssertionFunc
		expectX11Config   *x11.ServerConfig
	}{
		{
			desc:            "default",
			mutate:          func(cfg cfgMap) {},
			expectX11Config: &x11.ServerConfig{},
		},
		// Test x11 enabled
		{
			desc: "x11 disabled",
			mutate: func(cfg cfgMap) {
				cfg["ssh_service"].(cfgMap)["x11"] = cfgMap{
					"enabled": "no",
				}
			},
			expectX11Config: &x11.ServerConfig{},
		},
		{
			desc: "x11 enabled",
			mutate: func(cfg cfgMap) {
				cfg["ssh_service"].(cfgMap)["x11"] = cfgMap{
					"enabled": "yes",
				}
			},
			expectX11Config: &x11.ServerConfig{
				Enabled:       true,
				DisplayOffset: x11.DefaultDisplayOffset,
				MaxDisplay:    x11.DefaultDisplayOffset + x11.DefaultMaxDisplays,
			},
		},
		// Test display offset
		{
			desc: "display offset set",
			mutate: func(cfg cfgMap) {
				cfg["ssh_service"].(cfgMap)["x11"] = cfgMap{
					"enabled":        "yes",
					"display_offset": 100,
				}
			},
			expectX11Config: &x11.ServerConfig{
				Enabled:       true,
				DisplayOffset: 100,
				MaxDisplay:    100 + x11.DefaultMaxDisplays,
			},
		},
		{
			desc: "display offset value capped",
			mutate: func(cfg cfgMap) {
				cfg["ssh_service"].(cfgMap)["x11"] = cfgMap{
					"enabled":        "yes",
					"display_offset": math.MaxUint32,
				}
			},
			expectX11Config: &x11.ServerConfig{
				Enabled:       true,
				DisplayOffset: x11.MaxDisplayNumber,
				MaxDisplay:    x11.MaxDisplayNumber,
			},
		},
		// Test max display
		{
			desc: "max display set",
			mutate: func(cfg cfgMap) {
				cfg["ssh_service"].(cfgMap)["x11"] = cfgMap{
					"enabled":     "yes",
					"max_display": 100,
				}
			},
			expectX11Config: &x11.ServerConfig{
				Enabled:       true,
				DisplayOffset: x11.DefaultDisplayOffset,
				MaxDisplay:    100,
			},
		},
		{
			desc: "max display value capped",
			mutate: func(cfg cfgMap) {
				cfg["ssh_service"].(cfgMap)["x11"] = cfgMap{
					"enabled":     "yes",
					"max_display": math.MaxUint32,
				}
			},
			expectX11Config: &x11.ServerConfig{
				Enabled:       true,
				DisplayOffset: x11.DefaultDisplayOffset,
				MaxDisplay:    x11.MaxDisplayNumber,
			},
		},
		{
			desc: "max display smaller than display offset",
			mutate: func(cfg cfgMap) {
				cfg["ssh_service"].(cfgMap)["x11"] = cfgMap{
					"enabled":        "maybe",
					"display_offset": 1000,
					"max_display":    100,
				}
			},
			expectConfigError: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err), "got err = %v, want BadParameter", err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			text := bytes.NewBuffer(editConfig(t, tc.mutate))

			cfg, err := ReadConfig(text)
			if tc.expectReadError != nil {
				tc.expectReadError(t, err)
				return
			}
			require.NoError(t, err)

			serverCfg, err := cfg.SSH.X11ServerConfig()
			if tc.expectConfigError != nil {
				tc.expectConfigError(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expectX11Config, serverCfg)
		})
	}
}

func TestMakeSampleFileConfig(t *testing.T) {
	t.Run("Default roles", func(t *testing.T) {
		fc, err := MakeSampleFileConfig(SampleFlags{
			Roles: "",
		})
		require.NoError(t, err)
		require.Equal(t, "yes", fc.SSH.EnabledFlag)
		require.Equal(t, "yes", fc.Proxy.EnabledFlag)
		require.Equal(t, "yes", fc.Auth.EnabledFlag)
	})

	t.Run("Node role", func(t *testing.T) {
		fc, err := MakeSampleFileConfig(SampleFlags{
			Roles: "node",
		})
		require.NoError(t, err)
		require.Equal(t, "yes", fc.SSH.EnabledFlag)
		require.Equal(t, "no", fc.Proxy.EnabledFlag)
		require.Equal(t, "no", fc.Auth.EnabledFlag)
	})

	t.Run("App role", func(t *testing.T) {
		fc, err := MakeSampleFileConfig(SampleFlags{
			Roles:   "app",
			AppName: "app name",
			AppURI:  "localhost:8080",
		})
		require.NoError(t, err)
		require.Equal(t, "no", fc.SSH.EnabledFlag)
		require.Equal(t, "no", fc.Proxy.EnabledFlag)
		require.Equal(t, "no", fc.Auth.EnabledFlag)
		require.Equal(t, "yes", fc.Apps.EnabledFlag)
	})

	t.Run("App name and URI are mandatory", func(t *testing.T) {
		_, err := MakeSampleFileConfig(SampleFlags{
			Roles:  "app",
			AppURI: "localhost:8080",
		})
		require.Error(t, err)

		_, err = MakeSampleFileConfig(SampleFlags{
			Roles:   "app",
			AppName: "nginx",
		})
		require.Error(t, err)

		fc, err := MakeSampleFileConfig(SampleFlags{
			Roles:   "app",
			AppURI:  "localhost:8080",
			AppName: "nginx",
		})
		require.NoError(t, err)

		require.Equal(t, "no", fc.SSH.EnabledFlag)
		require.Equal(t, "no", fc.Proxy.EnabledFlag)
		require.Equal(t, "no", fc.Auth.EnabledFlag)
		require.Equal(t, "yes", fc.Apps.EnabledFlag)
	})

	t.Run("Proxy role", func(t *testing.T) {
		fc, err := MakeSampleFileConfig(SampleFlags{
			Roles: "proxy",
		})
		require.NoError(t, err)
		require.Equal(t, "no", fc.SSH.EnabledFlag)
		require.Equal(t, "yes", fc.Proxy.EnabledFlag)
		require.Equal(t, "no", fc.Auth.EnabledFlag)
	})

	t.Run("App role included when flag AppName is added", func(t *testing.T) {
		fc, err := MakeSampleFileConfig(SampleFlags{
			Roles:   "proxy",
			AppName: "my-app",
			AppURI:  "localhost:8080",
		})
		require.NoError(t, err)
		require.Equal(t, "no", fc.SSH.EnabledFlag)
		require.Equal(t, "yes", fc.Proxy.EnabledFlag)
		require.Equal(t, "no", fc.Auth.EnabledFlag)
		require.Equal(t, "yes", fc.Apps.EnabledFlag)
		require.Equal(t, "my-app", fc.Apps.Apps[0].Name)
	})

	t.Run("Multiple roles", func(t *testing.T) {
		fc, err := MakeSampleFileConfig(SampleFlags{
			Roles:   "proxy,app,db",
			AppName: "app name",
			AppURI:  "localhost:8080",
		})
		require.NoError(t, err)
		require.Equal(t, "no", fc.SSH.EnabledFlag)
		require.Equal(t, "yes", fc.Proxy.EnabledFlag)
		require.Equal(t, "no", fc.Auth.EnabledFlag)
		require.Equal(t, "yes", fc.Apps.EnabledFlag)
		require.Equal(t, "no", fc.Databases.EnabledFlag) // db is always disabled
	})

	t.Run("Auth server", func(t *testing.T) {
		fc, err := MakeSampleFileConfig(SampleFlags{
			AuthServer: "auth-server",
		})
		require.NoError(t, err)
		require.Equal(t, "auth-server", fc.AuthServers[0])
	})

	t.Run("Data dir", func(t *testing.T) {
		fc, err := MakeSampleFileConfig(SampleFlags{
			DataDir: "/path/to/data/dir",
		})
		require.NoError(t, err)
		require.Equal(t, "/path/to/data/dir", fc.DataDir)

		fc, err = MakeSampleFileConfig(SampleFlags{
			DataDir: "",
		})
		require.NoError(t, err)
		require.Equal(t, defaults.DataDir, fc.DataDir)
	})

	t.Run("Token", func(t *testing.T) {
		fc, err := MakeSampleFileConfig(SampleFlags{
			AuthToken:  "auth-token",
			JoinMethod: "token",
		})
		require.NoError(t, err)
		require.Equal(t, "auth-token", fc.JoinParams.TokenName)
		require.Equal(t, types.JoinMethodToken, fc.JoinParams.Method)
	})

	t.Run("Token, method not specified", func(t *testing.T) {
		fc, err := MakeSampleFileConfig(SampleFlags{
			AuthToken: "auth-token",
		})
		require.NoError(t, err)
		require.Equal(t, "auth-token", fc.JoinParams.TokenName)
		require.Equal(t, types.JoinMethodToken, fc.JoinParams.Method)
	})

	t.Run("App name and URI", func(t *testing.T) {
		fc, err := MakeSampleFileConfig(SampleFlags{
			AppName: "app-name",
			AppURI:  "https://localhost:8080",
		})
		require.NoError(t, err)
		require.Equal(t, "app-name", fc.Apps.Apps[0].Name)
		require.Equal(t, "https://localhost:8080", fc.Apps.Apps[0].URI)
	})

	t.Run("Node labels", func(t *testing.T) {
		fc, err := MakeSampleFileConfig(SampleFlags{
			NodeLabels: "foo=bar,baz=bax",
		})
		require.NoError(t, err)
		require.Equal(t, map[string]string{
			"foo": "bar",
			"baz": "bax",
		}, fc.SSH.Labels)
	})

	t.Run("Node labels - invalid", func(t *testing.T) {
		_, err := MakeSampleFileConfig(SampleFlags{
			NodeLabels: "foo=bar,baz",
		})
		require.Error(t, err)
	})

	t.Run("CAPin", func(t *testing.T) {
		fc, err := MakeSampleFileConfig(SampleFlags{
			CAPin: "sha256:7e12c17c20d9cb",
		})
		require.NoError(t, err)
		require.Equal(t, apiutils.Strings{"sha256:7e12c17c20d9cb"}, fc.CAPin)
		fc, err = MakeSampleFileConfig(SampleFlags{
			CAPin: "sha256:7e12c17c20d9cb,sha256:7e12c17c20d9cb",
		})
		require.NoError(t, err)
		require.Equal(t, apiutils.Strings{"sha256:7e12c17c20d9cb", "sha256:7e12c17c20d9cb"}, fc.CAPin)
	})
}
