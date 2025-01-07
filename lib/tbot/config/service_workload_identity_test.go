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

	"github.com/gravitational/teleport/lib/tbot/botfs"
)

func TestWorkloadIdentityX509Output_YAML(t *testing.T) {
	t.Parallel()

	dest := &DestinationMemory{}
	tests := []testYAMLCase[WorkloadIdentityX509Output]{
		{
			name: "full",
			in: WorkloadIdentityX509Output{
				Destination: dest,
				WorkloadIdentity: WorkloadIdentitySelector{
					Name: "my-workload-identity",
				},
				IncludeFederatedTrustBundles: true,
			},
		},
		{
			name: "minimal",
			in: WorkloadIdentityX509Output{
				Destination: dest,
				WorkloadIdentity: WorkloadIdentitySelector{
					Name: "my-workload-identity",
				},
			},
		},
	}
	testYAML(t, tests)
}

func TestWorkloadIdentityX509Output_CheckAndSetDefaults(t *testing.T) {
	t.Parallel()

	tests := []testCheckAndSetDefaultsCase[*WorkloadIdentityX509Output]{
		{
			name: "valid",
			in: func() *WorkloadIdentityX509Output {
				return &WorkloadIdentityX509Output{
					WorkloadIdentity: WorkloadIdentitySelector{
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
			name: "valid with labels",
			in: func() *WorkloadIdentityX509Output {
				return &WorkloadIdentityX509Output{
					WorkloadIdentity: WorkloadIdentitySelector{
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
			name: "missing selectors",
			in: func() *WorkloadIdentityX509Output {
				return &WorkloadIdentityX509Output{
					WorkloadIdentity: WorkloadIdentitySelector{},
					Destination: &DestinationDirectory{
						Path:     "/opt/machine-id",
						ACLs:     botfs.ACLOff,
						Symlinks: botfs.SymlinksInsecure,
					},
				}
			},
			wantErr: "one of ['name', 'labels'] must be set",
		},
		{
			name: "too many selectors",
			in: func() *WorkloadIdentityX509Output {
				return &WorkloadIdentityX509Output{
					WorkloadIdentity: WorkloadIdentitySelector{
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
			wantErr: "at most one of ['name', 'labels'] can be set",
		},
		{
			name: "missing destination",
			in: func() *WorkloadIdentityX509Output {
				return &WorkloadIdentityX509Output{
					Destination: nil,
				}
			},
			wantErr: "no destination configured for output",
		},
	}
	testCheckAndSetDefaults(t, tests)
}
