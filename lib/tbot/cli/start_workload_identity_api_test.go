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

func TestNewWorkloadIdentityAPICommand(t *testing.T) {
	testStartConfigureCommand(t, NewWorkloadIdentityAPICommand, []startConfigureTestCase{
		{
			name: "success",
			args: []string{
				"start",
				"workload-identity-api",
				"--token=foo",
				"--join-method=github",
				"--proxy-server=example.com:443",
				"--listen=tcp://0.0.0.0:8080",
				"--label-selector=*=*,foo=bar",
			},
			assertConfig: func(t *testing.T, cfg *config.BotConfig) {
				require.Len(t, cfg.Services, 1)

				svc := cfg.Services[0]
				wis, ok := svc.(*config.WorkloadIdentityAPIService)
				require.True(t, ok)
				require.Equal(t, "tcp://0.0.0.0:8080", wis.Listen)
				require.Equal(t, map[string][]string{
					"*":   {"*"},
					"foo": {"bar"},
				}, wis.Selector.Labels)
			},
		},
		{
			name: "success name selector",
			args: []string{
				"start",
				"workload-identity-api",
				"--token=foo",
				"--join-method=github",
				"--proxy-server=example.com:443",
				"--listen=unix:///opt/workload.sock",
				"--name-selector=jim",
			},
			assertConfig: func(t *testing.T, cfg *config.BotConfig) {
				require.Len(t, cfg.Services, 1)

				svc := cfg.Services[0]
				wis, ok := svc.(*config.WorkloadIdentityAPIService)
				require.True(t, ok)
				require.Equal(t, "unix:///opt/workload.sock", wis.Listen)
				require.Equal(t, "jim", wis.Selector.Name)
			},
		},
	})
}
