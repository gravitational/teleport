//go:build !windows

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

package tools_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/api/constants"
	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/autoupdate"
	"github.com/gravitational/teleport/integration/autoupdate/tools/updater"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/autoupdate/tools"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"
	testserver "github.com/gravitational/teleport/tool/teleport/testenv"
)

// TestAliasLoginWithUpdater runs test cluster with enabled auto updates for client tools,
// checks that defined alias in tsh configuration is replaced to the proper login command
// and after auto update this not leads to recursive alias re-execution.
//
// # Managed updates: enabled.
// $ tsh loginByAlias
// $ tctl status
// $ tsh version
// Teleport v3.2.1
func TestAliasLoginWithUpdater(t *testing.T) {
	ctx := context.Background()

	rootServer, homeDir, installDir := bootstrapTestServer(t)
	setupManagedUpdates(t, rootServer.GetAuthServer(), autoupdate.ToolsUpdateModeEnabled, testVersions[1])

	// Assign alias to the login command for test cluster.
	proxyAddr, err := rootServer.ProxyWebAddr()
	require.NoError(t, err)

	// Fetch compiled test binary and install to tools dir [v1.2.3].
	updater := tools.NewUpdater(installDir, testVersions[0], tools.WithBaseURL(baseURL))
	require.NoError(t, updater.Update(ctx, testVersions[0]))
	tshPath, err := updater.ToolPath("tsh", testVersions[0])
	require.NoError(t, err)
	tctlPath, err := updater.ToolPath("tctl", testVersions[0])
	require.NoError(t, err)

	configPath := filepath.Join(homeDir, client.TSHConfigPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0700))
	out, err := yaml.Marshal(client.TSHConfig{
		Aliases: map[string]string{
			"loginalice": fmt.Sprintf(
				"%s login --insecure --proxy %s --user alice --auth %s",
				tshPath, proxyAddr, constants.LocalConnector,
			),
		},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, out, 0600))

	// Execute alias command which must be transformed to the login command.
	// Since client tools autoupdates is enabled and target version is set
	// in the test cluster, we have to update client tools to new version.
	cmd := exec.CommandContext(ctx, tshPath, "loginalice")
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run())

	// Verify tctl status after login.
	cmd = exec.CommandContext(ctx, tctlPath, "status", "--insecure")
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run())

	// Run version command to verify that login command executed auto update and
	// tsh was upgraded to [v3.2.1].
	cmd = exec.CommandContext(ctx, tshPath, "version")
	out, err = cmd.Output()
	require.NoError(t, err)
	matches := pattern.FindStringSubmatch(string(out))
	require.Len(t, matches, 2)
	require.Equal(t, testVersions[1], matches[1])

	// Verifies that version commands shows version re-executed from.
	require.Contains(t, string(out), fmt.Sprintf("Re-executed from version: %s", testVersions[0]))
}

// TestLoginWithUpdaterAndProfile runs test cluster with disabled managed updates for client tools,
// verifies that if we set env variable during login we keep using updated version.
//
// # Managed updates: disabled.
// $ TELEPORT_TOOLS_VERSION=3.2.1 tsh login --proxy proxy.example.com
// # Check that created profile after login has enabled autoupdates flag.
// $ tsh version
// Teleport v3.2.1
func TestLoginWithUpdaterAndProfile(t *testing.T) {
	ctx := context.Background()

	rootServer, _, installDir := bootstrapTestServer(t)
	setupManagedUpdates(t, rootServer.GetAuthServer(), autoupdate.ToolsUpdateModeDisabled, testVersions[1])

	proxyAddr, err := rootServer.ProxyWebAddr()
	require.NoError(t, err)

	// Fetch compiled test binary and install to tools dir [v1.2.3].
	updater := tools.NewUpdater(installDir, testVersions[0], tools.WithBaseURL(baseURL))
	require.NoError(t, updater.Update(ctx, testVersions[0]))
	tshPath, err := updater.ToolPath("tsh", testVersions[0])
	require.NoError(t, err)

	// First login with set version during login process
	t.Setenv("TELEPORT_TOOLS_VERSION", testVersions[1])
	cmd := exec.CommandContext(ctx, tshPath,
		"login", "--proxy", proxyAddr.String(), "--insecure", "--user", "alice", "--auth", constants.LocalConnector)
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run())
	// Unset the version after update process.
	require.NoError(t, os.Unsetenv("TELEPORT_TOOLS_VERSION"))

	// Run version command to verify that login command executed auto update and
	// tsh was upgraded to [v3.2.1].
	cmd = exec.CommandContext(ctx, tshPath, "version")
	out, err := cmd.Output()
	require.NoError(t, err)
	matches := pattern.FindStringSubmatch(string(out))
	require.Len(t, matches, 2)
	require.Equal(t, testVersions[1], matches[1])
}

