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
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

func TestMain(m *testing.M) {
	modules.SetInsecureTestMode(true)
	os.Exit(m.Run())
}

// TestConnect tests client config and connection logic.
func TestConnect(t *testing.T) {
	dynAddr := helpers.NewDynamicServiceAddr(t)
	ctx := context.Background()

	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.AuthAddr,
			},
		},
	}
	makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.Descriptors))

	username := "admin"
	mustAddUser(t, fileConfig, "admin", "access")

	for _, tc := range []struct {
		name         string
		cliFlags     GlobalCLIFlags
		modifyConfig func(*servicecfg.Config)
	}{
		{
			name: "default to data dir",
			cliFlags: GlobalCLIFlags{
				AuthServerAddr: []string{fileConfig.Auth.ListenAddress},
				Insecure:       true,
			},
			modifyConfig: func(cfg *servicecfg.Config) {
				cfg.DataDir = fileConfig.DataDir
			},
		}, {
			name: "config file",
			cliFlags: GlobalCLIFlags{
				ConfigFile: mustWriteFileConfig(t, fileConfig),
				Insecure:   true,
			},
		}, {
			name: "config file string",
			cliFlags: GlobalCLIFlags{
				ConfigString: mustGetBase64EncFileConfig(t, fileConfig),
				Insecure:     true,
			},
		}, {
			name: "identity file",
			cliFlags: GlobalCLIFlags{
				AuthServerAddr:   []string{fileConfig.Auth.ListenAddress},
				IdentityFilePath: mustWriteIdentityFile(t, fileConfig, username),
				Insecure:         true,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := servicecfg.MakeDefaultConfig()
			cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()
			if tc.modifyConfig != nil {
				tc.modifyConfig(cfg)
			}

			clientConfig, err := ApplyConfig(&tc.cliFlags, cfg)
			require.NoError(t, err)

			_, err = authclient.Connect(ctx, clientConfig)
			require.NoError(t, err)
		})
	}
}
