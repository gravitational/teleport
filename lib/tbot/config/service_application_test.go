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
	"github.com/gravitational/teleport/lib/tbot/services/application"
)

func TestApplicationOutput_YAML(t *testing.T) {
	dest := &destination.Memory{}
	tests := []testutils.TestYAMLCase[application.OutputConfig]{
		{
			Name: "full",
			In: application.OutputConfig{
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
			Name: "minimal",
			In: application.OutputConfig{
				Destination: dest,
				AppName:     "my-app",
			},
		},
	}
	testutils.TestYAML(t, tests)
}

func TestApplicationOutput_CheckAndSetDefaults(t *testing.T) {
	tests := []testutils.TestCheckAndSetDefaultsCase[*application.OutputConfig]{
		{
			Name: "valid",
			In: func() *application.OutputConfig {
				return &application.OutputConfig{
					Destination: destination.NewMemory(),
					Roles:       []string{"access"},
					AppName:     "app",
				}
			},
		},
		{
			Name: "missing destination",
			In: func() *application.OutputConfig {
				return &application.OutputConfig{
					Destination: nil,
					AppName:     "app",
				}
			},
			WantErr: "no destination configured for output",
		},
		{
			Name: "missing app_name",
			In: func() *application.OutputConfig {
				return &application.OutputConfig{
					Destination: destination.NewMemory(),
				}
			},
			WantErr: "app_name must not be empty",
		},
	}
	testutils.TestCheckAndSetDefaults(t, tests)
}