// TestLoginWithDisabledUpdateInProfile runs test cluster with enabled managed updates for client tools,
// verifies that after first update and disabling.
//
// # Managed updates: disabled.
// $ TELEPORT_TOOLS_VERSION=3.2.1 tsh version
// Teleport v3.2.1
// $ tsh login --proxy proxy.example.com
// $ tsh version
// Teleport v1.2.3
func TestLoginWithDisabledUpdateInProfile(t *testing.T) {
	ctx := context.Background()

	rootServer, _, installDir := bootstrapTestServer(t)
	setupManagedUpdates(t, rootServer.GetAuthServer(), autoupdate.ToolsUpdateModeDisabled, testVersions[1])

	proxyAddr, err := rootServer.ProxyWebAddr()
	require.NoError(t, err)

	// Fetch compiled test binary and install to tools dir [v1.2.3].
	updater := tools.NewUpdater(installDir, testVersions[0], tools.WithBaseURL(baseURL))
	require.NoError(t, updater.Update(ctx, testVersions[0]))
	tshPath, err := updater.ToolPath("tsh", testVersions[0])
	require.NoError(t, err)

	// Set env variable to forcibly request update on version command.
	t.Setenv("TELEPORT_TOOLS_VERSION", testVersions[1])
	cmd := exec.CommandContext(ctx, tshPath, "version")
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	require.NoError(t, err)
	// Check the version.
	matches := pattern.FindStringSubmatch(string(out))
	require.Len(t, matches, 2)
	require.Equal(t, testVersions[1], matches[1])
	// Unset the version after update process.
	require.NoError(t, os.Unsetenv("TELEPORT_TOOLS_VERSION"))

	// Second login has to update profile and disable further managed updates.
	cmd = exec.CommandContext(ctx, tshPath,
		"login", "--proxy", proxyAddr.String(), "--insecure", "--user", "alice", "--auth", constants.LocalConnector)
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run())

	// Run version command to verify that login command executed auto update and
	// tsh was upgraded to [v3.2.1].
	cmd = exec.CommandContext(ctx, tshPath, "version")
	out, err = cmd.Output()
	require.NoError(t, err)
	// Check the version.
	matches = pattern.FindStringSubmatch(string(out))
	require.Len(t, matches, 2)
	fmt.Println(string(out))
	require.Equal(t, testVersions[0], matches[1])
}

