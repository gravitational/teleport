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

	"github.com/gravitational/teleport/lib/tbot/botfs"
	"github.com/gravitational/teleport/lib/tbot/internal/testutils"
)

func TestSSHMultiplexerService_YAML(t *testing.T) {
	t.Parallel()

	tests := []testutils.TestYAMLCase[SSHMultiplexerService]{
		{
			Name: "full",
			In: SSHMultiplexerService{
				Destination: &DestinationDirectory{
					Path: "/opt/machine-id",
				},
				EnableResumption:   ptr[bool](true),
				ProxyTemplatesPath: "/etc/teleport/templates",
				ProxyCommand:       []string{"rusty-boi"},
				CredentialLifetime: CredentialLifetime{
					TTL:             1 * time.Minute,
					RenewalInterval: 30 * time.Second,
				},
			},
		},
	}
	testutils.TestYAML(t, tests)
}

func TestSSHMultiplexerService_CheckAndSetDefaults(t *testing.T) {
	t.Parallel()

	tests := []testutils.TestCheckAndSetDefaultsCase[*SSHMultiplexerService]{
		{
			Name: "valid",
			In: func() *SSHMultiplexerService {
				return &SSHMultiplexerService{
					Destination: &DestinationDirectory{
						Path:     "/opt/machine-id",
						ACLs:     botfs.ACLOff,
						Symlinks: botfs.SymlinksInsecure,
					},
				}
			},
		},
		{
			Name: "missing destination",
			In: func() *SSHMultiplexerService {
				return &SSHMultiplexerService{
					Destination: nil,
				}
			},
			WantErr: "destination: must be specified",
		},
		{
			Name: "wrong destination type",
			In: func() *SSHMultiplexerService {
				return &SSHMultiplexerService{
					Destination: &DestinationMemory{},
				}
			},
			WantErr: "destination: must be of type `directory`",
		},
	}
	testutils.TestCheckAndSetDefaults(t, tests)
}
