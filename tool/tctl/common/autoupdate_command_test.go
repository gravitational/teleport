/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package common

import (
	"bytes"
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

// TestClientToolsAutoUpdateCommands verifies all commands related to client auto updates, by
// enabling/disabling auto update, setting the target version and retrieve it.
func TestClientToolsAutoUpdateCommands(t *testing.T) {
	ctx := context.Background()
	process := testenv.MakeTestServer(t)
	server := process.GetAuthServer()

	// Check that AutoUpdateConfig and AutoUpdateVersion are not created.
	_, err := server.GetAutoUpdateConfig(ctx)
	require.True(t, trace.IsNotFound(err))
	_, err = server.GetAutoUpdateVersion(ctx)
	require.True(t, trace.IsNotFound(err))

	authClient := testenv.MakeDefaultAuthClient(t, process)

	// Enable client tools auto updates to check that AutoUpdateConfig resource is modified.
	_, err = runAutoUpdateCommand(t, authClient, []string{"client-tools", "enable"})
	require.NoError(t, err)

	config, err := authClient.GetAutoUpdateConfig(ctx)
	require.NoError(t, err)
	assert.Equal(t, "enabled", config.Spec.Tools.Mode)

	// Disable client tools auto updates to check that AutoUpdateConfig resource is modified.
	_, err = runAutoUpdateCommand(t, authClient, []string{"client-tools", "disable"})
	require.NoError(t, err)

	config, err = authClient.GetAutoUpdateConfig(ctx)
	require.NoError(t, err)
	assert.Equal(t, "disabled", config.Spec.Tools.Mode)

	// Set target version for client tools auto updates.
	_, err = runAutoUpdateCommand(t, authClient, []string{"client-tools", "target", "1.2.3"})
	require.NoError(t, err)

	version, err := authClient.GetAutoUpdateVersion(ctx)
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", version.Spec.Tools.TargetVersion)

	getBuf, err := runAutoUpdateCommand(t, authClient, []string{"client-tools", "status", "--format=json"})
	require.NoError(t, err)
	response := mustDecodeJSON[getResponse](t, getBuf)
	assert.Equal(t, "1.2.3", response.TargetVersion)
	assert.Equal(t, "disabled", response.Mode)

	// Make same request with proxy flag to read command expecting the same
	// response from `webapi/find` endpoint.
	proxy, err := process.ProxyWebAddr()
	require.NoError(t, err)
	getProxyBuf, err := runAutoUpdateCommand(t, authClient, []string{"client-tools", "status", "--proxy=" + proxy.Addr, "--format=json"})
	require.NoError(t, err)
	response = mustDecodeJSON[getResponse](t, getProxyBuf)
	assert.Equal(t, "1.2.3", response.TargetVersion)
	assert.Equal(t, "disabled", response.Mode)

	// Set clear flag for the target version update to check that it is going to be reset.
	_, err = runAutoUpdateCommand(t, authClient, []string{"client-tools", "target", "--clear"})
	require.NoError(t, err)
	version, err = authClient.GetAutoUpdateVersion(ctx)
	require.NoError(t, err)
	assert.Nil(t, version.Spec.Tools)
}

func runAutoUpdateCommand(t *testing.T, client *authclient.Client, args []string) (*bytes.Buffer, error) {
	var stdoutBuff bytes.Buffer
	command := &AutoUpdateCommand{
		stdout:   &stdoutBuff,
		insecure: true,
	}

	cfg := servicecfg.MakeDefaultConfig()
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	app := utils.InitCLIParser("tctl", GlobalHelpString)
	command.Initialize(app, cfg)

	selectedCmd, err := app.Parse(append([]string{"autoupdate"}, args...))
	require.NoError(t, err)

	_, err = command.TryRun(context.Background(), selectedCmd, client)
	return &stdoutBuff, err
}
