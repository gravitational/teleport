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

package ssh

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/bot/testutils"
)

func TestSSHHostOutput_YAML(t *testing.T) {
	dest := &destination.Memory{}
	tests := []testutils.TestYAMLCase[HostOutputConfig]{
		{
			Name: "full",
			In: HostOutputConfig{
				Destination: dest,
				Roles:       []string{"access"},
				Principals:  []string{"host.example.com"},
				CredentialLifetime: bot.CredentialLifetime{
					TTL:             1 * time.Minute,
					RenewalInterval: 30 * time.Second,
				},
			},
		},
		{
			Name: "minimal",
			In: HostOutputConfig{
				Destination: dest,
				Principals:  []string{"host.example.com"},
			},
		},
	}
	testutils.TestYAML(t, tests)
}

func TestSSHHostOutput_CheckAndSetDefaults(t *testing.T) {
	tests := []testutils.TestCheckAndSetDefaultsCase[*HostOutputConfig]{
		{
			Name: "valid",
			In: func() *HostOutputConfig {
				return &HostOutputConfig{
					Destination: destination.NewMemory(),
					Roles:       []string{"access"},
					Principals:  []string{"host.example.com"},
				}
			},
		},
		{
			Name: "missing destination",
			In: func() *HostOutputConfig {
				return &HostOutputConfig{
					Destination: nil,
					Principals:  []string{"host.example.com"},
				}
			},
			WantErr: "no destination configured for output",
		},
		{
			Name: "missing principals",
			In: func() *HostOutputConfig {
				return &HostOutputConfig{
					Destination: destination.NewMemory(),
				}
			},
			WantErr: "at least one principal must be specified",
		},
	}
	testutils.TestCheckAndSetDefaults(t, tests)
}
