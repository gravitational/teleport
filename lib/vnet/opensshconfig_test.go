// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package vnet

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keypaths"
)

func TestSSHConfigurator(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	clock := clockwork.NewFakeClockAt(time.Now())
	homePath := t.TempDir()

	fakeClientApp := newFakeClientApp(ctx, t, &fakeClientAppConfig{
		clusters: map[string]testClusterSpec{
			"cluster1": {},
			"cluster2": {},
		},
		// Give the fake client app a different clock so we can rely on
		// clock.BlockUntilContext only capturing the SSH configuration loop.
		clock: clockwork.NewRealClock(),
	})

	c := newSSHConfigurator(sshConfiguratorConfig{
		clientApplication: fakeClientApp,
		homePath:          homePath,
		clock:             clock,
	})
	errC := make(chan error)
	go func() {
		errC <- c.runConfigurationLoop(ctx)
	}()

	// Intentionally not using the template defined in the production code to
	// test that it actually produces output that looks like this.
	expectedConfigFile := func(expectedHosts string) string {
		return fmt.Sprintf(`Host %s
    IdentityFile "%s/id_vnet"
    GlobalKnownHostsFile "%s/vnet_known_hosts"
    UserKnownHostsFile /dev/null
    StrictHostKeyChecking yes
    IdentitiesOnly yes
`,
			expectedHosts,
			homePath, homePath)
	}

	assertConfigFile := func(expectedHosts string) {
		t.Helper()
		expected := expectedConfigFile(expectedHosts)
		contents, err := os.ReadFile(keypaths.VNetSSHConfigPath(homePath))
		require.NoError(t, err)
		require.Equal(t, expected, string(contents))
	}

	// Wait until the configurator has had a chance to write the initial config
	// file and then get blocked in the loop.
	clock.BlockUntilContext(ctx, 1)
	// Assert the config file contains both root clusters reported by
	// fakeClientApp.
	assertConfigFile("*.cluster1 *.cluster2")

	// Add a root cluster, wait until the configurator is blocked in the loop,
	// advance the clock, wait until the configurator is blocked again
	// indicating it should have updated the config and made it back into the
	// loop, and then assert that the new cluster is in the config file.
	fakeClientApp.cfg.clusters["cluster3"] = testClusterSpec{}
	clock.BlockUntilContext(ctx, 1)
	clock.Advance(sshConfigurationUpdateInterval)
	clock.BlockUntilContext(ctx, 1)
	assertConfigFile("*.cluster1 *.cluster2 *.cluster3")

	// Kill the configurator, wait for it to return, and assert that the config
	// file was deleted.
	cancel()
	require.ErrorIs(t, <-errC, context.Canceled)
	_, err := os.Stat(keypaths.VNetSSHConfigPath(homePath))
	require.ErrorIs(t, err, os.ErrNotExist)
}
