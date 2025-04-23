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
)

func TestDatabaseOutput_YAML(t *testing.T) {
	dest := &DestinationMemory{}
	tests := []testYAMLCase[DatabaseOutput]{
		{
			name: "full",
			in: DatabaseOutput{
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
			name: "minimal",
			in: DatabaseOutput{
				Destination: dest,
				Service:     "my-database-service",
			},
		},
	}
	testYAML(t, tests)
}

func TestDatabaseOutput_CheckAndSetDefaults(t *testing.T) {
	tests := []testCheckAndSetDefaultsCase[*DatabaseOutput]{
		{
			name: "valid",
			in: func() *DatabaseOutput {
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
			name: "missing destination",
			in: func() *DatabaseOutput {
				return &DatabaseOutput{
					Destination: nil,
					Service:     "service",
				}
			},
			wantErr: "no destination configured for output",
		},
		{
			name: "missing service",
			in: func() *DatabaseOutput {
				return &DatabaseOutput{
					Destination: memoryDestForTest(),
				}
			},
			wantErr: "service must not be empty",
		},
		{
			name: "invalid format",
			in: func() *DatabaseOutput {
				return &DatabaseOutput{
					Destination: memoryDestForTest(),
					Service:     "service",
					Format:      "no-such-format",
				}
			},
			wantErr: "unrecognized format (no-such-format)",
		},
	}
	testCheckAndSetDefaults(t, tests)
}
