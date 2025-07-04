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
	"github.com/gravitational/teleport/lib/tbot/services/database"
)

func TestDatabaseOutput_YAML(t *testing.T) {
	dest := &destination.Memory{}
	tests := []testYAMLCase[database.OutputConfig]{
		{
			name: "full",
			in: database.OutputConfig{
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
			name: "minimal",
			in: database.OutputConfig{
				Destination: dest,
				Service:     "my-database-service",
			},
		},
	}
	testYAML(t, tests)
}

func TestDatabaseOutput_CheckAndSetDefaults(t *testing.T) {
	tests := []testCheckAndSetDefaultsCase[*database.OutputConfig]{
		{
			name: "valid",
			in: func() *database.OutputConfig {
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
			name: "missing destination",
			in: func() *database.OutputConfig {
				return &database.OutputConfig{
					Destination: nil,
					Service:     "service",
				}
			},
			wantErr: "no destination configured for output",
		},
		{
			name: "missing service",
			in: func() *database.OutputConfig {
				return &database.OutputConfig{
					Destination: destination.NewMemory(),
				}
			},
			wantErr: "service must not be empty",
		},
		{
			name: "invalid format",
			in: func() *database.OutputConfig {
				return &database.OutputConfig{
					Destination: destination.NewMemory(),
					Service:     "service",
					Format:      "no-such-format",
				}
			},
			wantErr: "unrecognized format (no-such-format)",
		},
	}
	testCheckAndSetDefaults(t, tests)
}
