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

	"github.com/gravitational/teleport/lib/tbot/workloadidentity/workloadattest"
)

func TestWorkloadIdentityAPIService_YAML(t *testing.T) {
	t.Parallel()

	tests := []testYAMLCase[WorkloadIdentityAPIService]{
		{
			name: "full",
			in: WorkloadIdentityAPIService{
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
				Selector: WorkloadIdentitySelector{
					Name: "my-workload-identity",
				},
				CredentialLifetime: CredentialLifetime{
					TTL:             1 * time.Minute,
					RenewalInterval: 30 * time.Second,
				},
			},
		},
		{
			name: "minimal",
			in: WorkloadIdentityAPIService{
				Listen: "tcp://0.0.0.0:4040",
				Selector: WorkloadIdentitySelector{
					Name: "my-workload-identity",
				},
			},
		},
	}
	testYAML(t, tests)
}

func TestWorkloadIdentityAPIService_CheckAndSetDefaults(t *testing.T) {
	t.Parallel()

	tests := []testCheckAndSetDefaultsCase[*WorkloadIdentityAPIService]{
		{
			name: "valid",
			in: func() *WorkloadIdentityAPIService {
				return &WorkloadIdentityAPIService{
					Selector: WorkloadIdentitySelector{
						Name: "my-workload-identity",
					},
					Listen: "tcp://0.0.0.0:4040",
				}
			},
		},
		{
			name: "valid with labels",
			in: func() *WorkloadIdentityAPIService {
				return &WorkloadIdentityAPIService{
					Selector: WorkloadIdentitySelector{
						Labels: map[string][]string{
							"key": {"value"},
						},
					},
					Listen: "tcp://0.0.0.0:4040",
				}
			},
		},
		{
			name: "missing selectors",
			in: func() *WorkloadIdentityAPIService {
				return &WorkloadIdentityAPIService{
					Selector: WorkloadIdentitySelector{},
					Listen:   "tcp://0.0.0.0:4040",
				}
			},
			wantErr: "one of ['name', 'labels'] must be set",
		},
		{
			name: "too many selectors",
			in: func() *WorkloadIdentityAPIService {
				return &WorkloadIdentityAPIService{
					Selector: WorkloadIdentitySelector{
						Name: "my-workload-identity",
						Labels: map[string][]string{
							"key": {"value"},
						},
					},
					Listen: "tcp://0.0.0.0:4040",
				}
			},
			wantErr: "at most one of ['name', 'labels'] can be set",
		},
		{
			name: "missing listen",
			in: func() *WorkloadIdentityAPIService {
				return &WorkloadIdentityAPIService{
					Selector: WorkloadIdentitySelector{
						Name: "my-workload-identity",
					},
				}
			},
			wantErr: "listen: should not be empty",
		},
	}
	testCheckAndSetDefaults(t, tests)
}
