/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package scripts

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils/teleportassets"
)

func TestGetInstallScript(t *testing.T) {
	ctx := context.Background()
	testVersion := "1.2.3"
	testProxyAddr := "proxy.example.com:443"

	tests := []struct {
		name     string
		opts     InstallScriptOptions
		assertFn func(t *testing.T, script string)
	}{
		{
			name: "Legacy install, no autoupdate",
			opts: InstallScriptOptions{AutoupdateStyle: NoAutoupdate},
			assertFn: func(t *testing.T, script string) {
				require.Equal(t, legacyInstallScript, script)
			},
		},
		{
			name: "Legacy install, package manager autoupdate",
			opts: InstallScriptOptions{AutoupdateStyle: NoAutoupdate},
			assertFn: func(t *testing.T, script string) {
				require.Equal(t, legacyInstallScript, script)
			},
		},
		{
			name: "Oneoff install",
			opts: InstallScriptOptions{
				AutoupdateStyle: UpdaterBinaryAutoupdate,
				TeleportVersion: testVersion,
				ProxyAddr:       testProxyAddr,
				TeleportFlavor:  types.PackageNameOSS,
			},
			assertFn: func(t *testing.T, script string) {
				require.Contains(t, script, "entrypoint='teleport-update'")
				require.Contains(t, script, fmt.Sprintf("teleportVersion='v%s'", testVersion))
				require.Contains(t, script, fmt.Sprintf("teleportFlavor='%s'", types.PackageNameOSS))
				require.Contains(t, script, fmt.Sprintf("cdnBaseURL='%s'", teleportassets.CDNBaseURL()))
				require.Contains(t, script, fmt.Sprintf("entrypointArgs='enable --proxy %s'", testProxyAddr))
				require.Contains(t, script, "packageSuffix='bin.tar.gz'")
			},
		},
		{
			name: "Oneoff install custom CDN",
			opts: InstallScriptOptions{
				AutoupdateStyle: UpdaterBinaryAutoupdate,
				TeleportVersion: testVersion,
				ProxyAddr:       testProxyAddr,
				TeleportFlavor:  types.PackageNameOSS,
				CDNBaseURL:      "https://cdn.example.com",
			},
			assertFn: func(t *testing.T, script string) {
				require.Contains(t, script, "entrypoint='teleport-update'")
				require.Contains(t, script, fmt.Sprintf("teleportVersion='v%s'", testVersion))
				require.Contains(t, script, fmt.Sprintf("teleportFlavor='%s'", types.PackageNameOSS))
				require.Contains(t, script, "cdnBaseURL='https://cdn.example.com'")
				require.Contains(t, script, fmt.Sprintf("entrypointArgs='enable --proxy %s --base-url %s'", testProxyAddr, "https://cdn.example.com"))
				require.Contains(t, script, "packageSuffix='bin.tar.gz'")
			},
		},
		{
			name: "Oneoff install default CDN",
			opts: InstallScriptOptions{
				AutoupdateStyle: UpdaterBinaryAutoupdate,
				TeleportVersion: testVersion,
				ProxyAddr:       testProxyAddr,
				TeleportFlavor:  types.PackageNameOSS,
				CDNBaseURL:      teleportassets.TeleportReleaseCDN,
			},
			assertFn: func(t *testing.T, script string) {
				require.Contains(t, script, "entrypoint='teleport-update'")
				require.Contains(t, script, fmt.Sprintf("teleportVersion='v%s'", testVersion))
				require.Contains(t, script, fmt.Sprintf("teleportFlavor='%s'", types.PackageNameOSS))
				require.Contains(t, script, fmt.Sprintf("cdnBaseURL='%s'", teleportassets.TeleportReleaseCDN))
				require.Contains(t, script, fmt.Sprintf("entrypointArgs='enable --proxy %s'", testProxyAddr))
				require.Contains(t, script, "packageSuffix='bin.tar.gz'")
			},
		},
		{
			name: "Oneoff enterprise install",
			opts: InstallScriptOptions{
				AutoupdateStyle: UpdaterBinaryAutoupdate,
				TeleportVersion: testVersion,
				ProxyAddr:       testProxyAddr,
				TeleportFlavor:  types.PackageNameEnt,
			},
			assertFn: func(t *testing.T, script string) {
				require.Contains(t, script, "entrypoint='teleport-update'")
				require.Contains(t, script, fmt.Sprintf("teleportVersion='v%s'", testVersion))
				require.Contains(t, script, fmt.Sprintf("teleportFlavor='%s'", types.PackageNameEnt))
				require.Contains(t, script, fmt.Sprintf("cdnBaseURL='%s'", teleportassets.CDNBaseURL()))
				require.Contains(t, script, fmt.Sprintf("entrypointArgs='enable --proxy %s'", testProxyAddr))
				require.Contains(t, script, "packageSuffix='bin.tar.gz'")
			},
		},
		{
			name: "Oneoff enterprise FIPS install",
			opts: InstallScriptOptions{
				AutoupdateStyle: UpdaterBinaryAutoupdate,
				TeleportVersion: testVersion,
				ProxyAddr:       testProxyAddr,
				TeleportFlavor:  types.PackageNameEnt,
				FIPS:            true,
			},
			assertFn: func(t *testing.T, script string) {
				require.Contains(t, script, "entrypoint='teleport-update'")
				require.Contains(t, script, fmt.Sprintf("teleportVersion='v%s'", testVersion))
				require.Contains(t, script, fmt.Sprintf("teleportFlavor='%s'", types.PackageNameEnt))
				require.Contains(t, script, fmt.Sprintf("cdnBaseURL='%s'", teleportassets.CDNBaseURL()))
				require.Contains(t, script, fmt.Sprintf("entrypointArgs='enable --proxy %s'", testProxyAddr))
				require.Contains(t, script, "packageSuffix='fips-bin.tar.gz'")
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Sanity check, test input should be legal.
			require.NoError(t, test.opts.Check())

			// Test execution.
			result, err := GetInstallScript(ctx, test.opts)
			require.NoError(t, err)
			test.assertFn(t, result)
		})
	}
}
