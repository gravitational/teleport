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

package common

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	toolcommon "github.com/gravitational/teleport/tool/common"
	"github.com/gravitational/teleport/tool/tctl/common"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
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
		"--proxy", proxyAddr.String(),
	}, setHomePath(tmpHomePath), setMockSSOLogin(authServer, alice, connector.GetName()))
	require.NoError(t, err)

	tests := []struct {
		name string
		ccf  *tctlcfg.GlobalCLIFlags
		cfg  *servicecfg.Config
		want error
	}{
		{
			name: "teleportHome is valid dir",
			ccf:  &tctlcfg.GlobalCLIFlags{},
			cfg: &servicecfg.Config{
				TeleportHome: tmpHomePath,
			},
			want: nil,
		}, {
			name: "teleportHome is nonexistent dir",
			ccf:  &tctlcfg.GlobalCLIFlags{},
			cfg: &servicecfg.Config{
				TeleportHome: "some/dir/that/does/not/exist",
			},
			want: trace.NotFound("current-profile is not set"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tctlcfg.LoadConfigFromProfile(tc.ccf, tc.cfg)
			if tc.want != nil {
				require.ErrorIs(t, err, tc.want)
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
		"--proxy", proxyAddr.String(),
	}, setHomePath(tmpHomePath), setMockSSOLogin(authServer, alice, connector.GetName()))
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
				require.ErrorAs(t, err, &exitError)
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

	// we wont actually run this agent, just make a config file to test with.
	fileConfigAgent := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "false",
				ListenAddress: authProcess.Config.Auth.ListenAddr.String(),
			},
		},
		SSH: config.SSH{
			Service: config.Service{
				EnabledFlag: "true",
			},
		},
	}

	err = Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--proxy", proxyAddr.String(),
	}, setHomePath(tmpHomePath), setMockSSOLogin(authServer, alice, connector.GetName()))
	require.NoError(t, err)
	// we're now logged in with a profile in tmpHomePath.

	tests := []struct {
		desc           string
		authServerFlag []string
		configFileFlag string
		want           []utils.NetAddr
	}{
		{
			desc:           "ignores agent config file and loads profile setting",
			configFileFlag: mustWriteFileConfig(t, fileConfigAgent),
			want:           []utils.NetAddr{*proxyAddr},
		},
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
			ccf := &tctlcfg.GlobalCLIFlags{}
			ccf.AuthServerAddr = tt.authServerFlag
			ccf.ConfigFile = tt.configFileFlag

			cfg := &servicecfg.Config{}
			cfg.TeleportHome = tmpHomePath
			// this is needed for the case where the --auth-server=host is not found in profile.
			// ApplyConfig will try to read local auth server identity if the profile is not found.
			cfg.DataDir = authProcess.Config.DataDir

			_, err = tctlcfg.ApplyConfig(ccf, cfg)
			require.NoError(t, err)
			require.NotEmpty(t, cfg.AuthServerAddresses(), "auth servers should be set to a non-empty default if not specified")

			require.ElementsMatch(t, tt.want, cfg.AuthServerAddresses())
		})
	}
}
