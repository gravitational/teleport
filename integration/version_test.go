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

// TestVersionCheckFirstLaunch tests initial version set to process database.
func TestVersionCheckFirstLaunch(t *testing.T) {
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create temporary directory for the auth service.
	authDir := t.TempDir()

	processStorage, err := storage.NewProcessStorage(ctx, filepath.Join(authDir, teleport.ComponentProcess))
	require.NoError(t, err)

	authAddr, err := getFreeListenAddr()
	require.NoError(t, err)

	authCfg := servicecfg.MakeDefaultConfig()
	authCfg.Version = defaults.TeleportConfigVersionV3
	authCfg.DataDir = authDir
	require.NoError(t, authCfg.SetAuthServerAddresses([]utils.NetAddr{
		{
			AddrNetwork: "tcp",
			Addr:        authAddr,
		},
	}))
	authCfg.Auth.Enabled = true
	authCfg.Auth.StorageConfig.Params = backend.Params{defaults.BackendPath: filepath.Join(authDir, defaults.BackendDir)}
	authCfg.Auth.ClusterName, err = services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "auth-server-first-launch",
	})
	require.NoError(t, err)
	authCfg.Auth.ListenAddr.Addr = authAddr
	authCfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)

	authCfg.Proxy.Enabled = false
	authCfg.SSH.Enabled = false

	authRunErrCh := make(chan error, 1)
	signal := make(chan struct{})
	go func() {
		authRunErrCh <- service.Run(ctx, *authCfg, func(cfg *servicecfg.Config) (service.Process, error) {
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
	}()

	timeout := time.After(time.Second * 30)
	select {
	case err := <-authRunErrCh:
		require.NoError(t, err)
	case <-timeout:
		t.Fatal("timed out waiting for fetching identity")
	case <-signal:
		localVersion, err := processStorage.GetTeleportVersion(ctx)
		require.NoError(t, err)

		assert.Equal(t, teleport.Version, localVersion)
	}
}

// TestVersionCheckUpgradeOneMajorVersion tests normal flow of major version upgrade,
// should verify that after launch local version is upgraded as well.
func TestVersionCheckUpgradeOneMajorVersion(t *testing.T) {
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create temporary directory for the auth service.
	authDir := t.TempDir()

	processStorage, err := storage.NewProcessStorage(ctx, filepath.Join(authDir, teleport.ComponentProcess))
	require.NoError(t, err)

	version := semver.New(teleport.Version)
	err = processStorage.WriteTeleportVersion(fmt.Sprintf("%d.0.0", version.Major-1))
	require.NoError(t, err)

	authAddr, err := getFreeListenAddr()
	require.NoError(t, err)

	authCfg := servicecfg.MakeDefaultConfig()
	authCfg.Version = defaults.TeleportConfigVersionV3
	authCfg.DataDir = authDir
	require.NoError(t, authCfg.SetAuthServerAddresses([]utils.NetAddr{
		{
			AddrNetwork: "tcp",
			Addr:        authAddr,
		},
	}))
	authCfg.Auth.Enabled = true
	authCfg.Auth.StorageConfig.Params = backend.Params{defaults.BackendPath: filepath.Join(authDir, defaults.BackendDir)}
	authCfg.Auth.ClusterName, err = services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "auth-server",
	})
	require.NoError(t, err)
	authCfg.Auth.ListenAddr.Addr = authAddr
	authCfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)

	authCfg.Proxy.Enabled = false
	authCfg.SSH.Enabled = false

	authRunErrCh := make(chan error, 1)
	signal := make(chan struct{})
	go func() {
		authRunErrCh <- service.Run(ctx, *authCfg, func(cfg *servicecfg.Config) (service.Process, error) {
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
	}()

	timeout := time.After(time.Second * 30)
	select {
	case err := <-authRunErrCh:
		require.NoError(t, err)
	case <-timeout:
		t.Fatal("timed out waiting for fetching identity")
	case <-signal:
		localVersion, err := processStorage.GetTeleportVersion(ctx)
		require.NoError(t, err)

		assert.Equal(t, teleport.Version, localVersion)
	}
}

// TestVersionCheckUpgradeFailed tests version upgrade restriction with incompatible major upgrade.
func TestVersionCheckUpgradeFailed(t *testing.T) {
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create temporary directory for the auth service.
	authDir := t.TempDir()

	processStorage, err := storage.NewProcessStorage(ctx, filepath.Join(authDir, teleport.ComponentProcess))
	require.NoError(t, err)

	version := semver.New(teleport.Version)
	initialLocalVersion := fmt.Sprintf("%d.0.0", version.Major-2)
	err = processStorage.WriteTeleportVersion(initialLocalVersion)
	require.NoError(t, err)

	authAddr, err := getFreeListenAddr()
	require.NoError(t, err)

	authCfg := servicecfg.MakeDefaultConfig()
	authCfg.Version = defaults.TeleportConfigVersionV3
	authCfg.DataDir = authDir
	require.NoError(t, authCfg.SetAuthServerAddresses([]utils.NetAddr{
		{
			AddrNetwork: "tcp",
			Addr:        authAddr,
		},
	}))
	authCfg.Auth.Enabled = true
	authCfg.Auth.StorageConfig.Params = backend.Params{defaults.BackendPath: filepath.Join(authDir, defaults.BackendDir)}
	authCfg.Auth.ClusterName, err = services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "auth-server",
	})
	require.NoError(t, err)
	authCfg.Auth.ListenAddr.Addr = authAddr
	authCfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)

	authCfg.Proxy.Enabled = false
	authCfg.SSH.Enabled = false

	authRunErrCh := make(chan error, 1)
	go func() {
		authRunErrCh <- service.Run(ctx, *authCfg, func(cfg *servicecfg.Config) (service.Process, error) {
			proc, err := service.NewTeleport(cfg)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return proc, nil
		})
	}()

	timeout := time.After(time.Second * 30)
	select {
	case <-timeout:
		t.Fatal("timed out waiting to receive error")
	case versionErr := <-authRunErrCh:
		localVersion, err := processStorage.GetTeleportVersion(ctx)
		require.NoError(t, err)

		assert.Error(t, versionErr)
		assert.Equal(t, initialLocalVersion, localVersion)
	}
}
