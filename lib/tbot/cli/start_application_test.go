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

package cli

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tbot/config"
)

// TestApplicationCommand tests that the ApplicationCommand properly parses its
// arguments and applies as expected onto a BotConfig.
func TestApplicationCommand(t *testing.T) {
	testStartConfigureCommand(t, NewApplicationCommand, []startConfigureTestCase{
		{
			name: "success",
			args: []string{
				"start",
				"application",
				"--destination=/bar",
				"--token=foo",
				"--join-method=github",
				"--proxy-server=example.com:443",
				"--app=foo",
				"--specific-tls-extensions",
			},
			assertConfig: func(t *testing.T, cfg *config.BotConfig) {
				// It must configure a app output with a directory destination.
				svc := cfg.Services[0]
				appSvc, ok := svc.(*config.ApplicationOutput)
				require.True(t, ok)

				require.Equal(t, "foo", appSvc.AppName)
				require.True(t, appSvc.SpecificTLSExtensions)

				dir, ok := appSvc.Destination.(*config.DestinationDirectory)
				require.True(t, ok)
				require.Equal(t, "/bar", dir.Path)
			},
		},
	})
}
