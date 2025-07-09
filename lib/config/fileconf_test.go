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
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"gopkg.in/yaml.v2"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/sshutils/x11"
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

discovery_service:
  enabled: false
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
		desc                    string
		mutate                  func(cfgMap)
		expectError             require.ErrorAssertionFunc
		expectEnabled           require.BoolAssertionFunc
		expectIdleMsg           require.ValueAssertionFunc
		expectMotd              require.ValueAssertionFunc
		expectWebIdleTimeout    require.ValueAssertionFunc
		expectProxyPingInterval require.ValueAssertionFunc
	}{
		{
			desc:                    "Default",
			mutate:                  func(cfg cfgMap) {},
			expectError:             require.NoError,
			expectEnabled:           require.True,
			expectIdleMsg:           require.Empty,
			expectMotd:              require.Empty,
			expectWebIdleTimeout:    require.Empty,
			expectProxyPingInterval: require.Empty,
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
		}, {
			desc: "Proxy ping interval",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["proxy_ping_interval"] = "10s"
			},
			expectError:             require.NoError,
			expectProxyPingInterval: requireEqual(types.Duration(10 * time.Second)),
		}, {
			desc: "Proxy ping interval (invalid)",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["proxy_ping_interval"] = "potato"
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

			if tt.expectProxyPingInterval != nil {
				tt.expectProxyPingInterval(t, cfg.Auth.ProxyPingInterval)
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
			expected: &AuthenticationConfig{
				Type:         "local",
				SecondFactor: "on",
				Webauthn: &Webauthn{
					RPID: "example.com",
				},
				Passwordless:  types.NewBoolOption(true),
				ConnectorName: "passwordless",
			},
		}, {
			desc: "Local auth with headless connector",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"type":          "local",
					"second_factor": "on",
					"webauthn": cfgMap{
						"rp_id": "example.com",
					},
					"headless":       "true",
					"connector_name": "headless",
				}
			},
			expected: &AuthenticationConfig{
				Type:         "local",
				SecondFactor: "on",
				Webauthn: &Webauthn{
					RPID: "example.com",
				},
				Headless:      types.NewBoolOption(true),
				ConnectorName: "headless",
			},
		}, {
			desc: "Device Trust config",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"device_trust": cfgMap{
						"mode": "required",
						"ekcert_allowed_cas": []string{
							"-----BEGIN CERTIFICATE-----\nfake certificate1\n-----END CERTIFICATE-----",
						},
					},
				}
			},
			expected: &AuthenticationConfig{
				DeviceTrust: &DeviceTrust{
					Mode: "required",
					EKCertAllowedCAs: []string{
						"-----BEGIN CERTIFICATE-----\nfake certificate1\n-----END CERTIFICATE-----",
					},
				},
			},
		}, {
			desc: "Device Trust with auto-enroll",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"device_trust": cfgMap{
						"mode":        "required",
						"auto_enroll": "yes",
					},
				}
			},
			expected: &AuthenticationConfig{
				DeviceTrust: &DeviceTrust{
					Mode:       "required",
					AutoEnroll: "yes",
				},
			},
		}, {
			desc: "signature suite empty",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"signature_algorithm_suite": "",
				}
			},
			expected: &AuthenticationConfig{
				SignatureAlgorithmSuite: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_UNSPECIFIED,
			},
		}, {
			desc: "signature suite legacy",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"signature_algorithm_suite": "legacy",
				}
			},
			expected: &AuthenticationConfig{
				SignatureAlgorithmSuite: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_LEGACY,
			},
		}, {
			desc: "signature suite balanced-v1",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"signature_algorithm_suite": "balanced-v1",
				}
			},
			expected: &AuthenticationConfig{
				SignatureAlgorithmSuite: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1,
			},
		}, {
			desc: "signature suite fips-v1",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"signature_algorithm_suite": "fips-v1",
				}
			},
			expected: &AuthenticationConfig{
				SignatureAlgorithmSuite: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_FIPS_V1,
			},
		}, {
			desc: "signature suite hsm-v1",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"signature_algorithm_suite": "hsm-v1",
				}
			},
			expected: &AuthenticationConfig{
				SignatureAlgorithmSuite: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_HSM_V1,
			},
		}, {
			desc: "signature suite typo",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"signature_algorithm_suite": "balanced-v0",
				}
			},
			expectError: func(t require.TestingT, err error, msgAndArgs ...interface{}) {
				require.ErrorContains(t, err, "invalid value: balanced-v0")
			},
		}, {
			desc: "stable unix users",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"stable_unix_user_config": cfgMap{
						"enabled":   true,
						"first_uid": 100,
						"last_uid":  10,
					},
				}
			},
			expected: &AuthenticationConfig{
				StableUNIXUserConfig: &StableUNIXUserConfig{
					Enabled:  true,
					FirstUID: 100,
					LastUID:  10,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			text := bytes.NewBuffer(editConfig(t, tt.mutate))
			cfg, err := ReadConfig(text)
			if tt.expectError != nil {
				tt.expectError(t, err)
				return
			}
			require.NoError(t, err)

			require.Empty(t, cmp.Diff(cfg.Auth.Authentication, tt.expected))
		})
	}
}

