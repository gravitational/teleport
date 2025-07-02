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
	"github.com/gravitational/teleport/lib/tbot/botfs"
	"github.com/gravitational/teleport/lib/tbot/services/ssh"
)

func TestSSHMultiplexerService_YAML(t *testing.T) {
	t.Parallel()

	tests := []testYAMLCase[ssh.MultiplexerConfig]{
		{
			name: "full",
			in: ssh.MultiplexerConfig{
				Destination: &destination.Directory{
					Path: "/opt/machine-id",
				},
				EnableResumption:   ptr[bool](true),
				ProxyTemplatesPath: "/etc/teleport/templates",
				ProxyCommand:       []string{"rusty-boi"},
				CredentialLifetime: bot.CredentialLifetime{
					TTL:             1 * time.Minute,
					RenewalInterval: 30 * time.Second,
				},
			},
		},
	}
	testYAML(t, tests)
}

func TestSSHMultiplexerService_CheckAndSetDefaults(t *testing.T) {
	t.Parallel()

	tests := []testCheckAndSetDefaultsCase[*ssh.MultiplexerConfig]{
		{
			name: "valid",
			in: func() *ssh.MultiplexerConfig {
				return &ssh.MultiplexerConfig{
					Destination: &destination.Directory{
						Path:     "/opt/machine-id",
						ACLs:     botfs.ACLOff,
						Symlinks: botfs.SymlinksInsecure,
					},
				}
			},
		},
		{
			name: "missing destination",
			in: func() *ssh.MultiplexerConfig {
				return &ssh.MultiplexerConfig{
					Destination: nil,
				}
			},
			wantErr: "destination: must be specified",
		},
		{
			name: "wrong destination type",
			in: func() *ssh.MultiplexerConfig {
				return &ssh.MultiplexerConfig{
					Destination: &destination.Memory{},
				}
			},
			wantErr: "destination: must be of type `directory`",
		},
	}
	testCheckAndSetDefaults(t, tests)
}
