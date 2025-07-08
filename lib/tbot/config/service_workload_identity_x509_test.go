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

	"github.com/gravitational/teleport/lib/tbot/botfs"
	"github.com/gravitational/teleport/lib/tbot/internal/testutils"
)

func TestWorkloadIdentityX509Service_YAML(t *testing.T) {
	t.Parallel()

	dest := &DestinationMemory{}
	tests := []testutils.TestYAMLCase[WorkloadIdentityX509Service]{
		{
			Name: "full",
			In: WorkloadIdentityX509Service{
				Destination: dest,
				Selector: WorkloadIdentitySelector{
					Name: "my-workload-identity",
				},
				IncludeFederatedTrustBundles: true,
				CredentialLifetime: CredentialLifetime{
					TTL:             1 * time.Minute,
					RenewalInterval: 30 * time.Second,
				},
			},
		},
		{
			Name: "minimal",
			In: WorkloadIdentityX509Service{
				Destination: dest,
				Selector: WorkloadIdentitySelector{
					Name: "my-workload-identity",
				},
			},
		},
	}
	testutils.TestYAML(t, tests)
}

func TestWorkloadIdentityX509Service_CheckAndSetDefaults(t *testing.T) {
	t.Parallel()

	tests := []testutils.TestCheckAndSetDefaultsCase[*WorkloadIdentityX509Service]{
		{
			Name: "valid",
			In: func() *WorkloadIdentityX509Service {
				return &WorkloadIdentityX509Service{
					Selector: WorkloadIdentitySelector{
						Name: "my-workload-identity",
					},
					Destination: &DestinationDirectory{
						Path:     "/opt/machine-id",
						ACLs:     botfs.ACLOff,
						Symlinks: botfs.SymlinksInsecure,
					},
				}
			},
		},
		{
			Name: "valid with labels",
			In: func() *WorkloadIdentityX509Service {
				return &WorkloadIdentityX509Service{
					Selector: WorkloadIdentitySelector{
						Labels: map[string][]string{
							"key": {"value"},
						},
					},
					Destination: &DestinationDirectory{
						Path:     "/opt/machine-id",
						ACLs:     botfs.ACLOff,
						Symlinks: botfs.SymlinksInsecure,
					},
				}
			},
		},
		{
			Name: "missing selectors",
			In: func() *WorkloadIdentityX509Service {
				return &WorkloadIdentityX509Service{
					Selector: WorkloadIdentitySelector{},
					Destination: &DestinationDirectory{
						Path:     "/opt/machine-id",
						ACLs:     botfs.ACLOff,
						Symlinks: botfs.SymlinksInsecure,
					},
				}
			},
			WantErr: "one of ['name', 'labels'] must be set",
		},
		{
			Name: "too many selectors",
			In: func() *WorkloadIdentityX509Service {
				return &WorkloadIdentityX509Service{
					Selector: WorkloadIdentitySelector{
						Name: "my-workload-identity",
						Labels: map[string][]string{
							"key": {"value"},
						},
					},
					Destination: &DestinationDirectory{
						Path:     "/opt/machine-id",
						ACLs:     botfs.ACLOff,
						Symlinks: botfs.SymlinksInsecure,
					},
				}
			},
			WantErr: "at most one of ['name', 'labels'] can be set",
		},
		{
			Name: "missing destination",
			In: func() *WorkloadIdentityX509Service {
				return &WorkloadIdentityX509Service{
					Destination: nil,
				}
			},
			WantErr: "no destination configured for output",
		},
	}
	testutils.TestCheckAndSetDefaults(t, tests)
}