func TestAuthenticationConfig_HandleSecondFactorOffOnWithoutQuotes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		desc               string
		input              string
		expectError        require.ErrorAssertionFunc
		expectSecondFactor require.ValueAssertionFunc
	}{
		{
			desc: "handle off with quotes", input: `
auth_service:
  enabled: yes
  authentication:
    type: local
    second_factor: "off"
teleport:
  nodename: testing
ssh_service:
  enabled: yes`,
			expectError:        require.NoError,
			expectSecondFactor: requireEqual(constants.SecondFactorOff),
		},
		{
			desc: "handle off without quotes", input: `
auth_service:
  enabled: yes
  authentication:
    type: local
    second_factor: off
teleport:
  nodename: testing
ssh_service:
  enabled: yes`,
			expectError:        require.NoError,
			expectSecondFactor: requireEqual(constants.SecondFactorOff),
		},
		{
			desc: "handle on without quotes", input: `
auth_service:
  enabled: yes
  authentication:
    type: local
    second_factor: on
teleport:
  nodename: testing
ssh_service:
  enabled: yes`,
			expectError:        require.NoError,
			expectSecondFactor: requireEqual(constants.SecondFactorOn),
		},
		{
			desc: "unsupported numeric type as second_factor", input: `
auth_service:
  enabled: yes
  authentication:
    type: local
    second_factor: 4.4
teleport:
  nodename: testing
ssh_service:
  enabled: yes`,
			expectError: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			cfg, err := ReadConfig(strings.NewReader(tt.input))
			tt.expectError(t, err)
			if tt.expectSecondFactor != nil {
				tt.expectSecondFactor(t, cfg.Auth.Authentication.SecondFactor)
			}
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
			require.Equal(t, want, provisionToken)
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
	require.True(t, trace.IsNotFound(u2fErr), "unexpected U2F error")

	_, webErr := cap.GetWebauthn()
	require.NoError(t, webErr, "unexpected webauthn error")
}

func TestAuthenticationConfig_RequireSessionMFA(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc                 string
		mutate               func(cfgMap)
		expectError          bool
		expectRequireMFAType types.RequireMFAType
	}{
		{
			desc: "RequireSessionMFA invalid",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"require_session_mfa": "unknown",
				}
			},
			expectError: true,
		}, {
			desc: "RequireSessionMFA empty",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{}
			},
			expectRequireMFAType: types.RequireMFAType_OFF,
		}, {
			desc: "RequireSessionMFA true",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"require_session_mfa": true,
				}
			},
			expectRequireMFAType: types.RequireMFAType_SESSION,
		}, {
			desc: "RequireSessionMFA false",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"require_session_mfa": false,
				}
			},
			expectRequireMFAType: types.RequireMFAType_OFF,
		}, {
			desc: "RequireSessionMFA true string",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"require_session_mfa": "yes",
				}
			},
			expectRequireMFAType: types.RequireMFAType_SESSION,
		}, {
			desc: "RequireSessionMFA false string",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"require_session_mfa": "off",
				}
			},
			expectRequireMFAType: types.RequireMFAType_OFF,
		}, {
			desc: "RequireSessionMFA hardware_key",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"require_session_mfa": types.RequireMFATypeHardwareKeyString,
				}
			},
			expectRequireMFAType: types.RequireMFAType_SESSION_AND_HARDWARE_KEY,
		}, {
			desc: "RequireSessionMFA hardware_key_touch",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"require_session_mfa": types.RequireMFATypeHardwareKeyTouchString,
				}
			},
			expectRequireMFAType: types.RequireMFAType_HARDWARE_KEY_TOUCH,
		}, {
			desc: "RequireSessionMFA hardware_key_pin",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"require_session_mfa": types.RequireMFATypeHardwareKeyPINString,
				}
			},
			expectRequireMFAType: types.RequireMFAType_HARDWARE_KEY_PIN,
		}, {
			desc: "RequireSessionMFA hardware_key_touch_and_pin",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"require_session_mfa": types.RequireMFATypeHardwareKeyTouchAndPINString,
				}
			},
			expectRequireMFAType: types.RequireMFAType_HARDWARE_KEY_TOUCH_AND_PIN,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			text := bytes.NewBuffer(editConfig(t, tt.mutate))
			cfg, err := ReadConfig(text)
			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Empty(t, cmp.Diff(cfg.Auth.Authentication.RequireMFAType, tt.expectRequireMFAType))
		})
	}
}

