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

	"github.com/gravitational/teleport/lib/tbot/botfs"
)

func TestSSHProxyService_YAML(t *testing.T) {
	t.Parallel()

	tests := []testYAMLCase[SSHProxyService]{
		{
			name: "full",
			in: SSHProxyService{
				Destination: &DestinationDirectory{
					Path: "/opt/machine-id",
				},
			},
		},
	}
	testYAML(t, tests)
}

func TestSSHProxyService_CheckAndSetDefaults(t *testing.T) {
	t.Parallel()

	tests := []testCheckAndSetDefaultsCase[*SSHProxyService]{
		{
			name: "valid",
			in: func() *SSHProxyService {
				return &SSHProxyService{
					Destination: &DestinationDirectory{
						Path:     "/opt/machine-id",
						ACLs:     botfs.ACLOff,
						Symlinks: botfs.SymlinksInsecure,
					},
				}
			},
		},
		{
			name: "missing destination",
			in: func() *SSHProxyService {
				return &SSHProxyService{
					Destination: nil,
				}
			},
			wantErr: "destination: must be specified",
		},
		{
			name: "wrong destination type",
			in: func() *SSHProxyService {
				return &SSHProxyService{
					Destination: &DestinationMemory{},
				}
			},
			wantErr: "destination: must be of type `directory`",
		},
	}
	testCheckAndSetDefaults(t, tests)
}
