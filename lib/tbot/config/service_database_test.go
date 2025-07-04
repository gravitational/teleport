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
	"github.com/gravitational/teleport/lib/tbot/services/database"
)

func TestDatabaseOutput_YAML(t *testing.T) {
	dest := &destination.Memory{}
	tests := []testutils.TestYAMLCase[database.OutputConfig]{
		{
			Name: "full",
			In: database.OutputConfig{
				Destination: dest,
				Roles:       []string{"access"},
				Format:      database.TLSDatabaseFormat,
				Service:     "my-database-service",
				Database:    "my-database",
				Username:    "my-username",
				CredentialLifetime: bot.CredentialLifetime{
					TTL:             1 * time.Minute,
					RenewalInterval: 30 * time.Second,
				},
			},
		},
		{
			Name: "minimal",
			In: database.OutputConfig{
				Destination: dest,
				Service:     "my-database-service",
			},
		},
	}
	testutils.TestYAML(t, tests)
}

func TestDatabaseOutput_CheckAndSetDefaults(t *testing.T) {
	tests := []testutils.TestCheckAndSetDefaultsCase[*database.OutputConfig]{
		{
			Name: "valid",
			In: func() *database.OutputConfig {
				return &database.OutputConfig{
					Destination: destination.NewMemory(),
					Roles:       []string{"access"},
					Database:    "db",
					Service:     "service",
					Username:    "username",
				}
			},
		},
		{
			Name: "missing destination",
			In: func() *database.OutputConfig {
				return &database.OutputConfig{
					Destination: nil,
					Service:     "service",
				}
			},
			WantErr: "no destination configured for output",
		},
		{
			Name: "missing service",
			In: func() *database.OutputConfig {
				return &database.OutputConfig{
					Destination: destination.NewMemory(),
				}
			},
			WantErr: "service must not be empty",
		},
		{
			Name: "invalid format",
			In: func() *database.OutputConfig {
				return &database.OutputConfig{
					Destination: destination.NewMemory(),
					Service:     "service",
					Format:      "no-such-format",
				}
			},
			WantErr: "unrecognized format (no-such-format)",
		},
	}
	testutils.TestCheckAndSetDefaults(t, tests)
}
