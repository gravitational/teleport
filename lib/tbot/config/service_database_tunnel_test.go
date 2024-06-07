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

package config

import "testing"

func TestDatabaseTunnelService_YAML(t *testing.T) {
	t.Parallel()

	tests := []testYAMLCase[DatabaseTunnelService]{
		{
			name: "full",
			in: DatabaseTunnelService{
				Listen:   "tcp://0.0.0.0:3621",
				Roles:    []string{"role1", "role2"},
				Service:  "service",
				Database: "database",
				Username: "username",
			},
		},
	}
	testYAML(t, tests)
}

func TestDatabaseTunnelService_CheckAndSetDefaults(t *testing.T) {
	t.Parallel()

	tests := []testCheckAndSetDefaultsCase[*DatabaseTunnelService]{
		{
			name: "valid",
			in: func() *DatabaseTunnelService {
				return &DatabaseTunnelService{
					Listen:   "tcp://0.0.0.0:3621",
					Roles:    []string{"role1", "role2"},
					Service:  "service",
					Database: "database",
					Username: "username",
				}
			},
			wantErr: "",
		},
		{
			name: "missing listen",
			in: func() *DatabaseTunnelService {
				return &DatabaseTunnelService{
					Roles:    []string{"role1", "role2"},
					Service:  "service",
					Database: "database",
					Username: "username",
				}
			},
			wantErr: "listen: should not be empty",
		},
		{
			name: "missing service",
			in: func() *DatabaseTunnelService {
				return &DatabaseTunnelService{
					Listen:   "tcp://0.0.0.0:3621",
					Roles:    []string{"role1", "role2"},
					Database: "database",
					Username: "username",
				}
			},
			wantErr: "service: should not be empty",
		},
		{
			name: "missing database",
			in: func() *DatabaseTunnelService {
				return &DatabaseTunnelService{
					Listen:   "tcp://0.0.0.0:3621",
					Roles:    []string{"role1", "role2"},
					Service:  "service",
					Username: "username",
				}
			},
			wantErr: "database: should not be empty",
		},
		{
			name: "missing username",
			in: func() *DatabaseTunnelService {
				return &DatabaseTunnelService{
					Listen:   "tcp://0.0.0.0:3621",
					Roles:    []string{"role1", "role2"},
					Service:  "service",
					Database: "database",
				}
			},
			wantErr: "username: should not be empty",
		},
	}
	testCheckAndSetDefaults(t, tests)
}
