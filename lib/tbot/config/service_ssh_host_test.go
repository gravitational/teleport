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

	"github.com/gravitational/teleport/lib/tbot/internal/testutils"
)

func TestSSHHostOutput_YAML(t *testing.T) {
	dest := &DestinationMemory{}
	tests := []testutils.TestYAMLCase[SSHHostOutput]{
		{
			Name: "full",
			In: SSHHostOutput{
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
			Name: "minimal",
			In: SSHHostOutput{
				Destination: dest,
				Principals:  []string{"host.example.com"},
			},
		},
	}
	testutils.TestYAML(t, tests)
}

func TestSSHHostOutput_CheckAndSetDefaults(t *testing.T) {
	tests := []testutils.TestCheckAndSetDefaultsCase[*SSHHostOutput]{
		{
			Name: "valid",
			In: func() *SSHHostOutput {
				return &SSHHostOutput{
					Destination: memoryDestForTest(),
					Roles:       []string{"access"},
					Principals:  []string{"host.example.com"},
				}
			},
		},
		{
			Name: "missing destination",
			In: func() *SSHHostOutput {
				return &SSHHostOutput{
					Destination: nil,
					Principals:  []string{"host.example.com"},
				}
			},
			WantErr: "no destination configured for output",
		},
		{
			Name: "missing principals",
			In: func() *SSHHostOutput {
				return &SSHHostOutput{
					Destination: memoryDestForTest(),
				}
			},
			WantErr: "at least one principal must be specified",
		},
	}
	testutils.TestCheckAndSetDefaults(t, tests)
}
