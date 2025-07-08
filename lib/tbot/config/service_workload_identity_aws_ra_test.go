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

func TestWorkloadIdentityAWSRAService_YAML(t *testing.T) {
	t.Parallel()

	dest := &DestinationMemory{}
	tests := []testutils.TestYAMLCase[WorkloadIdentityAWSRAService]{
		{
			Name: "full",
			In: WorkloadIdentityAWSRAService{
				Destination: dest,
				Selector: WorkloadIdentitySelector{
					Name: "my-workload-identity",
				},
				SessionDuration:         time.Minute * 59,
				SessionRenewalInterval:  time.Minute * 29,
				RoleARN:                 "arn:aws:iam::123456789012:role/example-role",
				TrustAnchorARN:          "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/0000000-0000-0000-0000-000000000000",
				ProfileARN:              "arn:aws:rolesanywhere:us-east-1:123456789012:profile/0000000-0000-0000-0000-00000000000",
				Region:                  "us-east-1",
				CredentialProfileName:   "my-profile",
				ArtifactName:            "my-artifact.toml",
				OverwriteCredentialFile: true,
			},
		},
		{
			Name: "simple",
			In: WorkloadIdentityAWSRAService{
				Destination: dest,
				Selector: WorkloadIdentitySelector{
					Name: "my-workload-identity",
				},
				SessionDuration:        time.Minute * 59,
				SessionRenewalInterval: time.Minute * 29,
				RoleARN:                "arn:aws:iam::123456789012:role/example-role",
				TrustAnchorARN:         "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/0000000-0000-0000-0000-000000000000",
				ProfileARN:             "arn:aws:rolesanywhere:us-east-1:123456789012:profile/0000000-0000-0000-0000-00000000000",
				Region:                 "us-east-1",
			},
		},
	}
	testutils.TestYAML(t, tests)
}

