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

func TestWorkloadIdentityJWTService_YAML(t *testing.T) {
	t.Parallel()

	dest := &DestinationMemory{}
	tests := []testYAMLCase[WorkloadIdentityJWTService]{
		{
			name: "full",
			in: WorkloadIdentityJWTService{
				Destination: dest,
				Selector: WorkloadIdentitySelector{
					Name: "my-workload-identity",
				},
				Audiences: []string{"audience1", "audience2"},
			},
		},
	}
	testYAML(t, tests)
}

func TestWorkloadIdentityJWTService_CheckAndSetDefaults(t *testing.T) {
	t.Parallel()

	tests := []testCheckAndSetDefaultsCase[*WorkloadIdentityJWTService]{
		{
			name: "valid",
			in: func() *WorkloadIdentityJWTService {
				return &WorkloadIdentityJWTService{
					Selector: WorkloadIdentitySelector{
						Name: "my-workload-identity",
					},
					Destination: &DestinationDirectory{
						Path:     "/opt/machine-id",
						ACLs:     botfs.ACLOff,
						Symlinks: botfs.SymlinksInsecure,
					},
					Audiences: []string{"audience1", "audience2"},
				}
			},
		},
		{
			name: "valid with labels",
			in: func() *WorkloadIdentityJWTService {
				return &WorkloadIdentityJWTService{
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
					Audiences: []string{"audience1", "audience2"},
				}
			},
		},
		{
			name: "missing audience",
			in: func() *WorkloadIdentityJWTService {
				return &WorkloadIdentityJWTService{
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
			wantErr: "audiences: must have at least one value",
		},
		{
			name: "missing selectors",
			in: func() *WorkloadIdentityJWTService {
				return &WorkloadIdentityJWTService{
					Selector: WorkloadIdentitySelector{},
					Destination: &DestinationDirectory{
						Path:     "/opt/machine-id",
						ACLs:     botfs.ACLOff,
						Symlinks: botfs.SymlinksInsecure,
					},
					Audiences: []string{"audience1", "audience2"},
				}
			},
			wantErr: "one of ['name', 'labels'] must be set",
		},
		{
			name: "too many selectors",
			in: func() *WorkloadIdentityJWTService {
				return &WorkloadIdentityJWTService{
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
					Audiences: []string{"audience1", "audience2"},
				}
			},
			wantErr: "at most one of ['name', 'labels'] can be set",
		},
		{
			name: "missing destination",
			in: func() *WorkloadIdentityJWTService {
				return &WorkloadIdentityJWTService{
					Destination: nil,
					Selector: WorkloadIdentitySelector{
						Name: "my-workload-identity",
					},
					Audiences: []string{"audience1", "audience2"},
				}
			},
			wantErr: "no destination configured for output",
		},
	}
	testCheckAndSetDefaults(t, tests)
}
