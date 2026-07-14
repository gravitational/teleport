// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package login

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/botfs"
)

func TestAgentConfig_YAML(t *testing.T) {
	t.Parallel()

	tests := []testYAMLCase[AgentConfig]{
		{
			name: "full",
			in: AgentConfig{
				Name: "login-agent",
				Destination: &destination.Directory{
					Path: "/opt/machine-id",
				},
				CredentialLifetime: bot.CredentialLifetime{
					TTL:             1 * time.Minute,
					RenewalInterval: 30 * time.Second,
				},
			},
		},
		{
			name: "minimal",
			in: AgentConfig{
				Destination: &destination.Directory{
					Path: "/opt/machine-id",
				},
			},
		},
	}

	testYAML(t, tests)
}

func TestAgentConfig_CheckAndSetDefaults(t *testing.T) {
	t.Parallel()

	tests := []testCheckAndSetDefaultsCase[*AgentConfig]{
		{
			name: "valid",
			in: func() *AgentConfig {
				return &AgentConfig{
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
			in: func() *AgentConfig {
				return &AgentConfig{}
			},
			wantErr: "destination: is required",
		},
		{
			name: "wrong destination type",
			in: func() *AgentConfig {
				return &AgentConfig{
					Destination: &destination.Memory{},
				}
			},
			wantErr: "destination: must be a filesystem directory",
		},
		{
			name:   "scoped",
			scoped: true,
			in: func() *AgentConfig {
				return &AgentConfig{
					Destination: &destination.Directory{
						Path: "/opt/machine-id",
					},
				}
			},
			wantErr: "is not supported in scoped mode",
		},
	}

	testCheckAndSetDefaults(t, tests)
}
