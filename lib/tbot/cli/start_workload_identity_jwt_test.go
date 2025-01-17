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

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tbot/config"
)

func TestWorkloadIdentityJWTCommand(t *testing.T) {
	testStartConfigureCommand(t, NewWorkloadIdentityJWTCommand, []startConfigureTestCase{
		{
			name: "success",
			args: []string{
				"start",
				"workload-identity-jwt",
				"--destination=/bar",
				"--token=foo",
				"--join-method=github",
				"--proxy-server=example.com:443",
				"--label-selector=*=*,foo=bar",
				"--audience=foo",
			},
			assertConfig: func(t *testing.T, cfg *config.BotConfig) {
				require.Len(t, cfg.Services, 1)

				svc := cfg.Services[0]
				wis, ok := svc.(*config.WorkloadIdentityJWTService)
				require.True(t, ok)

				dir, ok := wis.Destination.(*config.DestinationDirectory)
				require.True(t, ok)
				require.Equal(t, "/bar", dir.Path)

				require.Equal(t, map[string][]string{
					"*":   {"*"},
					"foo": {"bar"},
				}, wis.Selector.Labels)

				require.Equal(t, []string{"foo"}, wis.Audiences)
			},
		},
		{
			name: "success name selector",
			args: []string{
				"start",
				"workload-identity-jwt",
				"--destination=/bar",
				"--token=foo",
				"--join-method=github",
				"--proxy-server=example.com:443",
				"--name-selector=jim",
				"--audience=foo",
				"--audience=bar",
			},
			assertConfig: func(t *testing.T, cfg *config.BotConfig) {
				require.Len(t, cfg.Services, 1)

				svc := cfg.Services[0]
				wis, ok := svc.(*config.WorkloadIdentityJWTService)
				require.True(t, ok)

				dir, ok := wis.Destination.(*config.DestinationDirectory)
				require.True(t, ok)
				require.Equal(t, "/bar", dir.Path)

				require.Equal(t, "jim", wis.Selector.Name)
				require.Equal(t, []string{"foo", "bar"}, wis.Audiences)

			},
		},
	})
}
