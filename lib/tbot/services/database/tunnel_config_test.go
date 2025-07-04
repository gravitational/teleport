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

package database

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/testutils"
)

func TestDatabaseTunnelService_YAML(t *testing.T) {
	t.Parallel()

	tests := []testutils.TestYAMLCase[TunnelConfig]{
		{
			Name: "full",
			In: TunnelConfig{
				Listen:   "tcp://0.0.0.0:3621",
				Roles:    []string{"role1", "role2"},
				Service:  "service",
				Database: "database",
				Username: "username",
				CredentialLifetime: bot.CredentialLifetime{
					TTL:             1 * time.Minute,
					RenewalInterval: 30 * time.Second,
				},
			},
		},
	}
	testutils.TestYAML(t, tests)
}

func TestDatabaseTunnelService_CheckAndSetDefaults(t *testing.T) {
	t.Parallel()

	tests := []testutils.TestCheckAndSetDefaultsCase[*TunnelConfig]{
		{
			Name: "valid",
			In: func() *TunnelConfig {
				return &TunnelConfig{
					Listen:   "tcp://0.0.0.0:3621",
					Roles:    []string{"role1", "role2"},
					Service:  "service",
					Database: "database",
					Username: "username",
				}
			},
			WantErr: "",
		},
		{
			Name: "missing listen",
			In: func() *TunnelConfig {
				return &TunnelConfig{
					Roles:    []string{"role1", "role2"},
					Service:  "service",
					Database: "database",
					Username: "username",
				}
			},
			WantErr: "listen: should not be empty",
		},
		{
			Name: "missing service",
			In: func() *TunnelConfig {
				return &TunnelConfig{
					Listen:   "tcp://0.0.0.0:3621",
					Roles:    []string{"role1", "role2"},
					Database: "database",
					Username: "username",
				}
			},
			WantErr: "service: should not be empty",
		},
		{
			Name: "missing database",
			In: func() *TunnelConfig {
				return &TunnelConfig{
					Listen:   "tcp://0.0.0.0:3621",
					Roles:    []string{"role1", "role2"},
					Service:  "service",
					Username: "username",
				}
			},
			WantErr: "database: should not be empty",
		},
		{
			Name: "missing username",
			In: func() *TunnelConfig {
				return &TunnelConfig{
					Listen:   "tcp://0.0.0.0:3621",
					Roles:    []string{"role1", "role2"},
					Service:  "service",
					Database: "database",
				}
			},
			WantErr: "username: should not be empty",
		},
	}
	testutils.TestCheckAndSetDefaults(t, tests)
}
