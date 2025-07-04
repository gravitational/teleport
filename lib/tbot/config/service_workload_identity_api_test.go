// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package config

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/testutils"
	"github.com/gravitational/teleport/lib/tbot/services/workloadidentity"
	"github.com/gravitational/teleport/lib/tbot/workloadidentity/workloadattest"
)

func TestWorkloadIdentityAPIService_YAML(t *testing.T) {
	t.Parallel()

	tests := []testutils.TestYAMLCase[workloadidentity.WorkloadAPIConfig]{
		{
			Name: "full",
			In: workloadidentity.WorkloadAPIConfig{
				Listen: "tcp://0.0.0.0:4040",
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
				Selector: bot.WorkloadIdentitySelector{
					Name: "my-workload-identity",
				},
				CredentialLifetime: bot.CredentialLifetime{
					TTL:             1 * time.Minute,
					RenewalInterval: 30 * time.Second,
				},
			},
		},
		{
			Name: "minimal",
			In: workloadidentity.WorkloadAPIConfig{
				Listen: "tcp://0.0.0.0:4040",
				Selector: bot.WorkloadIdentitySelector{
					Name: "my-workload-identity",
				},
			},
		},
	}
	testutils.TestYAML(t, tests)
}

func TestWorkloadIdentityAPIService_CheckAndSetDefaults(t *testing.T) {
	t.Parallel()

	tests := []testutils.TestCheckAndSetDefaultsCase[*workloadidentity.WorkloadAPIConfig]{
		{
			Name: "valid",
			In: func() *workloadidentity.WorkloadAPIConfig {
				return &workloadidentity.WorkloadAPIConfig{
					Selector: bot.WorkloadIdentitySelector{
						Name: "my-workload-identity",
					},
					Listen: "tcp://0.0.0.0:4040",
				}
			},
			Want: &workloadidentity.WorkloadAPIConfig{
				Selector: bot.WorkloadIdentitySelector{
					Name: "my-workload-identity",
				},
				Listen: "tcp://0.0.0.0:4040",
				Attestors: workloadattest.Config{
					Unix: workloadattest.UnixAttestorConfig{
						BinaryHashMaxSizeBytes: workloadattest.DefaultBinaryHashMaxBytes,
					},
				},
			},
		},
		{
			Name: "valid with labels",
			In: func() *workloadidentity.WorkloadAPIConfig {
				return &workloadidentity.WorkloadAPIConfig{
					Selector: bot.WorkloadIdentitySelector{
						Labels: map[string][]string{
							"key": {"value"},
						},
					},
					Listen: "tcp://0.0.0.0:4040",
				}
			},
			Want: &workloadidentity.WorkloadAPIConfig{
				Selector: bot.WorkloadIdentitySelector{
					Labels: map[string][]string{
						"key": {"value"},
					},
				},
				Listen: "tcp://0.0.0.0:4040",
				Attestors: workloadattest.Config{
					Unix: workloadattest.UnixAttestorConfig{
						BinaryHashMaxSizeBytes: workloadattest.DefaultBinaryHashMaxBytes,
					},
				},
			},
		},
		{
			Name: "missing selectors",
			In: func() *workloadidentity.WorkloadAPIConfig {
				return &workloadidentity.WorkloadAPIConfig{
					Selector: bot.WorkloadIdentitySelector{},
					Listen:   "tcp://0.0.0.0:4040",
				}
			},
			WantErr: "one of ['name', 'labels'] must be set",
		},
		{
			Name: "too many selectors",
			In: func() *workloadidentity.WorkloadAPIConfig {
				return &workloadidentity.WorkloadAPIConfig{
					Selector: bot.WorkloadIdentitySelector{
						Name: "my-workload-identity",
						Labels: map[string][]string{
							"key": {"value"},
						},
					},
					Listen: "tcp://0.0.0.0:4040",
				}
			},
			WantErr: "at most one of ['name', 'labels'] can be set",
		},
		{
			Name: "missing listen",
			In: func() *workloadidentity.WorkloadAPIConfig {
				return &workloadidentity.WorkloadAPIConfig{
					Selector: bot.WorkloadIdentitySelector{
						Name: "my-workload-identity",
					},
				}
			},
			WantErr: "listen: should not be empty",
		},
	}
	testutils.TestCheckAndSetDefaults(t, tests)
}
