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
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keypaths"
)

func TestSSHConfigurator(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	homePath := t.TempDir()

	// This test gives a fake clock only to the SSH configurator and a real
	// clock to everything else, so that fakeClock.BlockUntilContext will
	// reliably only capture the SSH configuration loop and nothing else.
	fakeClock := clockwork.NewFakeClockAt(time.Now())
	realClock := clockwork.NewRealClock()

	fakeClientApp := newFakeClientApp(ctx, t, &fakeClientAppConfig{
		clusters: map[string]testClusterSpec{
			"cluster1": {
				leafClusters: map[string]testClusterSpec{
					"leaf1": {},
				},
			},
			"cluster2": {},
		},
		clock: realClock,
	})
	leafClusterCache, err := newLeafClusterCache(realClock)
	require.NoError(t, err)

	c := newSSHConfigurator(sshConfiguratorConfig{
		clientApplication: fakeClientApp,
		leafClusterCache:  leafClusterCache,
		homePath:          homePath,
		clock:             fakeClock,
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
	fakeClock.BlockUntilContext(ctx, 1)
	// Assert the config file contains both root clusters reported by
	// fakeClientApp.
	assertConfigFile("*.cluster1 *.cluster2 *.leaf1")

	// Add a new root and leaf cluster, wait until the configurator is blocked
	// in the loop, advance the clock, wait until the configurator is blocked
	// again indicating it should have updated the config and made it back into
	// the loop, and then assert that the new clusters are in the config file.
	fakeClientApp.cfg.clusters["cluster3"] = testClusterSpec{
		leafClusters: map[string]testClusterSpec{
			"leaf2": {},
		},
	}
	fakeClock.BlockUntilContext(ctx, 1)
	fakeClock.Advance(sshConfigurationUpdateInterval)
	fakeClock.BlockUntilContext(ctx, 1)
	assertConfigFile("*.cluster1 *.cluster2 *.cluster3 *.leaf1 *.leaf2")

	// Kill the configurator, wait for it to return, and assert that the config
	// file was deleted.
	cancel()
	require.ErrorIs(t, <-errC, context.Canceled)
	_, err = os.Stat(keypaths.VNetSSHConfigPath(homePath))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestAutoConfigureOpenSSH(t *testing.T) {
	d := t.TempDir()
	profilePath := filepath.Join(d, ".tsh")
	vnetSSHConfigPath := keypaths.VNetSSHConfigPath(profilePath)
	userOpenSSHConfigPath := filepath.Join(d, ".ssh", "config")
	expectedInclude := fmt.Sprintf(`# Include Teleport VNet generated configuration
Include "%s"

`, vnetSSHConfigPath)
	for _, tc := range []struct {
		desc                            string
		userOpenSSHConfigExists         bool
		userOpenSSHConfigContents       string
		expectAlreadyIncludedError      bool
		expectUserOpenSSHConfigContents string
	}{
		{
			// When the user OpenSSH config file doesn't exist, it should be
			// created with the include.
			desc:                            "no file",
			expectUserOpenSSHConfigContents: expectedInclude,
		},
		{
			// When the user OpenSSH config file already exists but it's empty,
			// the include should be added.
			desc:                            "empty file",
			userOpenSSHConfigExists:         true,
			expectUserOpenSSHConfigContents: expectedInclude,
		},
		{
			// When the user OpenSSH config file already exists with some
			// content, the include should be added at the top.
			desc:                            "not empty",
			userOpenSSHConfigExists:         true,
			userOpenSSHConfigContents:       "something\nsomethingelse\n",
			expectUserOpenSSHConfigContents: expectedInclude + "something\nsomethingelse\n",
		},
		{
			// When the user OpenSSH config file already includes VNet's config
			// file, it should return an AlreadyExists error and the file
			// should not be modified.
			desc:                            "already included",
			userOpenSSHConfigExists:         true,
			userOpenSSHConfigContents:       expectedInclude,
			expectAlreadyIncludedError:      true,
			expectUserOpenSSHConfigContents: expectedInclude,
		},
		{
			// When the user OpenSSH config file already includes VNet's config
			// file along with existing content, it should return an
			// AlreadyExists error and the file should not be modified.
			desc:                            "already included with extra content",
			userOpenSSHConfigExists:         true,
			userOpenSSHConfigContents:       "something\n" + expectedInclude + "somethingelse",
			expectAlreadyIncludedError:      true,
			expectUserOpenSSHConfigContents: "something\n" + expectedInclude + "somethingelse",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.userOpenSSHConfigExists {
				// Write the existing user OpenSSH config file if it's supposed
				// to exist for this test case.
				require.NoError(t, os.WriteFile(userOpenSSHConfigPath,
					[]byte(tc.userOpenSSHConfigContents), filePerms))
			}

			err := AutoConfigureOpenSSH(t.Context(), profilePath, userOpenSSHConfigPath)

			if tc.expectAlreadyIncludedError {
				assert.ErrorIs(t, err, trace.AlreadyExists("%s is already included in %s",
					vnetSSHConfigPath, userOpenSSHConfigPath))
			} else {
				assert.NoError(t, err)
			}

			contents, err := os.ReadFile(userOpenSSHConfigPath)
			require.NoError(t, err)
			assert.Equal(t, tc.expectUserOpenSSHConfigContents, string(contents))
		})
	}
}
