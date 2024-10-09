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

// TestSPIFFESVIDCommand tests that the SPIFFESVIDCommand properly parses its
// arguments and applies as expected onto a BotConfig.
func TestSPIFFESVIDCommand(t *testing.T) {
	testStartConfigureCommand(t, NewSPIFFESVIDCommand, []startConfigureTestCase{
		{
			name: "success",
			args: []string{
				"start",
				"spiffe-svid",
				"--destination=/bar",
				"--token=foo",
				"--join-method=github",
				"--proxy-server=example.com:443",
				"--include-federated-trust-bundles",
				"--svid-path=/foo/bar",
				"--svid-hint=hello world",
				"--dns-san=foo.example.com",
				"--dns-san=bar.example.com",
				"--ip-san=192.168.1.1",
				"--ip-san=192.168.1.2",
			},
			assertConfig: func(t *testing.T, cfg *config.BotConfig) {
				require.Len(t, cfg.Services, 1)

				// It must configure a SPIFFE output with a directory destination.
				svc := cfg.Services[0]
				spiffe, ok := svc.(*config.SPIFFESVIDOutput)
				require.True(t, ok)

				require.True(t, spiffe.IncludeFederatedTrustBundles)

				svid := spiffe.SVID
				require.Equal(t, "/foo/bar", svid.Path)
				require.Equal(t, "hello world", svid.Hint)

				sans := svid.SANS
				require.ElementsMatch(t, sans.DNS, []string{"foo.example.com", "bar.example.com"})
				require.ElementsMatch(t, sans.IP, []string{"192.168.1.1", "192.168.1.2"})

				dir, ok := spiffe.Destination.(*config.DestinationDirectory)
				require.True(t, ok)
				require.Equal(t, "/bar", dir.Path)
			},
		},
	})
}
