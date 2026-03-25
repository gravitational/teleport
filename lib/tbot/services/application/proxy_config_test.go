/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package application

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/tbot/bot"
)

func TestProxyServiceConfig_YAML(t *testing.T) {
	t.Parallel()

	tests := []testYAMLCase[ProxyServiceConfig]{
		{
			name: "full",
			in: ProxyServiceConfig{
				Name:   "foo",
				Listen: "tcp://0.0.0.0:3621",
				CredentialLifetime: bot.CredentialLifetime{
					TTL:             1 * time.Minute,
					RenewalInterval: 30 * time.Second,
				},
			},
		},
	}
	testYAML(t, tests)
}

func TestProxyServiceConfig_CheckAndSetDefaults(t *testing.T) {
	t.Parallel()

	tests := []testCheckAndSetDefaultsCase[*ProxyServiceConfig]{
		{
			name: "valid",
			in: func() *ProxyServiceConfig {
				return &ProxyServiceConfig{
					Listen: "tcp://0.0.0.0:3621",
				}
			},
			wantErr: "",
		},
		{
			name: "missing listen",
			in: func() *ProxyServiceConfig {
				return &ProxyServiceConfig{}
			},
			wantErr: "listen: should not be empty",
		},
		{
			name: "listen not url",
			in: func() *ProxyServiceConfig {
				return &ProxyServiceConfig{
					Listen: "\x00",
				}
			},
			wantErr: "parsing listen",
		},
	}
	testCheckAndSetDefaults(t, tests)
}
