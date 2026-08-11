/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package application

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/lib/tbot/bot"
)

func TestApplicationTunnelService_YAML(t *testing.T) {
	t.Parallel()

	tests := []testYAMLCase[TunnelConfig]{
		{
			name: "full",
			in: TunnelConfig{
				Listen:              "tcp://0.0.0.0:3621",
				AppName:             "my-app",
				DelegationSessionID: "8a50ba48-2fad-4c2c-a8ce-f48bc18db9ee",
				CredentialLifetime: bot.CredentialLifetime{
					TTL:             1 * time.Minute,
					RenewalInterval: 30 * time.Second,
				},
			},
		},
	}
	testYAML(t, tests)
}

func TestApplicationTunnelService_CheckAndSetDefaults(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()

	tests := []testCheckAndSetDefaultsCase[*TunnelConfig]{
		{
			name: "valid",
			in: func() *TunnelConfig {
				return &TunnelConfig{
					Listen:  "tcp://0.0.0.0:3621",
					AppName: "my-app",
					clock:   clock,
				}
			},
			want: &TunnelConfig{
				Listen:  "tcp://0.0.0.0:3621",
				AppName: "my-app",
				clock:   clock,
			},
			wantErr: "",
		},
		{
			name: "missing listen",
			in: func() *TunnelConfig {
				return &TunnelConfig{
					AppName: "my-app",
				}
			},
			wantErr: "listen: should not be empty",
		},
		{
			name: "listen not url",
			in: func() *TunnelConfig {
				return &TunnelConfig{
					Listen:  "\x00",
					AppName: "my-app",
				}
			},
			wantErr: "parsing listen",
		},
		{
			name: "missing app name",
			in: func() *TunnelConfig {
				return &TunnelConfig{
					Listen: "tcp://0.0.0.0:3621",
				}
			},
			wantErr: "app_name: should not be empty",
		},
		{
			name: "roles is no longer supported",
			in: func() *TunnelConfig {
				return &TunnelConfig{
					Listen:          "tcp://0.0.0.0:3621",
					AppName:         "my-app",
					DeprecatedRoles: []string{"role1", "role2"},
				}
			},
			wantErr: "roles: the roles field is no longer supported",
		},
		{
			name:   "not scoped with SQN",
			scoped: false,
			in: func() *TunnelConfig {
				return &TunnelConfig{
					Listen:  "tcp://0.0.0.0:3621",
					AppName: "/staging::my-app",
					clock:   clock,
				}
			},
			wantErr: "app_name: can not be a scope-qualified name when not in scope mode",
		},
		{
			name:   "scoped",
			scoped: true,
			in: func() *TunnelConfig {
				return &TunnelConfig{
					Listen:  "tcp://0.0.0.0:3621",
					AppName: "/staging::my-app",
					clock:   clock,
				}
			},
		},
		{
			name:   "scoped with delegation_session_id set",
			scoped: true,
			in: func() *TunnelConfig {
				return &TunnelConfig{
					Listen:              "tcp://0.0.0.0:3621",
					AppName:             "/staging::my-app",
					DelegationSessionID: "8a50ba48-2fad-4c2c-a8ce-f48bc18db9ee",
				}
			},
			wantErr: "delegation_session_id: not supported with scopes",
		},
		{
			name:   "scoped without SQN",
			scoped: true,
			in: func() *TunnelConfig {
				return &TunnelConfig{
					Listen:  "tcp://0.0.0.0:3621",
					AppName: "my-app",
				}
			},
			wantErr: "app_name: needs to be a scope-qualified name when in scope mode",
		},
	}
	testCheckAndSetDefaults(t, tests)
}
