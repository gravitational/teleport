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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tbot/botfs"
	"github.com/gravitational/teleport/lib/tbot/config"
)

func TestSharedStartArgs(t *testing.T) {
	app, subcommand := buildMinimalKingpinApp("test")
	args := newSharedStartArgs(subcommand)
	_, err := app.Parse([]string{
		"test",
		"--token=foo",
		"--ca-pin=bar",
		"--certificate-ttl=10m",
		"--renewal-interval=5m",
		"--join-method=github",
		"--oneshot",
		"--diag-addr=0.0.0.0:8080",
		"--storage=file:///foo/bar",
		"--proxy-server=example.teleport.sh:443",
	})
	require.NoError(t, err)

	require.Equal(t, "foo", args.Token)
	require.Len(t, args.CAPins, 1)
	require.Equal(t, "bar", args.CAPins[0])
	require.Equal(t, time.Minute*10, args.CertificateTTL)
	require.Equal(t, time.Minute*5, args.RenewalInterval)
	require.True(t, args.Oneshot)
	require.Equal(t, "0.0.0.0:8080", args.DiagAddr)
	require.Equal(t, "file:///foo/bar", args.Storage)
	require.Equal(t, "example.teleport.sh:443", args.ProxyServer)

	// Convert these args to a BotConfig.
	cfg, err := LoadConfigWithMutators(&GlobalArgs{}, args)
	require.NoError(t, err)

	token, err := cfg.Onboarding.Token()
	require.NoError(t, err)
	require.Equal(t, "foo", token)

	require.ElementsMatch(t, cfg.Onboarding.CAPins, []string{"bar"})
	require.Equal(t, time.Minute*10, cfg.CertificateLifetime.TTL)
	require.Equal(t, time.Minute*5, cfg.CertificateLifetime.RenewalInterval)
	require.Equal(t, types.JoinMethodGitHub, cfg.Onboarding.JoinMethod)
	require.True(t, cfg.Oneshot)
	require.Equal(t, "0.0.0.0:8080", cfg.DiagAddr)

	dir, ok := cfg.Storage.Destination.(*config.DestinationDirectory)
	require.True(t, ok)
	require.Equal(t, "/foo/bar", dir.Path)
}

func TestSharedDestinationArgs(t *testing.T) {
	dir := t.TempDir()

	app, subcommand := buildMinimalKingpinApp("test")
	args := newSharedDestinationArgs(subcommand)
	_, err := app.Parse([]string{
		"test",
		fmt.Sprintf("--destination=file://%s", dir),
		"--reader-user=123",
		"--reader-user=456",
		"--reader-group=789",
		"--reader-group=101112",
	})
	require.NoError(t, err)

	require.ElementsMatch(t, args.ReaderUsers, []string{"123", "456"})
	require.ElementsMatch(t, args.ReaderGroups, []string{"789", "101112"})

	dest, err := args.BuildDestination()
	require.NoError(t, err)

	dd, ok := dest.(*config.DestinationDirectory)
	require.True(t, ok)

	require.ElementsMatch(t, dd.Readers, []*botfs.ACLSelector{
		{User: "123"},
		{User: "456"},
		{Group: "789"},
		{Group: "101112"},
	})
}
