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

import (
	"testing"
	"time"
)

func TestApplicationTunnelService_YAML(t *testing.T) {
	t.Parallel()

	tests := []testYAMLCase[ApplicationTunnelService]{
		{
			name: "full",
			in: ApplicationTunnelService{
				Listen:  "tcp://0.0.0.0:3621",
				AppName: "my-app",
				CredentialLifetime: CredentialLifetime{
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

	tests := []testCheckAndSetDefaultsCase[*ApplicationTunnelService]{
		{
			name: "valid",
			in: func() *ApplicationTunnelService {
				return &ApplicationTunnelService{
					Listen:  "tcp://0.0.0.0:3621",
					Roles:   []string{"role1", "role2"},
					AppName: "my-app",
				}
			},
			wantErr: "",
		},
		{
			name: "missing listen",
			in: func() *ApplicationTunnelService {
				return &ApplicationTunnelService{
					Roles:   []string{"role1", "role2"},
					AppName: "my-app",
				}
			},
			wantErr: "listen: should not be empty",
		},
		{
			name: "listen not url",
			in: func() *ApplicationTunnelService {
				return &ApplicationTunnelService{
					Listen:  "\x00",
					Roles:   []string{"role1", "role2"},
					AppName: "my-app",
				}
			},
			wantErr: "parsing listen",
		},
		{
			name: "missing app name",
			in: func() *ApplicationTunnelService {
				return &ApplicationTunnelService{
					Listen: "tcp://0.0.0.0:3621",
					Roles:  []string{"role1", "role2"},
				}
			},
			wantErr: "app_name: should not be empty",
		},
	}
	testCheckAndSetDefaults(t, tests)
}
