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
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/trace"
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

func TestOpenSSHConfigIncludesVNetSSHConfig(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		desc        string
		profilePath string
		input       string
		expect      bool
	}{
		{
			desc: "empty",
		},
		{
			desc:        "macos tsh",
			profilePath: `/Users/user/.tsh`,
			input:       `Include /Users/user/.tsh/vnet_ssh_config`,
			expect:      true,
		},
		{
			desc:        "macos connect",
			profilePath: `/Users/user/Application Support/Teleport Connect/tsh`,
			input:       `Include "/Users/user/Application\ Support/Teleport\ Connect/tsh/vnet_ssh_config"`,
			expect:      true,
		},
		{
			desc:        "macos tsh not match connect",
			profilePath: `/Users/user/.tsh`,
			input:       `Include "/Users/user/Application\ Support/Teleport\ Connect/tsh/vnet_ssh_config"`,
		},
		{
			desc:        "macos connect not match tsh",
			profilePath: `/Users/user/Application Support/Teleport Connect/tsh`,
			input:       `Include /Users/user/.tsh/vnet_ssh_config`,
		},
		{
			// Unfortunately we can't use Windows-style \ separators in
			// profilePath we want these tests to pass on MacOS because
			// filepath.Base will not split on them unless it's compiled for
			// Windows. On Windows, both are treated as equivalent by
			// filepath.Base so only testing the case with / separators should
			// be okay.
			desc:        "windows tsh",
			profilePath: `C:/Users/User/.tsh`,
			input:       `Include "C:\\Users\\User\\.tsh\\vnet_ssh_config"`,
			expect:      true,
		},
		{
			desc:        "windows connect",
			profilePath: `C:/Users/User/AppData/Roaming/Teleport\ Connect/tsh`,
			input:       `Include "C:\\Users\\User\\AppData\\Roaming\\Teleport\ Connect\\tsh\\vnet_ssh_config"`,
			expect:      true,
		},
		{
			desc:        "windows tsh not match connect",
			profilePath: `C:/Users/User/.tsh`,
			input:       `Include "C:\\Users\\User\\AppData\\Roaming\\Teleport\ Connect\\tsh\\vnet_ssh_config"`,
		},
		{
			desc:        "windows connect not match tsh",
			profilePath: `C:/Users/User/AppData/Roaming/Teleport/ Connect/tsh`,
			input:       `Include "C:\\Users\\User\\.tsh\\vnet_ssh_config"`,
		},
		{
			desc:        "some other file",
			profilePath: `/Users/user/.tsh`,
			input:       `Include /Users/user/.tsh/ssh_config`,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			result, err := fileIncludesVNetSSHConfig(
				tc.profilePath,
				strings.NewReader(tc.input),
			)
			require.NoError(t, err)
			require.Equal(t, tc.expect, result)
		})
	}
}

func TestAutoConfigureOpenSSH(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		desc             string
		profilePath      string
		existingContents string
		expectError      error
		expectResult     string
	}{
		{
			desc:        "empty",
			profilePath: "~/.tsh",
			expectResult: `
# Include VNet SSH configuration options
Include "~/.tsh/vnet_ssh_config"
`,
		},
		{
			desc:        "some existing config",
			profilePath: "~/.tsh",
			existingContents: `Host *.example.com
  IdentityFile /my/identity
`,
			expectResult: `Host *.example.com
  IdentityFile /my/identity

# Include VNet SSH configuration options
Include "~/.tsh/vnet_ssh_config"
`,
		},
		{
			desc:        "already includes vnet config in same profile path",
			profilePath: "~/.tsh",
			existingContents: `
# Include VNet SSH configuration options
Include "~/.tsh/vnet_ssh_config"
`,
			expectError: trace.BadParameter("user OpenSSH config file already includes vnet_ssh_config"),
			// Contents should be unmodified.
			expectResult: `
# Include VNet SSH configuration options
Include "~/.tsh/vnet_ssh_config"
`,
		},
		{
			desc:        "already includes vnet config in other profile path",
			profilePath: "/Users/user/Application Support/Teleport Connect/tsh",
			existingContents: `
# Include VNet SSH configuration options
Include "~/.tsh/vnet_ssh_config"
`,
			// It's okay to include both, only the one that actually exists will be
			// included, and it only exists while VNet is running so only one should exist
			// at a time. This allows "tsh vnet" and Connect to both work as long as you
			// run them at different times.
			expectResult: `
# Include VNet SSH configuration options
Include "~/.tsh/vnet_ssh_config"

# Include VNet SSH configuration options
Include "/Users/user/Application Support/Teleport Connect/tsh/vnet_ssh_config"
`,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			userSSHConfigPath := filepath.Join(t.TempDir(), "test_ssh_config"+uuid.NewString())
			if len(tc.existingContents) > 0 {
				f, err := os.Create(userSSHConfigPath)
				require.NoError(t, err)
				_, err = f.WriteString(tc.existingContents)
				require.NoError(t, err)
				require.NoError(t, f.Close())
			}

			err := autoConfigureOpenSSHConfigFile(tc.profilePath, userSSHConfigPath)
			if tc.expectError != nil {
				require.ErrorIs(t, err, tc.expectError)
			}

			result, err := os.ReadFile(userSSHConfigPath)
			require.NoError(t, err)
			require.Equal(t, tc.expectResult, string(result))
		})
	}
}
