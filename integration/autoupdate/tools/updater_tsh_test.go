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
	"github.com/gravitational/teleport/lib/autoupdate/tools"
	"github.com/gravitational/teleport/lib/client"
	testserver "github.com/gravitational/teleport/tool/teleport/testenv"
)

// TestAliasLoginWithUpdater runs test cluster with enabled auto updates for client tools,
// checks that defined alias in tsh configuration is replaced to the proper login command
// and after auto update this not leads to recursive alias re-execution.
func TestAliasLoginWithUpdater(t *testing.T) {
	ctx := context.Background()
	t.Setenv(types.HomeEnvVar, toolsDir)

	alice, err := types.NewUser("alice")
	require.NoError(t, err)
	alice.SetRoles([]string{"access"})

	// Enable client tools auto updates and set the target version.
	config, err := autoupdate.NewAutoUpdateConfig(&autoupdatev1pb.AutoUpdateConfigSpec{
		Tools: &autoupdatev1pb.AutoUpdateConfigSpecTools{
			Mode: autoupdate.ToolsUpdateModeEnabled,
		},
	})
	require.NoError(t, err)
	version, err := autoupdate.NewAutoUpdateVersion(&autoupdatev1pb.AutoUpdateVersionSpec{
		Tools: &autoupdatev1pb.AutoUpdateVersionSpecTools{
			TargetVersion: testVersions[1], // [v3.2.1]
		},
	})
	require.NoError(t, err)

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
	_, err = authService.UpsertAutoUpdateConfig(ctx, config)
	require.NoError(t, err)
	_, err = authService.UpsertAutoUpdateVersion(ctx, version)
	require.NoError(t, err)
	err = authService.UpsertPassword("alice", []byte(updater.TestPassword))
	require.NoError(t, err)

	// Assign alias to the login command for test cluster.
	proxyAddr, err := rootServer.ProxyWebAddr()
	require.NoError(t, err)
	configPath := filepath.Join(toolsDir, client.TSHConfigPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0700))
	executable := filepath.Join(toolsDir, "tsh")
	out, err := yaml.Marshal(client.TSHConfig{
		Aliases: map[string]string{
			"loginalice": fmt.Sprintf(
				"%s login --insecure --proxy %s --user alice --auth %s",
				executable, proxyAddr, constants.LocalConnector,
			),
		},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, out, 0600))

	// Fetch compiled test binary and install to tools dir [v1.2.3].
	err = tools.NewUpdater(toolsDir, testVersions[0], tools.WithBaseURL(baseURL)).Update(ctx, testVersions[0])
	require.NoError(t, err)

	// Execute alias command which must be transformed to the login command.
	// Since client tools autoupdates is enabled and target version is set
	// in the test cluster, we have to update client tools to new version.
	cmd := exec.CommandContext(ctx, executable, "loginalice")
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run())

	// Verify tctl status after login.
	cmd = exec.CommandContext(ctx, filepath.Join(toolsDir, "tctl"), "status", "--insecure")
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run())

	// Run version command to verify that login command executed auto update and
	// tsh was upgraded to [v3.2.1].
	cmd = exec.CommandContext(ctx, executable, "version")
	out, err = cmd.Output()
	require.NoError(t, err)

	matches := pattern.FindStringSubmatch(string(out))
	require.Len(t, matches, 2)
	require.Equal(t, testVersions[1], matches[1])
}
