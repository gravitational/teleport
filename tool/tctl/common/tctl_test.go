// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

// TestConnect tests client config and connection logic.
func TestConnect(t *testing.T) {
	dynAddr := newDynamicServiceAddr(t)
	ctx := context.Background()

	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.authAddr,
			},
		},
	}
	makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.descriptors))

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
