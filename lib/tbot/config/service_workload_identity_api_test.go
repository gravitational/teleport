// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

	"github.com/gravitational/teleport/lib/tbot/spiffe/workloadattest"
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
				WorkloadIdentity: WorkloadIdentitySelector{
					Name: "my-workload-identity",
				},
			},
		},
		{
			name: "minimal",
			in: WorkloadIdentityAPIService{
				Listen: "tcp://0.0.0.0:4040",
				WorkloadIdentity: WorkloadIdentitySelector{
					Name: "my-workload-identity",
				},
			},
		},
	}
	testYAML(t, tests)
}
