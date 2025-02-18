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

func TestApplicationOutput_YAML(t *testing.T) {
	dest := &DestinationMemory{}
	tests := []testYAMLCase[ApplicationOutput]{
		{
			name: "full",
			in: ApplicationOutput{
				Destination: dest,
				Roles:       []string{"access"},
				AppName:     "my-app",
				CredentialLifetime: CredentialLifetime{
					TTL:             1 * time.Minute,
					RenewalInterval: 30 * time.Second,
				},
			},
		},
		{
			name: "minimal",
			in: ApplicationOutput{
				Destination: dest,
				AppName:     "my-app",
			},
		},
	}
	testYAML(t, tests)
}

func TestApplicationOutput_CheckAndSetDefaults(t *testing.T) {
	tests := []testCheckAndSetDefaultsCase[*ApplicationOutput]{
		{
			name: "valid",
			in: func() *ApplicationOutput {
				return &ApplicationOutput{
					Destination: memoryDestForTest(),
					Roles:       []string{"access"},
					AppName:     "app",
				}
			},
		},
		{
			name: "missing destination",
			in: func() *ApplicationOutput {
				return &ApplicationOutput{
					Destination: nil,
					AppName:     "app",
				}
			},
			wantErr: "no destination configured for output",
		},
		{
			name: "missing app_name",
			in: func() *ApplicationOutput {
				return &ApplicationOutput{
					Destination: memoryDestForTest(),
				}
			},
			wantErr: "app_name must not be empty",
		},
	}
	testCheckAndSetDefaults(t, tests)
}
