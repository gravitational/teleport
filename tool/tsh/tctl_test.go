/*
Copyright 2015-2017 Gravitational, Inc.

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

package main

import (
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/tool/tctl/common"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigFromProfile(t *testing.T) {
	tmpHomePath := t.TempDir()
	connector := mockConnector(t)

	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	alice.SetRoles([]string{"access"})

	authProcess, proxyProcess := makeTestServers(t, withBootstrap(connector, alice))

	authServer := authProcess.GetAuthServer()
	require.NotNil(t, authServer)

	proxyAddr, err := proxyProcess.ProxyWebAddr()
	require.NoError(t, err)

	err = Run([]string{
		"login",
		"--insecure",
		"--debug",
		"--auth", connector.GetName(),
		"--proxy", proxyAddr.String(),
	}, setHomePath(tmpHomePath), cliOption(func(cf *CLIConf) error {
		cf.mockSSOLogin = mockSSOLogin(t, authServer, alice)
		return nil
	}))
	require.NoError(t, err)

	tests := []struct {
		name string
		ccf  *common.GlobalCLIFlags
		cfg  *service.Config
		want error
	}{
		{
			name: "teleportHome is valid dir",
			ccf:  &common.GlobalCLIFlags{},
			cfg: &service.Config{
				TeleportHome: tmpHomePath,
			},
			want: nil,
		}, {
			name: "teleportHome is nonexistent dir",
			ccf:  &common.GlobalCLIFlags{},
			cfg: &service.Config{
				TeleportHome: "some/dir/that/does/not/exist",
			},
			want: trace.NotFound("profile is not found"),
		}, {
			name: "teleportHome is not specified",
			ccf:  &common.GlobalCLIFlags{},
			cfg:  &service.Config{},
			want: trace.NotFound("profile is not found"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := common.LoadConfigFromProfile(tc.ccf, tc.cfg)
			if tc.want != nil {
				require.Error(t, err, tc.want)
				return
			}
			require.NoError(t, err)
		})
	}
}