func TestAuthenticationConfig_Parse_deviceTrustPB(t *testing.T) {
	// Device trust mode=required is an Enterprise feature.
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
	})

	tpmEKCertPath := "testdata/tpm_ekcert_ca.pem"
	tpmEKCertPEM, err := os.ReadFile(tpmEKCertPath)
	require.NoError(t, err)

	tests := []struct {
		name       string
		configYAML []byte
		wantErr    bool
		wantPB     *types.DeviceTrust
	}{
		{
			name: "minimal config",
			configYAML: editConfig(t, func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"type":          "local",
					"second_factor": "otp",
					"device_trust": cfgMap{
						"mode": "optional",
					},
				}
			}),
			wantPB: &types.DeviceTrust{
				Mode: "optional",
			},
		},
		{
			name: "all fields",
			configYAML: editConfig(t, func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"type":          "local",
					"second_factor": "otp",
					"device_trust": cfgMap{
						"mode":        "required",
						"auto_enroll": "yes",
						"ekcert_allowed_cas": []string{
							string(tpmEKCertPEM),
						},
					},
				}
			}),
			wantPB: &types.DeviceTrust{
				Mode:       "required",
				AutoEnroll: true,
				EKCertAllowedCAs: []string{
					string(tpmEKCertPEM),
				},
			},
		},
		{
			name: "reads ekcert_allowed_cas from path",
			configYAML: editConfig(t, func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"device_trust": cfgMap{
						"ekcert_allowed_cas": []string{
							tpmEKCertPath,
						},
					},
				}
			}),
			wantPB: &types.DeviceTrust{
				EKCertAllowedCAs: []string{
					string(tpmEKCertPEM),
				},
			},
			wantErr: false,
		},
		{
			name: "invalid ekcert_allowed_cas entry",
			configYAML: editConfig(t, func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"device_trust": cfgMap{
						"ekcert_allowed_cas": []string{
							"this is not a pem encoded CA",
						},
					},
				}
			}),
			wantErr: true,
		},
		{
			name: "auto-enroll invalid",
			configYAML: editConfig(t, func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"type":          "local",
					"second_factor": "otp",
					"device_trust": cfgMap{
						"mode":        "required",
						"auto_enroll": "NOT A BOOLEAN", // invalid
					},
				}
			}),
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg, err := ReadConfig(bytes.NewBuffer(test.configYAML))
			require.NoError(t, err, "ReadConfig failed")

			cap, err := cfg.Auth.Authentication.Parse()
			if test.wantErr {
				assert.Error(t, err, "ReadConfig error mismatch")
				assert.True(t, trace.IsBadParameter(err), "ReadConfig returned non-BadParameter error: %v (%T)", err, err)
				return
			}
			require.NoError(t, err, "Parse failed")

			got := cap.GetDeviceTrust()
			if diff := cmp.Diff(test.wantPB, got, protocmp.Transform()); diff != "" {
				t.Errorf("Parse mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

func TestAuthenticationConfig_Parse_StableUNIXUserConfig(t *testing.T) {
	text := editConfig(t, func(cfg cfgMap) {
		cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
			"stable_unix_user_config": nil,
		}
	})
	cfg, err := ReadConfig(bytes.NewBuffer(text))
	require.NoError(t, err)
	_, err = cfg.Auth.Authentication.Parse()
	require.NoError(t, err)

	text = editConfig(t, func(cfg cfgMap) {
		cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
			"stable_unix_user_config": cfgMap{
				"enabled": true,
			},
		}
	})
	cfg, err = ReadConfig(bytes.NewBuffer(text))
	require.NoError(t, err)
	_, err = cfg.Auth.Authentication.Parse()
	require.ErrorContains(t, err, "UID range includes negative or system UIDs")
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
		expectFileCopying         require.BoolAssertionFunc
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
			desc: "disabled",
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
		}, {
			desc: "File copying is enabled",
			mutate: func(cfg cfgMap) {
				cfg["ssh_service"].(cfgMap)["ssh_file_copy"] = true
			},
			expectError:       require.NoError,
			expectEnabled:     require.True,
			expectFileCopying: require.True,
		}, {
			desc: "File copying is disabled",
			mutate: func(cfg cfgMap) {
				cfg["ssh_service"].(cfgMap)["ssh_file_copy"] = false
			},
			expectError:       require.NoError,
			expectEnabled:     require.True,
			expectFileCopying: require.False,
		}, {
			desc:              "File copying is enabled by default",
			mutate:            func(cfg cfgMap) {},
			expectError:       require.NoError,
			expectEnabled:     require.True,
			expectFileCopying: require.True,
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

			if testCase.expectFileCopying != nil {
				testCase.expectFileCopying(t, cfg.SSH.SSHFileCopy())
			}
		})
	}
}

