/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"github.com/gravitational/teleport/lib/tbot/bot/testutils"
	"github.com/gravitational/teleport/lib/tbot/services/application"
)

func TestApplicationTunnelService_YAML(t *testing.T) {
	t.Parallel()

	tests := []testutils.TestYAMLCase[application.TunnelConfig]{
		{
			Name: "full",
			In: application.TunnelConfig{
				Listen:  "tcp://0.0.0.0:3621",
				AppName: "my-app",
				CredentialLifetime: bot.CredentialLifetime{
					TTL:             1 * time.Minute,
					RenewalInterval: 30 * time.Second,
				},
			},
		},
	}
	testutils.TestYAML(t, tests)
}

func TestApplicationTunnelService_CheckAndSetDefaults(t *testing.T) {
	t.Parallel()

	tests := []testutils.TestCheckAndSetDefaultsCase[*application.TunnelConfig]{
		{
			Name: "valid",
			In: func() *application.TunnelConfig {
				return &application.TunnelConfig{
					Listen:  "tcp://0.0.0.0:3621",
					Roles:   []string{"role1", "role2"},
					AppName: "my-app",
				}
			},
			WantErr: "",
		},
		{
			Name: "missing listen",
			In: func() *application.TunnelConfig {
				return &application.TunnelConfig{
					Roles:   []string{"role1", "role2"},
					AppName: "my-app",
				}
			},
			WantErr: "listen: should not be empty",
		},
		{
			Name: "listen not url",
			In: func() *application.TunnelConfig {
				return &application.TunnelConfig{
					Listen:  "\x00",
					Roles:   []string{"role1", "role2"},
					AppName: "my-app",
				}
			},
			WantErr: "parsing listen",
		},
		{
			Name: "missing app name",
			In: func() *application.TunnelConfig {
				return &application.TunnelConfig{
					Listen: "tcp://0.0.0.0:3621",
					Roles:  []string{"role1", "role2"},
				}
			},
			WantErr: "app_name: should not be empty",
		},
	}
	testutils.TestCheckAndSetDefaults(t, tests)
}
