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

package awsra

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/botfs"
)

func TestWorkloadIdentityAWSRAService_YAML(t *testing.T) {
	t.Parallel()

	dest := &destination.Memory{}
	tests := []testYAMLCase[Config]{
		{
			name: "full",
			in: Config{
				Destination: dest,
				Selector: bot.WorkloadIdentitySelector{
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
			name: "simple",
			in: Config{
				Destination: dest,
				Selector: bot.WorkloadIdentitySelector{
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
	testYAML(t, tests)
}

func TestWorkloadIdentityAWSRAService_CheckAndSetDefaults(t *testing.T) {
	t.Parallel()

	tests := []testCheckAndSetDefaultsCase[*Config]{
		{
			name: "valid",
			in: func() *Config {
				return &Config{
					Selector: bot.WorkloadIdentitySelector{
						Name: "my-workload-identity",
					},
					Destination: &destination.Directory{
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
			want: &Config{
				Selector: bot.WorkloadIdentitySelector{
					Name: "my-workload-identity",
				},
				Destination: &destination.Directory{
					Path:     "/opt/machine-id",
					ACLs:     botfs.ACLOff,
					Symlinks: botfs.SymlinksInsecure,
				},
				SessionDuration:        DefaultAWSSessionDuration,
				SessionRenewalInterval: DefaultAWSSessionRenewalInterval,
				RoleARN:                "arn:aws:iam::123456789012:role/example-role",
				TrustAnchorARN:         "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/0000000-0000-0000-0000-000000000000",
				ProfileARN:             "arn:aws:rolesanywhere:us-east-1:123456789012:profile/0000000-0000-0000-0000-00000000000",
				Region:                 "us-east-1",
			},
		},
		{
			name: "valid with labels",
			in: func() *Config {
				return &Config{
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
			name: "missing selectors",
			in: func() *Config {
				return &Config{
					Selector: bot.WorkloadIdentitySelector{},
					Destination: &destination.Directory{
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
			wantErr: "one of ['name', 'labels'] must be set",
		},
		{
			name: "too many selectors",
			in: func() *Config {
				return &Config{
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
					RoleARN:        "arn:aws:iam::123456789012:role/example-role",
					TrustAnchorARN: "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/0000000-0000-0000-0000-000000000000",
					ProfileARN:     "arn:aws:rolesanywhere:us-east-1:123456789012:profile/0000000-0000-0000-0000-00000000000",
					Region:         "us-east-1",
				}
			},
			wantErr: "at most one of ['name', 'labels'] can be set",
		},
		{
			name: "missing destination",
			in: func() *Config {
				return &Config{
					Destination: nil,
					Selector: bot.WorkloadIdentitySelector{
						Name: "my-workload-identity",
					},
					RoleARN:        "arn:aws:iam::123456789012:role/example-role",
					TrustAnchorARN: "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/0000000-0000-0000-0000-000000000000",
					ProfileARN:     "arn:aws:rolesanywhere:us-east-1:123456789012:profile/0000000-0000-0000-0000-00000000000",
					Region:         "us-east-1",
				}
			},
			wantErr: "no destination configured for output",
		},
		{
			name: "missing role_arn",
			in: func() *Config {
				return &Config{
					Selector: bot.WorkloadIdentitySelector{
						Name: "my-workload-identity",
					},
					Destination: &destination.Directory{
						Path:     "/opt/machine-id",
						ACLs:     botfs.ACLOff,
						Symlinks: botfs.SymlinksInsecure,
					},
					TrustAnchorARN: "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/0000000-0000-0000-0000-000000000000",
					ProfileARN:     "arn:aws:rolesanywhere:us-east-1:123456789012:profile/0000000-0000-0000-0000-00000000000",
					Region:         "us-east-1",
				}
			},
			wantErr: "role_arn: must be set",
		},
		{
			name: "missing trust_anchor_arn",
			in: func() *Config {
				return &Config{
					Selector: bot.WorkloadIdentitySelector{
						Name: "my-workload-identity",
					},
					Destination: &destination.Directory{
						Path:     "/opt/machine-id",
						ACLs:     botfs.ACLOff,
						Symlinks: botfs.SymlinksInsecure,
					},
					RoleARN:    "arn:aws:iam::123456789012:role/example-role",
					ProfileARN: "arn:aws:rolesanywhere:us-east-1:123456789012:profile/0000000-0000-0000-0000-00000000000",
					Region:     "us-east-1",
				}
			},
			wantErr: "trust_anchor_arn: must be set",
		},
		{
			name: "missing profile_arn",
			in: func() *Config {
				return &Config{
					Selector: bot.WorkloadIdentitySelector{
						Name: "my-workload-identity",
					},
					Destination: &destination.Directory{
						Path:     "/opt/machine-id",
						ACLs:     botfs.ACLOff,
						Symlinks: botfs.SymlinksInsecure,
					},
					RoleARN:        "arn:aws:iam::123456789012:role/example-role",
					TrustAnchorARN: "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/0000000-0000-0000-0000-000000000000",
					Region:         "us-east-1",
				}
			},
			wantErr: "profile_arn: must be set",
		},
		{
			name: "invalid role arn",
			in: func() *Config {
				return &Config{
					Selector: bot.WorkloadIdentitySelector{
						Name: "my-workload-identity",
					},
					Destination: &destination.Directory{
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
			wantErr: "arn: invalid prefix",
		},
		{
			name: "invalid trust anchor arn",
			in: func() *Config {
				return &Config{
					Selector: bot.WorkloadIdentitySelector{
						Name: "my-workload-identity",
					},
					Destination: &destination.Directory{
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
			wantErr: "arn: invalid prefix",
		},
		{
			name: "invalid profile arn",
			in: func() *Config {
				return &Config{
					Selector: bot.WorkloadIdentitySelector{
						Name: "my-workload-identity",
					},
					Destination: &destination.Directory{
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
			wantErr: "arn: invalid prefix",
		},
		{
			name: "invalid region",
			in: func() *Config {
				return &Config{
					Selector: bot.WorkloadIdentitySelector{
						Name: "my-workload-identity",
					},
					Destination: &destination.Directory{
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
			wantErr: "validating region",
		},
	}
	testCheckAndSetDefaults(t, tests)
}