func TestWorkloadIdentityAWSRAService_CheckAndSetDefaults(t *testing.T) {
	t.Parallel()

	tests := []testutils.TestCheckAndSetDefaultsCase[*WorkloadIdentityAWSRAService]{
		{
			Name: "valid",
			In: func() *WorkloadIdentityAWSRAService {
				return &WorkloadIdentityAWSRAService{
					Selector: WorkloadIdentitySelector{
						Name: "my-workload-identity",
					},
					Destination: &DestinationDirectory{
						Path:     "/opt/machine-id",
						ACLs:     botfs.ACLOff,
						Symlinks: botfs.SymlinksInsecure,
					},
					RoleARN:        "arn:aws:iam::123456789012:role/example-role",
					TrustAnchorARN: "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/0000000-0000-0000-0000-000000000000",
					ProfileARN:     "arn:aws:rolesanywhere:us-east-1:123456789012:profile/0000000-0000-0000-0000-00000000000",
					Region:         "us-east-1",
				}
			},
			Want: &WorkloadIdentityAWSRAService{
				Selector: WorkloadIdentitySelector{
					Name: "my-workload-identity",
				},
				Destination: &DestinationDirectory{
					Path:     "/opt/machine-id",
					ACLs:     botfs.ACLOff,
					Symlinks: botfs.SymlinksInsecure,
				},
				SessionDuration:        defaultAWSSessionDuration,
				SessionRenewalInterval: defaultAWSSessionRenewalInterval,
				RoleARN:                "arn:aws:iam::123456789012:role/example-role",
				TrustAnchorARN:         "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/0000000-0000-0000-0000-000000000000",
				ProfileARN:             "arn:aws:rolesanywhere:us-east-1:123456789012:profile/0000000-0000-0000-0000-00000000000",
				Region:                 "us-east-1",
			},
		},
		{
			Name: "valid with labels",
			In: func() *WorkloadIdentityAWSRAService {
				return &WorkloadIdentityAWSRAService{
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
					SessionDuration:        1 * time.Hour,
					SessionRenewalInterval: 30 * time.Minute,
					RoleARN:                "arn:aws:iam::123456789012:role/example-role",
					TrustAnchorARN:         "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/0000000-0000-0000-0000-000000000000",
					ProfileARN:             "arn:aws:rolesanywhere:us-east-1:123456789012:profile/0000000-0000-0000-0000-00000000000",
					Region:                 "us-east-1",
				}
			},
		},
		{
			Name: "missing selectors",
			In: func() *WorkloadIdentityAWSRAService {
				return &WorkloadIdentityAWSRAService{
					Selector: WorkloadIdentitySelector{},
					Destination: &DestinationDirectory{
						Path:     "/opt/machine-id",
						ACLs:     botfs.ACLOff,
						Symlinks: botfs.SymlinksInsecure,
					},
					RoleARN:        "arn:aws:iam::123456789012:role/example-role",
					TrustAnchorARN: "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/0000000-0000-0000-0000-000000000000",
					ProfileARN:     "arn:aws:rolesanywhere:us-east-1:123456789012:profile/0000000-0000-0000-0000-00000000000",
					Region:         "us-east-1",
				}
			},
			WantErr: "one of ['name', 'labels'] must be set",
		},
		{
			Name: "too many selectors",
			In: func() *WorkloadIdentityAWSRAService {
				return &WorkloadIdentityAWSRAService{
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
					RoleARN:        "arn:aws:iam::123456789012:role/example-role",
					TrustAnchorARN: "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/0000000-0000-0000-0000-000000000000",
					ProfileARN:     "arn:aws:rolesanywhere:us-east-1:123456789012:profile/0000000-0000-0000-0000-00000000000",
					Region:         "us-east-1",
				}
			},
			WantErr: "at most one of ['name', 'labels'] can be set",
		},
		{
			Name: "missing destination",
			In: func() *WorkloadIdentityAWSRAService {
				return &WorkloadIdentityAWSRAService{
					Destination: nil,
					Selector: WorkloadIdentitySelector{
						Name: "my-workload-identity",
					},
					RoleARN:        "arn:aws:iam::123456789012:role/example-role",
					TrustAnchorARN: "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/0000000-0000-0000-0000-000000000000",
					ProfileARN:     "arn:aws:rolesanywhere:us-east-1:123456789012:profile/0000000-0000-0000-0000-00000000000",
					Region:         "us-east-1",
				}
			},
			WantErr: "no destination configured for output",
		},
		{
			Name: "missing role_arn",
			In: func() *WorkloadIdentityAWSRAService {
				return &WorkloadIdentityAWSRAService{
					Selector: WorkloadIdentitySelector{
						Name: "my-workload-identity",
					},
					Destination: &DestinationDirectory{
						Path:     "/opt/machine-id",
						ACLs:     botfs.ACLOff,
						Symlinks: botfs.SymlinksInsecure,
					},
					TrustAnchorARN: "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/0000000-0000-0000-0000-000000000000",
					ProfileARN:     "arn:aws:rolesanywhere:us-east-1:123456789012:profile/0000000-0000-0000-0000-00000000000",
					Region:         "us-east-1",
				}
			},
			WantErr: "role_arn: must be set",
		},
		{
			Name: "missing trust_anchor_arn",
			In: func() *WorkloadIdentityAWSRAService {
				return &WorkloadIdentityAWSRAService{
					Selector: WorkloadIdentitySelector{
						Name: "my-workload-identity",
					},
					Destination: &DestinationDirectory{
						Path:     "/opt/machine-id",
						ACLs:     botfs.ACLOff,
						Symlinks: botfs.SymlinksInsecure,
					},
					RoleARN:    "arn:aws:iam::123456789012:role/example-role",
					ProfileARN: "arn:aws:rolesanywhere:us-east-1:123456789012:profile/0000000-0000-0000-0000-00000000000",
					Region:     "us-east-1",
				}
			},
			WantErr: "trust_anchor_arn: must be set",
		},
		{
			Name: "missing profile_arn",
			In: func() *WorkloadIdentityAWSRAService {
				return &WorkloadIdentityAWSRAService{
					Selector: WorkloadIdentitySelector{
						Name: "my-workload-identity",
					},
					Destination: &DestinationDirectory{
						Path:     "/opt/machine-id",
						ACLs:     botfs.ACLOff,
						Symlinks: botfs.SymlinksInsecure,
					},
					RoleARN:        "arn:aws:iam::123456789012:role/example-role",
					TrustAnchorARN: "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/0000000-0000-0000-0000-000000000000",
					Region:         "us-east-1",
				}
			},
			WantErr: "profile_arn: must be set",
		},
		{
			Name: "invalid role arn",
			In: func() *WorkloadIdentityAWSRAService {
				return &WorkloadIdentityAWSRAService{
					Selector: WorkloadIdentitySelector{
						Name: "my-workload-identity",
					},
					Destination: &DestinationDirectory{
						Path:     "/opt/machine-id",
						ACLs:     botfs.ACLOff,
						Symlinks: botfs.SymlinksInsecure,
					},
					RoleARN:        "foo",
					TrustAnchorARN: "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/0000000-0000-0000-0000-000000000000",
					ProfileARN:     "arn:aws:rolesanywhere:us-east-1:123456789012:profile/0000000-0000-0000-0000-00000000000",
					Region:         "us-east-1",
				}
			},
			WantErr: "arn: invalid prefix",
		},
		{
			Name: "invalid trust anchor arn",
			In: func() *WorkloadIdentityAWSRAService {
				return &WorkloadIdentityAWSRAService{
					Selector: WorkloadIdentitySelector{
						Name: "my-workload-identity",
					},
					Destination: &DestinationDirectory{
						Path:     "/opt/machine-id",
						ACLs:     botfs.ACLOff,
						Symlinks: botfs.SymlinksInsecure,
					},
					RoleARN:        "arn:aws:iam::123456789012:role/example-role",
					TrustAnchorARN: "foo",
					ProfileARN:     "arn:aws:rolesanywhere:us-east-1:123456789012:profile/0000000-0000-0000-0000-00000000000",
					Region:         "us-east-1",
				}
			},
			WantErr: "arn: invalid prefix",
		},
		{
			Name: "invalid profile arn",
			In: func() *WorkloadIdentityAWSRAService {
				return &WorkloadIdentityAWSRAService{
					Selector: WorkloadIdentitySelector{
						Name: "my-workload-identity",
					},
					Destination: &DestinationDirectory{
						Path:     "/opt/machine-id",
						ACLs:     botfs.ACLOff,
						Symlinks: botfs.SymlinksInsecure,
					},
					RoleARN:        "arn:aws:iam::123456789012:role/example-role",
					TrustAnchorARN: "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/0000000-0000-0000-0000-000000000000",
					ProfileARN:     "foo",
					Region:         "us-east-1",
				}
			},
			WantErr: "arn: invalid prefix",
		},
		{
			Name: "invalid region",
			In: func() *WorkloadIdentityAWSRAService {
				return &WorkloadIdentityAWSRAService{
					Selector: WorkloadIdentitySelector{
						Name: "my-workload-identity",
					},
					Destination: &DestinationDirectory{
						Path:     "/opt/machine-id",
						ACLs:     botfs.ACLOff,
						Symlinks: botfs.SymlinksInsecure,
					},
					RoleARN:        "arn:aws:iam::123456789012:role/example-role",
					TrustAnchorARN: "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/0000000-0000-0000-0000-000000000000",
					ProfileARN:     "arn:aws:rolesanywhere:us-east-1:123456789012:profile/0000000-0000-0000-0000-00000000000",
					Region:         "us-east-1!!!!",
				}
			},
			WantErr: "validating region",
		},
	}
	testutils.TestCheckAndSetDefaults(t, tests)
}
