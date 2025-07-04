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
	"github.com/gravitational/teleport/lib/tbot/bot/testutils"
	"github.com/gravitational/teleport/lib/tbot/services/identity"
)

func TestIdentityOutput_YAML(t *testing.T) {
	dest := &destination.Memory{}
	tests := []testutils.TestYAMLCase[identity.OutputConfig]{
		{
			Name: "full",
			In: identity.OutputConfig{
				Destination:   dest,
				Roles:         []string{"access"},
				Cluster:       "leaf.example.com",
				SSHConfigMode: identity.SSHConfigModeOff,
				AllowReissue:  true,
				CredentialLifetime: bot.CredentialLifetime{
					TTL:             1 * time.Minute,
					RenewalInterval: 30 * time.Second,
				},
			},
		},
		{
			Name: "minimal",
			In: identity.OutputConfig{
				Destination: dest,
			},
		},
	}
	testutils.TestYAML(t, tests)
}

func TestIdentityOutput_CheckAndSetDefaults(t *testing.T) {
	tests := []testutils.TestCheckAndSetDefaultsCase[*identity.OutputConfig]{
		{
			Name: "valid",
			In: func() *identity.OutputConfig {
				return &identity.OutputConfig{
					Destination:   destination.NewMemory(),
					Roles:         []string{"access"},
					SSHConfigMode: identity.SSHConfigModeOn,
				}
			},
		},
		{
			Name: "ssh config mode defaults",
			In: func() *identity.OutputConfig {
				return &identity.OutputConfig{
					Destination: destination.NewMemory(),
				}
			},
			Want: &identity.OutputConfig{
				Destination:   destination.NewMemory(),
				SSHConfigMode: identity.SSHConfigModeOn,
			},
		},
		{
			Name: "missing destination",
			In: func() *identity.OutputConfig {
				return &identity.OutputConfig{
					Destination: nil,
				}
			},
			WantErr: "no destination configured for output",
		},
		{
			Name: "invalid ssh config mode",
			In: func() *identity.OutputConfig {
				return &identity.OutputConfig{
					Destination:   destination.NewMemory(),
					SSHConfigMode: "invalid",
				}
			},
			WantErr: "ssh_config: unrecognized value \"invalid\"",
		},
	}
	testutils.TestCheckAndSetDefaults(t, tests)
}
