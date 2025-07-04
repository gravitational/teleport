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
	"github.com/gravitational/teleport/lib/tbot/services/application"
)

func TestApplicationOutput_YAML(t *testing.T) {
	dest := &destination.Memory{}
	tests := []testYAMLCase[application.OutputConfig]{
		{
			name: "full",
			in: application.OutputConfig{
				Destination: dest,
				Roles:       []string{"access"},
				AppName:     "my-app",
				CredentialLifetime: bot.CredentialLifetime{
					TTL:             1 * time.Minute,
					RenewalInterval: 30 * time.Second,
				},
			},
		},
		{
			name: "minimal",
			in: application.OutputConfig{
				Destination: dest,
				AppName:     "my-app",
			},
		},
	}
	testYAML(t, tests)
}

func TestApplicationOutput_CheckAndSetDefaults(t *testing.T) {
	tests := []testCheckAndSetDefaultsCase[*application.OutputConfig]{
		{
			name: "valid",
			in: func() *application.OutputConfig {
				return &application.OutputConfig{
					Destination: destination.NewMemory(),
					Roles:       []string{"access"},
					AppName:     "app",
				}
			},
		},
		{
			name: "missing destination",
			in: func() *application.OutputConfig {
				return &application.OutputConfig{
					Destination: nil,
					AppName:     "app",
				}
			},
			wantErr: "no destination configured for output",
		},
		{
			name: "missing app_name",
			in: func() *application.OutputConfig {
				return &application.OutputConfig{
					Destination: destination.NewMemory(),
				}
			},
			wantErr: "app_name must not be empty",
		},
	}
	testCheckAndSetDefaults(t, tests)
}
