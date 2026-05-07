// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package identity

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
)

func TestPIVAgentService_YAML(t *testing.T) {
	t.Parallel()

	tests := []testYAMLCase[PIVAgentConfig]{
		{
			name: "full",
			in: PIVAgentConfig{
				Name: "piv-agent",
				Destination: &destination.Directory{
					Path: "/opt/machine-id",
				},
				Roles:        []string{"access"},
				Cluster:      "leaf.example.com",
				AllowReissue: true,
				CredentialLifetime: bot.CredentialLifetime{
					TTL:             1 * time.Minute,
					RenewalInterval: 30 * time.Second,
				},
			},
		},
		{
			name: "minimal",
			in: PIVAgentConfig{
				Destination: &destination.Directory{
					Path: "/opt/machine-id",
				},
			},
		},
	}

	testYAML(t, tests)
}

func TestPIVAgentService_CheckAndSetDefaults(t *testing.T) {
	t.Parallel()

	tests := []testCheckAndSetDefaultsCase[*PIVAgentConfig]{
		{
			name: "valid",
			in: func() *PIVAgentConfig {
				return &PIVAgentConfig{
					Destination: &destination.Directory{
						Path: "/opt/machine-id",
					},
				}
			},
		},
		{
			name: "valid with roles",
			in: func() *PIVAgentConfig {
				return &PIVAgentConfig{
					Destination: &destination.Directory{
						Path: "/opt/machine-id",
					},
					Roles: []string{"access"},
				}
			},
		},
		{
			name: "valid with delegation session id",
			in: func() *PIVAgentConfig {
				return &PIVAgentConfig{
					Destination: &destination.Directory{
						Path: "/opt/machine-id",
					},
					DelegationSessionID: "8a50ba48-2fad-4c2c-a8ce-f48bc18db9ee",
				}
			},
		},
		{
			name: "missing destination",
			in: func() *PIVAgentConfig {
				return &PIVAgentConfig{}
			},
			wantErr: "destination: is required",
		},
		{
			name: "wrong destination type",
			in: func() *PIVAgentConfig {
				return &PIVAgentConfig{
					Destination: &destination.Memory{},
				}
			},
			wantErr: "destination: must be a filesystem directory",
		},
		{
			name: "delegation session id conflicts with roles",
			in: func() *PIVAgentConfig {
				return &PIVAgentConfig{
					Destination: &destination.Directory{
						Path: "/opt/machine-id",
					},
					Roles:               []string{"access"},
					DelegationSessionID: "8a50ba48-2fad-4c2c-a8ce-f48bc18db9ee",
				}
			},
			wantErr: "delegation_session_id: is mutually-exclusive with roles",
		},
		{
			name:   "scoped",
			scoped: true,
			in: func() *PIVAgentConfig {
				return &PIVAgentConfig{
					Destination: &destination.Directory{
						Path: "/opt/machine-id",
					},
				}
			},
			wantErr: "is not supported in scoped mode",
		},
	}

	testCheckAndSetDefaults(t, tests)
}
