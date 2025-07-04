/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/testutils"
	"github.com/gravitational/teleport/lib/tbot/services/legacyspiffe"
	"github.com/gravitational/teleport/lib/tbot/workloadidentity/workloadattest"
)

func ptr[T any](v T) *T {
	return &v
}

func TestSPIFFEWorkloadAPIService_YAML(t *testing.T) {
	t.Parallel()

	tests := []testutils.TestYAMLCase[legacyspiffe.WorkloadAPIConfig]{
		{
			Name: "full",
			In: legacyspiffe.WorkloadAPIConfig{
				Listen:     "unix:///var/run/spiffe.sock",
				JWTSVIDTTL: time.Minute * 5,
				Attestors: workloadattest.Config{
					Kubernetes: workloadattest.KubernetesAttestorConfig{
						Enabled: true,
						Kubelet: workloadattest.KubeletClientConfig{
							SecurePort: 12345,
							TokenPath:  "/path/to/token",
							CAPath:     "/path/to/ca.pem",
							SkipVerify: true,
							Anonymous:  true,
						},
					},
				},
				SVIDs: []legacyspiffe.SVIDRequestWithRules{
					{
						SVIDRequest: legacyspiffe.SVIDRequest{
							Path: "/foo",
							Hint: "hint",
							SANS: legacyspiffe.SVIDRequestSANs{
								DNS: []string{"example.com"},
								IP:  []string{"10.0.0.1", "10.42.0.1"},
							},
						},
						Rules: []legacyspiffe.SVIDRequestRule{
							{
								Unix: legacyspiffe.SVIDRequestRuleUnix{
									PID: ptr(100),
									UID: ptr(1000),
									GID: ptr(1234),
								},
							},
							{
								Unix: legacyspiffe.SVIDRequestRuleUnix{
									PID: ptr(100),
								},
								Kubernetes: legacyspiffe.SVIDRequestRuleKubernetes{
									Namespace:      "my-namespace",
									PodName:        "my-pod",
									ServiceAccount: "service-account",
								},
							},
						},
					},
				},
				CredentialLifetime: bot.CredentialLifetime{
					TTL:             1 * time.Minute,
					RenewalInterval: 30 * time.Second,
				},
			},
		},
		{
			Name: "minimal",
			In: legacyspiffe.WorkloadAPIConfig{
				Listen: "unix:///var/run/spiffe.sock",
				SVIDs: []legacyspiffe.SVIDRequestWithRules{
					{
						SVIDRequest: legacyspiffe.SVIDRequest{
							Path: "/foo",
						},
					},
				},
			},
		},
	}
	testutils.TestYAML(t, tests)
}

func TestSPIFFEWorkloadAPIService_CheckAndSetDefaults(t *testing.T) {
	t.Parallel()

	tests := []testutils.TestCheckAndSetDefaultsCase[*legacyspiffe.WorkloadAPIConfig]{
		{
			Name: "valid",
			In: func() *legacyspiffe.WorkloadAPIConfig {
				return &legacyspiffe.WorkloadAPIConfig{
					JWTSVIDTTL: time.Minute,
					Listen:     "unix:///var/run/spiffe.sock",
					SVIDs: []legacyspiffe.SVIDRequestWithRules{
						{
							SVIDRequest: legacyspiffe.SVIDRequest{
								Path: "/foo",
								Hint: "hint",
								SANS: legacyspiffe.SVIDRequestSANs{
									DNS: []string{"example.com"},
									IP:  []string{"10.0.0.1", "10.42.0.1"},
								},
							},
						},
					},
				}
			},
			Want: &legacyspiffe.WorkloadAPIConfig{
				JWTSVIDTTL: time.Minute,
				Listen:     "unix:///var/run/spiffe.sock",
				SVIDs: []legacyspiffe.SVIDRequestWithRules{
					{
						SVIDRequest: legacyspiffe.SVIDRequest{
							Path: "/foo",
							Hint: "hint",
							SANS: legacyspiffe.SVIDRequestSANs{
								DNS: []string{"example.com"},
								IP:  []string{"10.0.0.1", "10.42.0.1"},
							},
						},
					},
				},
				Attestors: workloadattest.Config{
					Unix: workloadattest.UnixAttestorConfig{
						BinaryHashMaxSizeBytes: workloadattest.DefaultBinaryHashMaxBytes,
					},
				},
			},
		},
		{
			Name: "missing path",
			In: func() *legacyspiffe.WorkloadAPIConfig {
				return &legacyspiffe.WorkloadAPIConfig{
					Listen: "unix:///var/run/spiffe.sock",
					SVIDs: []legacyspiffe.SVIDRequestWithRules{
						{
							SVIDRequest: legacyspiffe.SVIDRequest{
								Path: "",
								Hint: "hint",
								SANS: legacyspiffe.SVIDRequestSANs{
									DNS: []string{"example.com"},
									IP:  []string{"10.0.0.1", "10.42.0.1"},
								},
							},
						},
					},
				}
			},
			WantErr: "svid.path: should not be empty",
		},
		{
			Name: "path missing leading slash",
			In: func() *legacyspiffe.WorkloadAPIConfig {
				return &legacyspiffe.WorkloadAPIConfig{
					Listen: "unix:///var/run/spiffe.sock",
					SVIDs: []legacyspiffe.SVIDRequestWithRules{
						{
							SVIDRequest: legacyspiffe.SVIDRequest{
								Path: "foo",
								Hint: "hint",
								SANS: legacyspiffe.SVIDRequestSANs{
									DNS: []string{"example.com"},
									IP:  []string{"10.0.0.1", "10.42.0.1"},
								},
							},
						},
					},
				}
			},
			WantErr: "svid.path: should be prefixed with /",
		},
		{
			Name: "missing listen addr",
			In: func() *legacyspiffe.WorkloadAPIConfig {
				return &legacyspiffe.WorkloadAPIConfig{
					Listen: "",
					SVIDs: []legacyspiffe.SVIDRequestWithRules{
						{
							SVIDRequest: legacyspiffe.SVIDRequest{
								Path: "foo",
								Hint: "hint",
								SANS: legacyspiffe.SVIDRequestSANs{
									DNS: []string{"example.com"},
									IP:  []string{"10.0.0.1", "10.42.0.1"},
								},
							},
						},
					},
				}
			},
			WantErr: "listen: should not be empty",
		},
		{
			Name: "invalid ip",
			In: func() *legacyspiffe.WorkloadAPIConfig {
				return &legacyspiffe.WorkloadAPIConfig{
					Listen: "unix:///var/run/spiffe.sock",
					SVIDs: []legacyspiffe.SVIDRequestWithRules{
						{
							SVIDRequest: legacyspiffe.SVIDRequest{
								Path: "/foo",
								Hint: "hint",
								SANS: legacyspiffe.SVIDRequestSANs{
									DNS: []string{"example.com"},
									IP:  []string{"invalid ip"},
								},
							},
						},
					},
				}
			},
			WantErr: "ip_sans[0]: invalid IP address",
		},
	}
	testutils.TestCheckAndSetDefaults(t, tests)
}
