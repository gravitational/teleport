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
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tbot/config"
)

// TestLegacyCommand tests that the LegacyCommand properly parses its arguments
// and applies as expected onto a BotConfig.
func TestLegacyCommand(t *testing.T) {
	testStartConfigureCommand(t, NewLegacyCommand, []startConfigureTestCase{
		{
			name: "success",
			args: []string{
				"start", // Note: implied legacy, it should be the default.
				"--token=foo",
				"--ca-pin=bar",
				"--certificate-ttl=10m",
				"--renewal-interval=5m",
				"--join-method=github",
				"--oneshot",
				"--diag-addr=0.0.0.0:8080",
				"--data-dir=/foo",
				"--destination-dir=/bar",
				"--auth-server=example.com:3024",
			},
			assertConfig: func(t *testing.T, cfg *config.BotConfig) {
				token, err := cfg.Onboarding.Token()
				require.NoError(t, err)
				require.Equal(t, "foo", token)

				require.ElementsMatch(t, cfg.Onboarding.CAPins, []string{"bar"})
				require.Equal(t, time.Minute*10, cfg.CredentialLifetime.TTL)
				require.Equal(t, time.Minute*5, cfg.CredentialLifetime.RenewalInterval)
				require.Equal(t, types.JoinMethodGitHub, cfg.Onboarding.JoinMethod)
				require.True(t, cfg.Oneshot)
				require.Equal(t, "0.0.0.0:8080", cfg.DiagAddr)
				require.Equal(t, "example.com:3024", cfg.AuthServer)

				dir, ok := cfg.Storage.Destination.(*config.DestinationDirectory)
				require.True(t, ok)
				require.Equal(t, "/foo", dir.Path)

				require.Len(t, cfg.Services, 1)

				// It must configure an identity output with a directory destination.
				svc := cfg.Services[0]
				ident, ok := svc.(*config.IdentityOutput)
				require.True(t, ok)

				dir, ok = ident.Destination.(*config.DestinationDirectory)
				require.True(t, ok)
				require.Equal(t, "/bar", dir.Path)
			},
		},
	})
}
