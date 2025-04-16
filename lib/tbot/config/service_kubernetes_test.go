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
	"testing"
	"time"
)

func TestKubernetesOutput_YAML(t *testing.T) {
	dest := &DestinationMemory{}
	tests := []testYAMLCase[KubernetesOutput]{
		{
			name: "full",
			in: KubernetesOutput{
				Destination:       dest,
				Roles:             []string{"access"},
				KubernetesCluster: "k8s.example.com",
				CredentialLifetime: CredentialLifetime{
					TTL:             1 * time.Minute,
					RenewalInterval: 30 * time.Second,
				},
			},
		},
		{
			name: "minimal",
			in: KubernetesOutput{
				Destination:       dest,
				KubernetesCluster: "k8s.example.com",
			},
		},
	}
	testYAML(t, tests)
}

func TestKubernetesOutput_CheckAndSetDefaults(t *testing.T) {
	tests := []testCheckAndSetDefaultsCase[*KubernetesOutput]{
		{
			name: "valid",
			in: func() *KubernetesOutput {
				return &KubernetesOutput{
					Destination:       memoryDestForTest(),
					Roles:             []string{"access"},
					KubernetesCluster: "my-cluster",
				}
			},
		},
		{
			name: "missing destination",
			in: func() *KubernetesOutput {
				return &KubernetesOutput{
					Destination:       nil,
					KubernetesCluster: "my-cluster",
				}
			},
			wantErr: "no destination configured for output",
		},
		{
			name: "missing kubernetes_config",
			in: func() *KubernetesOutput {
				return &KubernetesOutput{
					Destination: memoryDestForTest(),
				}
			},
			wantErr: "kubernetes_cluster must not be empty",
		},
	}
	testCheckAndSetDefaults(t, tests)
}
