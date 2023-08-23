/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
