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

package common

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPingCommand(t *testing.T) {
	t.Parallel()

	tmpHomePath := t.TempDir()
	connector := mockConnector(t)
	alice := makeUserAlice(t)

	authProcess, proxyProcess := makeTestServers(t, withBootstrap(connector, alice))
	authServer := authProcess.GetAuthServer()
	require.NotNil(t, authServer)
	proxyAddr, err := proxyProcess.ProxyWebAddr()
	require.NoError(t, err)

	expectOutputContains := fmt.Sprintf(`"web_listen_addr": "%s"`, proxyAddr)

	// Test ping without logging in
	pingCommandArgs := []string{"ping", "--insecure", "--debug", "--proxy", proxyAddr.String()}
	mustRunAndContainsOutput(t, pingCommandArgs, tmpHomePath, expectOutputContains)

	// Test ping while logged in (--proxy is not required after login).
	err = Run(context.Background(), []string{
		"login", "--insecure", "--debug", "--proxy", proxyAddr.String(),
	}, setHomePath(tmpHomePath), setMockSSOLogin(authServer, alice, connector.GetName()))
	require.NoError(t, err)

	pingCommandArgs = []string{"ping", "--insecure", "--debug"}
	mustRunAndContainsOutput(t, pingCommandArgs, tmpHomePath, expectOutputContains)
}

func mustRunAndContainsOutput(t *testing.T, args []string, tmpHomePath, expectOutputContains string) {
	t.Helper()

	var buf bytes.Buffer
	err := Run(context.Background(), args, setHomePath(tmpHomePath), setCopyStdout(&buf))
	require.NoError(t, err)
	require.Contains(t, buf.String(), expectOutputContains)
}
