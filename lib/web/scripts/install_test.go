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
				require.Contains(t, script, "teleportBin='teleport-update'")
				require.Contains(t, script, fmt.Sprintf("teleportVersion='v%s'", testVersion))
				require.Contains(t, script, fmt.Sprintf("teleportFlavor='%s'", types.PackageNameOSS))
				require.Contains(t, script, fmt.Sprintf("cdnBaseURL='%s'", teleportassets.CDNBaseURL()))
				require.Contains(t, script, fmt.Sprintf("teleportArgs='enable --proxy %q'", testProxyAddr))
				require.Contains(t, script, "teleportFIPSSuffix=''")
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
				require.Contains(t, script, "teleportBin='teleport-update'")
				require.Contains(t, script, fmt.Sprintf("teleportVersion='v%s'", testVersion))
				require.Contains(t, script, fmt.Sprintf("teleportFlavor='%s'", types.PackageNameOSS))
				require.Contains(t, script, "cdnBaseURL='https://cdn.example.com'")
				require.Contains(t, script, fmt.Sprintf("teleportArgs='enable --proxy %q --base-url %q'", testProxyAddr, "https://cdn.example.com"))
				require.Contains(t, script, "teleportFIPSSuffix=''")
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
				require.Contains(t, script, "teleportBin='teleport-update'")
				require.Contains(t, script, fmt.Sprintf("teleportVersion='v%s'", testVersion))
				require.Contains(t, script, fmt.Sprintf("teleportFlavor='%s'", types.PackageNameEnt))
				require.Contains(t, script, fmt.Sprintf("cdnBaseURL='%s'", teleportassets.CDNBaseURL()))
				require.Contains(t, script, fmt.Sprintf("teleportArgs='enable --proxy %q'", testProxyAddr))
				require.Contains(t, script, "teleportFIPSSuffix=''")
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
				require.Contains(t, script, "teleportBin='teleport-update'")
				require.Contains(t, script, fmt.Sprintf("teleportVersion='v%s'", testVersion))
				require.Contains(t, script, fmt.Sprintf("teleportFlavor='%s'", types.PackageNameEnt))
				require.Contains(t, script, fmt.Sprintf("cdnBaseURL='%s'", teleportassets.CDNBaseURL()))
				require.Contains(t, script, fmt.Sprintf("teleportArgs='enable --proxy %q'", testProxyAddr))
				require.Contains(t, script, "teleportFIPSSuffix='fips-'")
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
