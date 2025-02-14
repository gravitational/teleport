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
)

func TestSSHHostOutput_YAML(t *testing.T) {
	dest := &DestinationMemory{}
	tests := []testYAMLCase[SSHHostOutput]{
		{
			name: "full",
			in: SSHHostOutput{
				Destination: dest,
				Roles:       []string{"access"},
				Principals:  []string{"host.example.com"},
				CredentialLifetime: CredentialLifetime{
					TTL:             1 * time.Minute,
					RenewalInterval: 30 * time.Second,
				},
			},
		},
		{
			name: "minimal",
			in: SSHHostOutput{
				Destination: dest,
				Principals:  []string{"host.example.com"},
			},
		},
	}
	testYAML(t, tests)
}

func TestSSHHostOutput_CheckAndSetDefaults(t *testing.T) {
	tests := []testCheckAndSetDefaultsCase[*SSHHostOutput]{
		{
			name: "valid",
			in: func() *SSHHostOutput {
				return &SSHHostOutput{
					Destination: memoryDestForTest(),
					Roles:       []string{"access"},
					Principals:  []string{"host.example.com"},
				}
			},
		},
		{
			name: "missing destination",
			in: func() *SSHHostOutput {
				return &SSHHostOutput{
					Destination: nil,
					Principals:  []string{"host.example.com"},
				}
			},
			wantErr: "no destination configured for output",
		},
		{
			name: "missing principals",
			in: func() *SSHHostOutput {
				return &SSHHostOutput{
					Destination: memoryDestForTest(),
				}
			},
			wantErr: "at least one principal must be specified",
		},
	}
	testCheckAndSetDefaults(t, tests)
}
