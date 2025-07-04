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

package k8s

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/bot/testutils"
)

func TestKubernetesOutput_YAML(t *testing.T) {
	dest := &destination.Memory{}
	tests := []testutils.TestYAMLCase[OutputV1Config]{
		{
			Name: "full",
			In: OutputV1Config{
				Destination:       dest,
				Roles:             []string{"access"},
				KubernetesCluster: "k8s.example.com",
				CredentialLifetime: bot.CredentialLifetime{
					TTL:             1 * time.Minute,
					RenewalInterval: 30 * time.Second,
				},
			},
		},
		{
			Name: "minimal",
			In: OutputV1Config{
				Destination:       dest,
				KubernetesCluster: "k8s.example.com",
			},
		},
	}
	testutils.TestYAML(t, tests)
}

func TestKubernetesOutput_CheckAndSetDefaults(t *testing.T) {
	tests := []testutils.TestCheckAndSetDefaultsCase[*OutputV1Config]{
		{
			Name: "valid",
			In: func() *OutputV1Config {
				return &OutputV1Config{
					Destination:       destination.NewMemory(),
					Roles:             []string{"access"},
					KubernetesCluster: "my-cluster",
				}
			},
		},
		{
			Name: "missing destination",
			In: func() *OutputV1Config {
				return &OutputV1Config{
					Destination:       nil,
					KubernetesCluster: "my-cluster",
				}
			},
			WantErr: "no destination configured for output",
		},
		{
			Name: "missing kubernetes_config",
			In: func() *OutputV1Config {
				return &OutputV1Config{
					Destination: destination.NewMemory(),
				}
			},
			WantErr: "kubernetes_cluster must not be empty",
		},
	}
	testutils.TestCheckAndSetDefaults(t, tests)
}
