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

package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/services/awsra"
)

func TestWorkloadIdentityAWSRACommand(t *testing.T) {
	testStartConfigureCommand(t, NewWorkloadIdentityAWSRACommand, []startConfigureTestCase{
		{
			name: "success",
			args: []string{
				"start",
				"workload-identity-aws-roles-anywhere",
				"--destination=/bar",
				"--token=foo",
				"--join-method=github",
				"--proxy-server=example.com:443",
				"--label-selector=*=*,foo=bar",
				"--role-arn=arn:aws:iam::123456789012:role/example-role",
				"--trust-anchor-arn=arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/0000000-0000-0000-0000-000000000000",
				"--profile-arn=arn:aws:rolesanywhere:us-east-1:123456789012:profile/0000000-0000-0000-0000-00000000000",
				"--region=us-east-1",
				"--session-duration=2h",
				"--session-renewal-interval=30m",
			},
			assertConfig: func(t *testing.T, cfg *config.BotConfig) {
				require.Len(t, cfg.Services, 1)

				svc := cfg.Services[0]
				wis, ok := svc.(*awsra.Config)
				require.True(t, ok)

				dir, ok := wis.Destination.(*destination.Directory)
				require.True(t, ok)
				require.Equal(t, "/bar", dir.Path)

				require.Equal(t, map[string][]string{
					"*":   {"*"},
					"foo": {"bar"},
				}, wis.Selector.Labels)
				require.Equal(t, "arn:aws:iam::123456789012:role/example-role", wis.RoleARN)
				require.Equal(
					t,
					"arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/0000000-0000-0000-0000-000000000000",
					wis.TrustAnchorARN,
				)
				require.Equal(
					t,
					"arn:aws:rolesanywhere:us-east-1:123456789012:profile/0000000-0000-0000-0000-00000000000",
					wis.ProfileARN,
				)
				require.Equal(t, "us-east-1", wis.Region)

				require.Equal(t, 2*time.Hour, wis.SessionDuration)
				require.Equal(t, 30*time.Minute, wis.SessionRenewalInterval)
			},
		},
		{
			name: "success name selector",
			args: []string{
				"start",
				"workload-identity-aws-roles-anywhere",
				"--destination=/bar",
				"--token=foo",
				"--join-method=github",
				"--proxy-server=example.com:443",
				"--name-selector=jim",
				"--role-arn=arn:aws:iam::123456789012:role/example-role",
				"--trust-anchor-arn=arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/0000000-0000-0000-0000-000000000000",
				"--profile-arn=arn:aws:rolesanywhere:us-east-1:123456789012:profile/0000000-0000-0000-0000-00000000000",
				"--region=us-east-1",
			},
			assertConfig: func(t *testing.T, cfg *config.BotConfig) {
				require.Len(t, cfg.Services, 1)

				svc := cfg.Services[0]
				wis, ok := svc.(*awsra.Config)
				require.True(t, ok)

				dir, ok := wis.Destination.(*destination.Directory)
				require.True(t, ok)
				require.Equal(t, "/bar", dir.Path)

				require.Equal(t, "jim", wis.Selector.Name)
				require.Equal(t, "arn:aws:iam::123456789012:role/example-role", wis.RoleARN)
				require.Equal(
					t,
					"arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/0000000-0000-0000-0000-000000000000",
					wis.TrustAnchorARN,
				)
				require.Equal(
					t,
					"arn:aws:rolesanywhere:us-east-1:123456789012:profile/0000000-0000-0000-0000-00000000000",
					wis.ProfileARN,
				)
				require.Equal(t, "us-east-1", wis.Region)
			},
		},
	})
}
