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
	"encoding/base64"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestAuthenticationSection(t *testing.T) {
	tests := []struct {
		comment                 string
		inConfigString          string
		outAuthenticationConfig *AuthenticationConfig
	}{
		{
			`0 - local with otp`,

			`
auth_service:
  authentication:
    type: local
    second_factor: otp
`,
			&AuthenticationConfig{
				Type:         "local",
				SecondFactor: "otp",
			},
		},
		{
			`1 - local auth without otp`,

			`
auth_service:
  authentication:
    type: local
    second_factor: off
`,
			&AuthenticationConfig{
				Type:         "local",
				SecondFactor: "off",
			},
		},
		{
			`2 - local auth with u2f`,

			`
auth_service:
   authentication:
       type: local
       second_factor: u2f
       u2f:
           app_id: https://graviton:3080
           facets:
           - https://graviton:3080
           device_attestation_cas:
           - testdata/u2f_attestation_ca.pam
           - |
             -----BEGIN CERTIFICATE-----
             fake certificate
             -----END CERTIFICATE-----
`,
			&AuthenticationConfig{
				Type:         "local",
				SecondFactor: "u2f",
				U2F: &UniversalSecondFactor{
					AppID: "https://graviton:3080",
					Facets: []string{
						"https://graviton:3080",
					},
					DeviceAttestationCAs: []string{
						"testdata/u2f_attestation_ca.pam",
						`-----BEGIN CERTIFICATE-----
fake certificate
-----END CERTIFICATE-----
`,
					},
				},
			},
		},
	}

	// run tests
	for _, tt := range tests {
		t.Run(tt.comment, func(t *testing.T) {
			encodedConfigString := base64.StdEncoding.EncodeToString([]byte(tt.inConfigString))

			fc, err := ReadFromString(encodedConfigString)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(fc.Auth.Authentication, tt.outAuthenticationConfig))
		})
	}
}

// minimalConfigFile is a minimal subset of a teleport config file that can be
// mutated programatically by test cases and then re-serialised to test the
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

// editConfig takes the minimal YAML configuration file, de-serialises it into a
// nested key-value dictionary suitable for manipulation by a test case,
// passes that dictionary to the caller-supplied mutator and then re-serialises
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
		desc          string
		mutate        func(cfgMap)
		expectError   require.ErrorAssertionFunc
		expectEnabled require.BoolAssertionFunc
		expectIdleMsg require.ValueAssertionFunc
	}{
		{
			desc:          "Default",
			mutate:        func(cfg cfgMap) {},
			expectError:   require.NoError,
			expectEnabled: require.True,
			expectIdleMsg: require.Empty,
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
		})
	}
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