// TestLoginWithDisabledUpdateForcedByEnv verifies that on disabled cluster we are still
// able to update client tools by always setting the environment variable.
//
// # Managed updates: disabled.
// $ tsh login --proxy proxy.example.com
// $ TELEPORT_TOOLS_VERSION=3.2.1 tsh version
// Teleport v3.2.1
// $ tsh version
// Teleport v1.2.3
func TestLoginWithDisabledUpdateForcedByEnv(t *testing.T) {
	ctx := context.Background()

	rootServer, _, installDir := bootstrapTestServer(t)
	setupManagedUpdates(t, rootServer.GetAuthServer(), autoupdate.ToolsUpdateModeDisabled, testVersions[1])

	proxyAddr, err := rootServer.ProxyWebAddr()
	require.NoError(t, err)

	// Fetch compiled test binary and install to tools dir [v1.2.3].
	updater := tools.NewUpdater(installDir, testVersions[0], tools.WithBaseURL(baseURL))
	require.NoError(t, updater.Update(ctx, testVersions[0]))
	tshPath, err := updater.ToolPath("tsh", testVersions[0])
	require.NoError(t, err)

	// Second login has to update profile and disable further managed updates.
	cmd := exec.CommandContext(ctx, tshPath,
		"login", "--proxy", proxyAddr.String(), "--insecure", "--user", "alice", "--auth", constants.LocalConnector)
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run())

	// Trying to forcibly use specific version not during login.
	t.Setenv("TELEPORT_TOOLS_VERSION", testVersions[1])
	cmd = exec.CommandContext(ctx, tshPath, "version")
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	require.NoError(t, err)
	// Check that version is used that requested from env variable.
	matches := pattern.FindStringSubmatch(string(out))
	require.Len(t, matches, 2)
	fmt.Println(string(out))
	require.Equal(t, testVersions[1], matches[1])
	// Unset the version after update process.
	require.NoError(t, os.Unsetenv("TELEPORT_TOOLS_VERSION"))

	// Run version command to verify that login command executed auto update and
	// tsh is version [v1.2.3] since it was requested not during login and cluster
	// has disabled managed updates.
	cmd = exec.CommandContext(ctx, tshPath, "version")
	out, err = cmd.Output()
	require.NoError(t, err)
	matches = pattern.FindStringSubmatch(string(out))
	require.Len(t, matches, 2)
	fmt.Println(string(out))
	require.Equal(t, testVersions[0], matches[1])
}

func bootstrapTestServer(t *testing.T) (*service.TeleportProcess, string, string) {
	t.Helper()
	homeDir := filepath.Join(t.TempDir(), "home")
	require.NoError(t, os.MkdirAll(homeDir, 0700))
	installDir := filepath.Join(t.TempDir(), "local")
	require.NoError(t, os.MkdirAll(installDir, 0700))

	t.Setenv(types.HomeEnvVar, homeDir)

	alice, err := types.NewUser("alice")
	require.NoError(t, err)
	alice.SetRoles([]string{"access"})

	// Disable 2fa to simplify login for test.
	ap, err := types.NewAuthPreferenceFromConfigFile(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOff,
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
	})
	require.NoError(t, err)

	rootServer := testserver.MakeTestServer(t,
		testserver.WithBootstrap(alice),
		testserver.WithClusterName(t, "root"),
		testserver.WithAuthPreference(ap),
	)
	authService := rootServer.GetAuthServer()

	// Set password for the cluster login.
	password, err := utils.CryptoRandomHex(6)
	require.NoError(t, err)
	t.Setenv(updater.TestPassword, password)
	err = authService.UpsertPassword("alice", []byte(password))
	require.NoError(t, err)

	return rootServer, homeDir, installDir
}

func setupManagedUpdates(t *testing.T, server *auth.Server, muMode string, muVersion string) {
	t.Helper()
	ctx := context.Background()
	config, err := autoupdate.NewAutoUpdateConfig(&autoupdatev1pb.AutoUpdateConfigSpec{
		Tools: &autoupdatev1pb.AutoUpdateConfigSpecTools{
			Mode: muMode,
		},
	})
	require.NoError(t, err)
	version, err := autoupdate.NewAutoUpdateVersion(&autoupdatev1pb.AutoUpdateVersionSpec{
		Tools: &autoupdatev1pb.AutoUpdateVersionSpecTools{
			TargetVersion: muVersion,
		},
	})
	require.NoError(t, err)
	_, err = server.UpsertAutoUpdateConfig(ctx, config)
	require.NoError(t, err)
	_, err = server.UpsertAutoUpdateVersion(ctx, version)
	require.NoError(t, err)
}
