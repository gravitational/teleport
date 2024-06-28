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

package integration

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth/storage"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// TestVersionCheck tests version upgrade cases of teleport.
func TestVersionCheck(t *testing.T) {
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	var err error

	authCfg := servicecfg.MakeDefaultConfig()
	authCfg.Version = defaults.TeleportConfigVersionV3
	authCfg.SetAuthServerAddress(utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"})
	authCfg.Auth.Enabled = true
	authCfg.Auth.ClusterName, err = services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "auth-server",
	})
	require.NoError(t, err)
	authCfg.Auth.ListenAddr.Addr = "localhost:0"
	authCfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)

	authCfg.Proxy.Enabled = false
	authCfg.SSH.Enabled = false

	launchFunc := func(ctx context.Context, authCfg servicecfg.Config, authRunErrCh chan error, signal chan struct{}) {
		authRunErrCh <- service.Run(ctx, authCfg, func(cfg *servicecfg.Config) (service.Process, error) {
			proc, err := service.NewTeleport(cfg)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			if _, err = proc.GetIdentity(types.RoleAdmin); err != nil {
				proc.Close()
				return nil, trace.Wrap(err)
			}
			signal <- struct{}{}

			return proc, nil
		})
	}

	teleportVersion, err := semver.NewVersion(teleport.Version)
	require.NoError(t, err)

	tests := map[string]struct {
		initialVersion  string
		expectedVersion string
		expectError     bool
	}{
		"first-launch": {
			initialVersion:  "",
			expectedVersion: teleport.Version,
			expectError:     false,
		},
		"old-version-upgrade": {
			initialVersion:  fmt.Sprintf("%d.0.0", teleportVersion.Major-1),
			expectedVersion: teleport.Version,
			expectError:     false,
		},
		"major-upgrade-fail": {
			initialVersion:  fmt.Sprintf("%d.0.0", teleportVersion.Major-2),
			expectedVersion: fmt.Sprintf("%d.0.0", teleportVersion.Major-2),
			expectError:     true,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// Create temporary directory for the auth service.
			authDir := t.TempDir()
			authCfg.DataDir = authDir
			authCfg.Auth.StorageConfig.Params = backend.Params{defaults.BackendPath: filepath.Join(authDir, defaults.BackendDir)}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			processStorage, err := storage.NewProcessStorage(ctx, filepath.Join(authDir, teleport.ComponentProcess))
			require.NoError(t, err)
			if test.initialVersion != "" {
				err := processStorage.WriteTeleportVersion(test.initialVersion)
				require.NoError(t, err)
			}

			authRunErrCh := make(chan error, 1)
			signal := make(chan struct{})
			go launchFunc(ctx, *authCfg, authRunErrCh, signal)

			timeout := time.After(time.Second * 30)
			select {
			case err := <-authRunErrCh:
				if test.expectError {
					assert.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			case <-timeout:
				t.Fatal("timed out waiting for fetching identity")
			case <-signal:
			}

			localVersion, err := processStorage.GetTeleportVersion(ctx)
			require.NoError(t, err)
			assert.Equal(t, test.expectedVersion, localVersion)
		})
	}
}
