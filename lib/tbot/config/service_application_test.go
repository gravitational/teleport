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

func TestApplicationOutput_YAML(t *testing.T) {
	dest := &DestinationMemory{}
	tests := []testutils.TestYAMLCase[ApplicationOutput]{
		{
			Name: "full",
			In: ApplicationOutput{
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
			Name: "minimal",
			In: ApplicationOutput{
				Destination: dest,
				AppName:     "my-app",
			},
		},
	}
	testutils.TestYAML(t, tests)
}

func TestApplicationOutput_CheckAndSetDefaults(t *testing.T) {
	tests := []testutils.TestCheckAndSetDefaultsCase[*ApplicationOutput]{
		{
			Name: "valid",
			In: func() *ApplicationOutput {
				return &ApplicationOutput{
					Destination: memoryDestForTest(),
					Roles:       []string{"access"},
					AppName:     "app",
				}
			},
		},
		{
			Name: "missing destination",
			In: func() *ApplicationOutput {
				return &ApplicationOutput{
					Destination: nil,
					AppName:     "app",
				}
			},
			WantErr: "no destination configured for output",
		},
		{
			Name: "missing app_name",
			In: func() *ApplicationOutput {
				return &ApplicationOutput{
					Destination: memoryDestForTest(),
				}
			},
			WantErr: "app_name must not be empty",
		},
	}
	testutils.TestCheckAndSetDefaults(t, tests)
}
