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

func TestDatabaseOutput_YAML(t *testing.T) {
	dest := &DestinationMemory{}
	tests := []testutils.TestYAMLCase[DatabaseOutput]{
		{
			Name: "full",
			In: DatabaseOutput{
				Destination: dest,
				Roles:       []string{"access"},
				Format:      TLSDatabaseFormat,
				Service:     "my-database-service",
				Database:    "my-database",
				Username:    "my-username",
				CredentialLifetime: CredentialLifetime{
					TTL:             1 * time.Minute,
					RenewalInterval: 30 * time.Second,
				},
			},
		},
		{
			Name: "minimal",
			In: DatabaseOutput{
				Destination: dest,
				Service:     "my-database-service",
			},
		},
	}
	testutils.TestYAML(t, tests)
}

func TestDatabaseOutput_CheckAndSetDefaults(t *testing.T) {
	tests := []testutils.TestCheckAndSetDefaultsCase[*DatabaseOutput]{
		{
			Name: "valid",
			In: func() *DatabaseOutput {
				return &DatabaseOutput{
					Destination: memoryDestForTest(),
					Roles:       []string{"access"},
					Database:    "db",
					Service:     "service",
					Username:    "username",
				}
			},
		},
		{
			Name: "missing destination",
			In: func() *DatabaseOutput {
				return &DatabaseOutput{
					Destination: nil,
					Service:     "service",
				}
			},
			WantErr: "no destination configured for output",
		},
		{
			Name: "missing service",
			In: func() *DatabaseOutput {
				return &DatabaseOutput{
					Destination: memoryDestForTest(),
				}
			},
			WantErr: "service must not be empty",
		},
		{
			Name: "invalid format",
			In: func() *DatabaseOutput {
				return &DatabaseOutput{
					Destination: memoryDestForTest(),
					Service:     "service",
					Format:      "no-such-format",
				}
			},
			WantErr: "unrecognized format (no-such-format)",
		},
	}
	testutils.TestCheckAndSetDefaults(t, tests)
}
