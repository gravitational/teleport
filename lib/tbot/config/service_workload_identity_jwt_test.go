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
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/bot/testutils"
	"github.com/gravitational/teleport/lib/tbot/botfs"
	"github.com/gravitational/teleport/lib/tbot/services/workloadidentity"
)

func TestWorkloadIdentityJWTService_YAML(t *testing.T) {
	t.Parallel()

	dest := &destination.Memory{}
	tests := []testutils.TestYAMLCase[workloadidentity.JWTOutputConfig]{
		{
			Name: "full",
			In: workloadidentity.JWTOutputConfig{
				Destination: dest,
				Selector: bot.WorkloadIdentitySelector{
					Name: "my-workload-identity",
				},
				Audiences: []string{"audience1", "audience2"},
				CredentialLifetime: bot.CredentialLifetime{
					TTL:             time.Minute,
					RenewalInterval: 30 * time.Second,
				},
			},
		},
	}
	testutils.TestYAML(t, tests)
}

func TestWorkloadIdentityJWTService_CheckAndSetDefaults(t *testing.T) {
	t.Parallel()

	tests := []testutils.TestCheckAndSetDefaultsCase[*workloadidentity.JWTOutputConfig]{
		{
			Name: "valid",
			In: func() *workloadidentity.JWTOutputConfig {
				return &workloadidentity.JWTOutputConfig{
					Selector: bot.WorkloadIdentitySelector{
						Name: "my-workload-identity",
					},
					Destination: &destination.Directory{
						Path:     "/opt/machine-id",
						ACLs:     botfs.ACLOff,
						Symlinks: botfs.SymlinksInsecure,
					},
					Audiences: []string{"audience1", "audience2"},
				}
			},
		},
		{
			Name: "valid with labels",
			In: func() *workloadidentity.JWTOutputConfig {
				return &workloadidentity.JWTOutputConfig{
					Selector: bot.WorkloadIdentitySelector{
						Labels: map[string][]string{
							"key": {"value"},
						},
					},
					Destination: &destination.Directory{
						Path:     "/opt/machine-id",
						ACLs:     botfs.ACLOff,
						Symlinks: botfs.SymlinksInsecure,
					},
					Audiences: []string{"audience1", "audience2"},
				}
			},
		},
		{
			Name: "missing audience",
			In: func() *workloadidentity.JWTOutputConfig {
				return &workloadidentity.JWTOutputConfig{
					Selector: bot.WorkloadIdentitySelector{
						Name: "my-workload-identity",
					},
					Destination: &destination.Directory{
						Path:     "/opt/machine-id",
						ACLs:     botfs.ACLOff,
						Symlinks: botfs.SymlinksInsecure,
					},
				}
			},
			WantErr: "audiences: must have at least one value",
		},
		{
			Name: "missing selectors",
			In: func() *workloadidentity.JWTOutputConfig {
				return &workloadidentity.JWTOutputConfig{
					Selector: bot.WorkloadIdentitySelector{},
					Destination: &destination.Directory{
						Path:     "/opt/machine-id",
						ACLs:     botfs.ACLOff,
						Symlinks: botfs.SymlinksInsecure,
					},
					Audiences: []string{"audience1", "audience2"},
				}
			},
			WantErr: "one of ['name', 'labels'] must be set",
		},
		{
			Name: "too many selectors",
			In: func() *workloadidentity.JWTOutputConfig {
				return &workloadidentity.JWTOutputConfig{
					Selector: bot.WorkloadIdentitySelector{
						Name: "my-workload-identity",
						Labels: map[string][]string{
							"key": {"value"},
						},
					},
					Destination: &destination.Directory{
						Path:     "/opt/machine-id",
						ACLs:     botfs.ACLOff,
						Symlinks: botfs.SymlinksInsecure,
					},
					Audiences: []string{"audience1", "audience2"},
				}
			},
			WantErr: "at most one of ['name', 'labels'] can be set",
		},
		{
			Name: "missing destination",
			In: func() *workloadidentity.JWTOutputConfig {
				return &workloadidentity.JWTOutputConfig{
					Destination: nil,
					Selector: bot.WorkloadIdentitySelector{
						Name: "my-workload-identity",
					},
					Audiences: []string{"audience1", "audience2"},
				}
			},
			WantErr: "no destination configured for output",
		},
	}
	testutils.TestCheckAndSetDefaults(t, tests)
}
