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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestLegacyCommand tests that the LegacyCommand properly parses its arguments
// and applies as expected onto a BotConfig.
func TestLegacyCommand(t *testing.T) {
	mockAction := configMutatorMock{}
	mockAction.On("action", mock.Anything).Return(nil)

	app, subcommand := buildMinimalKingpinApp("start")
	legacy := NewLegacyCommand(subcommand, mockAction.action)

	command, err := app.Parse([]string{
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
	})
	require.NoError(t, err)

	match, err := legacy.TryRun(command)
	require.NoError(t, err)
	require.True(t, match)

	mockAction.AssertCalled(t, "action", mock.Anything)

	require.Equal(t, "foo", legacy.Token)
	require.Len(t, legacy.CAPins, 1)
	require.Equal(t, "bar", legacy.CAPins[0])
	require.Equal(t, time.Minute*10, legacy.CertificateTTL)
	require.Equal(t, time.Minute*5, legacy.RenewalInterval)
	require.True(t, legacy.Oneshot)
	require.Equal(t, "0.0.0.0:8080", legacy.DiagAddr)
	require.Equal(t, "/foo", legacy.DataDir)
	require.Equal(t, "/bar", legacy.DestinationDir)
	require.Equal(t, "example.com:3024", legacy.AuthServer)

	// Convert these args to a BotConfig and check it.
	cfg, err := LoadConfigWithMutators(&GlobalArgs{}, legacy)
	require.NoError(t, err)

	token, err := cfg.Onboarding.Token()
	require.NoError(t, err)
	require.Equal(t, "foo", token)

	require.ElementsMatch(t, cfg.Onboarding.CAPins, []string{"bar"})
	require.Equal(t, time.Minute*10, cfg.CertificateTTL)
	require.Equal(t, time.Minute*5, cfg.RenewalInterval)
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
}