func TestHardwareKeyConfig(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		mutate                  func(cfgMap)
		expectReadError         require.ErrorAssertionFunc
		expectParseError        require.ErrorAssertionFunc
		expectHardwareKeyConfig *types.HardwareKey
	}{
		{
			name: "OK empty",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"hardware_key": cfgMap{},
				}
			},
			expectHardwareKeyConfig: &types.HardwareKey{},
		},
		{
			name: "OK piv_slot",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"hardware_key": cfgMap{
						"piv_slot": "9a",
					},
				}
			},
			expectHardwareKeyConfig: &types.HardwareKey{
				PIVSlot: "9a",
			},
		},
		{
			name: "NOK piv_slot unsupported slot",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"hardware_key": cfgMap{
						"piv_slot": "8f",
					},
				}
			},
			expectParseError: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err), "got err = %v, want BadParameter", err)
			},
		},
		{
			name: "OK serial_number_validation",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"hardware_key": cfgMap{
						"serial_number_validation": cfgMap{
							"enabled":                  true,
							"serial_number_trait_name": "custom_trait_name",
						},
					},
				}
			},
			expectHardwareKeyConfig: &types.HardwareKey{
				SerialNumberValidation: &types.HardwareKeySerialNumberValidation{
					Enabled:               true,
					SerialNumberTraitName: "custom_trait_name",
				},
			},
		},
		{
			name: "OK pin_cache_ttl",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"hardware_key": cfgMap{
						"pin_cache_ttl": "1m",
					},
				}
			},
			expectHardwareKeyConfig: &types.HardwareKey{
				PinCacheTTL: types.Duration(time.Minute),
			},
		},
		{
			name: "NOK pin_cache_ttl not a duration",
			mutate: func(cfg cfgMap) {
				cfg["auth_service"].(cfgMap)["authentication"] = cfgMap{
					"hardware_key": cfgMap{
						"pin_cache_ttl": "1minute",
					},
				}
			},
			expectReadError: require.Error,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text := bytes.NewBuffer(editConfig(t, tc.mutate))

			cfg, err := ReadConfig(text)
			if tc.expectReadError != nil {
				tc.expectReadError(t, err)
				return
			}
			require.NoError(t, err)

			cap, err := cfg.Auth.Authentication.Parse()
			if tc.expectParseError != nil {
				tc.expectParseError(t, err)
				return
			}
			require.NoError(t, err)

			hardwareKeyConfig, err := cap.GetHardwareKey()
			require.NoError(t, err)

			require.Equal(t, tc.expectHardwareKeyConfig, hardwareKeyConfig)
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

	t.Run("V3 - auth server", func(t *testing.T) {
		fc, err := MakeSampleFileConfig(SampleFlags{
			Version:    defaults.TeleportConfigVersionV3,
			AuthServer: "auth-server",
		})
		require.NoError(t, err)
		require.Equal(t, "auth-server", fc.AuthServer)
	})

	t.Run("V3 - proxy server", func(t *testing.T) {
		fc, err := MakeSampleFileConfig(SampleFlags{
			Version:      defaults.TeleportConfigVersionV3,
			ProxyAddress: "proxy.server",
		})
		require.NoError(t, err)
		require.Equal(t, "proxy.server", fc.ProxyServer)
	})

	t.Run("v2 - auth server", func(t *testing.T) {
		fc, err := MakeSampleFileConfig(SampleFlags{
			Version:    defaults.TeleportConfigVersionV2,
			AuthServer: "auth-server",
		})
		require.NoError(t, err)
		require.Equal(t, []string{"auth-server"}, fc.AuthServers)
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

	t.Run("Azure client ID", func(t *testing.T) {
		fc, err := MakeSampleFileConfig(SampleFlags{
			AzureClientID: "abcd1234",
		})
		require.NoError(t, err)
		require.Equal(t, "abcd1234", fc.JoinParams.Azure.ClientID)
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
