/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
)

func TestIdentityOutput_YAML(t *testing.T) {
	dest := &destination.Memory{}
	tests := []testYAMLCase[IdentityOutput]{
		{
			name: "full",
			in: IdentityOutput{
				Destination:   dest,
				Roles:         []string{"access"},
				Cluster:       "leaf.example.com",
				SSHConfigMode: SSHConfigModeOff,
				AllowReissue:  true,
				CredentialLifetime: bot.CredentialLifetime{
					TTL:             1 * time.Minute,
					RenewalInterval: 30 * time.Second,
				},
			},
		},
		{
			name: "minimal",
			in: IdentityOutput{
				Destination: dest,
			},
		},
	}
	testYAML(t, tests)
}

func TestIdentityOutput_CheckAndSetDefaults(t *testing.T) {
	tests := []testCheckAndSetDefaultsCase[*IdentityOutput]{
		{
			name: "valid",
			in: func() *IdentityOutput {
				return &IdentityOutput{
					Destination:   destination.NewMemory(),
					Roles:         []string{"access"},
					SSHConfigMode: SSHConfigModeOn,
				}
			},
		},
		{
			name: "ssh config mode defaults",
			in: func() *IdentityOutput {
				return &IdentityOutput{
					Destination: destination.NewMemory(),
				}
			},
			want: &IdentityOutput{
				Destination:   destination.NewMemory(),
				SSHConfigMode: SSHConfigModeOn,
			},
		},
		{
			name: "missing destination",
			in: func() *IdentityOutput {
				return &IdentityOutput{
					Destination: nil,
				}
			},
			wantErr: "no destination configured for output",
		},
		{
			name: "invalid ssh config mode",
			in: func() *IdentityOutput {
				return &IdentityOutput{
					Destination:   destination.NewMemory(),
					SSHConfigMode: "invalid",
				}
			},
			wantErr: "ssh_config: unrecognized value \"invalid\"",
		},
	}
	testCheckAndSetDefaults(t, tests)
}
