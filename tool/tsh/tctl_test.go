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
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	toolcommon "github.com/gravitational/teleport/tool/common"
	"github.com/gravitational/teleport/tool/tctl/common"
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

	err = Run(context.Background(), []string{
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
		cfg  *servicecfg.Config
		want error
	}{
		{
			name: "teleportHome is valid dir",
			ccf:  &common.GlobalCLIFlags{},
			cfg: &servicecfg.Config{
				TeleportHome: tmpHomePath,
			},
			want: nil,
		}, {
			name: "teleportHome is nonexistent dir",
			ccf:  &common.GlobalCLIFlags{},
			cfg: &servicecfg.Config{
				TeleportHome: "some/dir/that/does/not/exist",
			},
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

func TestRemoteTctlWithProfile(t *testing.T) {
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

	err = Run(context.Background(), []string{
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

	t.Setenv(types.HomeEnvVar, tmpHomePath)

	tests := []struct {
		desc            string
		commands        []common.CLICommand
		args            []string
		wantErrContains string
	}{
		{
			desc:            "fails with untrusted certificate error without insecure mode",
			commands:        []common.CLICommand{&common.StatusCommand{}},
			args:            []string{"status"},
			wantErrContains: "certificate is not trusted",
		},
		{
			desc:     "success with insecure mode",
			commands: []common.CLICommand{&common.StatusCommand{}},
			args:     []string{"status", "--insecure"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			err := common.TryRun(tt.commands, tt.args)
			if tt.wantErrContains != "" {
				var exitError *toolcommon.ExitCodeError
				require.True(t, errors.As(err, &exitError))
				require.ErrorContains(t, err, tt.wantErrContains)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestSetAuthServerFlagWhileLoggedIn(t *testing.T) {
	tmpHomePath := filepath.Clean(t.TempDir())
	connector := mockConnector(t)

	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	alice.SetRoles([]string{"access"})

	authProcess, proxyProcess := makeTestServers(t, withBootstrap(connector, alice))

	authServer := authProcess.GetAuthServer()
	require.NotNil(t, authServer)

	authAddr, err := authProcess.AuthAddr()
	require.NoError(t, err)

	proxyAddr, err := proxyProcess.ProxyWebAddr()
	require.NoError(t, err)

	err = Run(context.Background(), []string{
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
	// we're now logged in with a profile in tmpHomePath.

	tests := []struct {
		desc           string
		authServerFlag []string
		want           []utils.NetAddr
	}{
		{
			desc: "sets default web proxy addr without auth server flag",
			want: []utils.NetAddr{*proxyAddr},
		},
		{
			desc:           "sets auth addr from auth server flag ignoring profile setting",
			authServerFlag: []string{authAddr.String()},
			want:           []utils.NetAddr{*authAddr},
		},
		{
			desc:           "sets auth addr from auth server flag when profile is not found",
			authServerFlag: []string{"site.not.in.profile.com:3080"},
			want: []utils.NetAddr{
				{
					Addr:        "site.not.in.profile.com:3080",
					AddrNetwork: "tcp",
					Path:        "",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ccf := &common.GlobalCLIFlags{}
			ccf.AuthServerAddr = tt.authServerFlag

			cfg := &servicecfg.Config{}
			cfg.TeleportHome = tmpHomePath
			// this is needed for the case where the --auth-server=host is not found in profile.
			// ApplyConfig will try to read local auth server identity if the profile is not found.
			cfg.DataDir = authProcess.Config.DataDir

			_, err = common.ApplyConfig(ccf, cfg)
			require.NoError(t, err)
			require.NotEmpty(t, cfg.AuthServerAddresses(), "auth servers should be set to a non-empty default if not specified")

			require.ElementsMatch(t, tt.want, cfg.AuthServerAddresses())
		})
	}
}
