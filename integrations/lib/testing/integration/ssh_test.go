/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package integration

import (
	"bytes"
	"fmt"
	"os/user"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
)

type IntegrationSSHSuite struct {
	SSHSetup
}

func TestIntegrationSSH(t *testing.T) { suite.Run(t, &IntegrationSSHSuite{}) }

func (s *IntegrationSSHSuite) SetupTest() {
	s.SSHSetup.SetupService()
}

func (s *IntegrationSSHSuite) TestSSH() {
	t := s.T()
	me, err := user.Current()
	require.NoError(t, err)
	var bootstrap Bootstrap
	role, err := bootstrap.AddRole(me.Username, types.RoleSpecV6{Allow: types.RoleConditions{
		Logins:     []string{me.Username},
		NodeLabels: types.Labels{types.Wildcard: utils.Strings{types.Wildcard}},
	}})
	require.NoError(t, err)
	user, err := bootstrap.AddUserWithRoles(me.Username, role.GetName())
	require.NoError(t, err)
	err = s.Integration.Bootstrap(s.Context(), s.Auth, bootstrap.Resources())
	require.NoError(t, err)
	identityPath, err := s.Integration.Sign(s.Context(), s.Auth, user.GetName())
	require.NoError(t, err)
	tshCmd := s.Integration.NewTsh(s.Proxy.WebProxyAddr().String(), identityPath)
	cmd := tshCmd.SSHCommand(s.Context(), user.GetName()+"@localhost")

	stdinPipe, err := cmd.StdinPipe()
	require.NoError(t, err)

	cmdStdout := &bytes.Buffer{}
	cmdStderr := &bytes.Buffer{}

	cmd.Stdout = cmdStdout
	cmd.Stderr = cmdStderr

	err = cmd.Start()
	require.NoError(t, err)

	_, err = stdinPipe.Write([]byte("echo MYUSER=$USER\r\n"))
	require.NoError(t, err)

	_, err = stdinPipe.Write([]byte("exit\r\n"))
	require.NoError(t, err)

	err = cmd.Wait()
	require.NoError(t, err)

	require.Contains(t, cmdStdout.String(), fmt.Sprintf("MYUSER=%s", user.GetName()))
}
