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

	"github.com/gravitational/teleport/lib/tbot/internal/testutils"
)

func TestKubernetesOutput_YAML(t *testing.T) {
	dest := &DestinationMemory{}
	tests := []testutils.TestYAMLCase[KubernetesOutput]{
		{
			Name: "full",
			In: KubernetesOutput{
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
			Name: "minimal",
			In: KubernetesOutput{
				Destination:       dest,
				KubernetesCluster: "k8s.example.com",
			},
		},
	}
	testutils.TestYAML(t, tests)
}

func TestKubernetesOutput_CheckAndSetDefaults(t *testing.T) {
	tests := []testutils.TestCheckAndSetDefaultsCase[*KubernetesOutput]{
		{
			Name: "valid",
			In: func() *KubernetesOutput {
				return &KubernetesOutput{
					Destination:       memoryDestForTest(),
					Roles:             []string{"access"},
					KubernetesCluster: "my-cluster",
				}
			},
		},
		{
			Name: "missing destination",
			In: func() *KubernetesOutput {
				return &KubernetesOutput{
					Destination:       nil,
					KubernetesCluster: "my-cluster",
				}
			},
			WantErr: "no destination configured for output",
		},
		{
			Name: "missing kubernetes_config",
			In: func() *KubernetesOutput {
				return &KubernetesOutput{
					Destination: memoryDestForTest(),
				}
			},
			WantErr: "kubernetes_cluster must not be empty",
		},
	}
	testutils.TestCheckAndSetDefaults(t, tests)
}
