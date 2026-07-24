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

package server

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

// installerScriptChecksFor returns the expected shell checks prefix for a given proxy address.
// It mirrors the logic in installerScriptChecks so that tests stay in sync with the implementation.
func installerScriptChecksFor(proxyAddr string) string {
	return `command -v bash > /dev/null 2>&1 || { echo "bash is missing"; exit 100; }` +
		`; command -v sudo > /dev/null 2>&1 || { echo "sudo is missing"; exit 101; }` +
		`; command -v curl > /dev/null 2>&1 || { echo "curl is missing"; exit 102; }` +
		`; df -Pm $(p=/opt/teleport; until [ -e "$p" ]; do p=$(dirname "$p"); done; echo "$p") | awk 'NR==2{exit($4<1250)}' || { echo "insufficient disk space"; exit 103; }` +
		`; curl --silent --max-time 10 --output /dev/null https://` + proxyAddr + `/webapi/find || { echo "proxy is unreachable"; exit 104; }` +
		`; `
}

// windowsInstallerSetup is the fixed PowerShell setup emitted by
// installerScriptWindowsDesktop before the pre-flight checks.
const windowsInstallerSetup = `$ErrorActionPreference = 'Stop'; [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12; `

// windowsInstallerProxyCheck is the PowerShell fragment injected into the proxy
// pre-flight check and the fetch command when an HTTPS proxy applies to the
// proxy endpoint. It mirrors the fragment in installerScriptWindowsDesktop.
const windowsInstallerProxyCheck = `; if ($env:HTTPS_PROXY) { $req.Proxy = $env:HTTPS_PROXY }`

// windowsInstallerChecksFor returns the expected Windows pre-flight checks
// prefix for a given proxy address and proxy-check fragment. It mirrors the
// logic in preFlightInstallerChecksWindows so that tests stay in sync with the
// implementation.
func windowsInstallerChecksFor(proxyAddr, proxyCheck string) string {
	return `$req = @{ Uri = 'https://` + proxyAddr + `/webapi/find'; UseBasicParsing = $true; TimeoutSec = 10 }` + proxyCheck + `; try { Invoke-WebRequest @req | Out-Null } catch { Write-Host "Proxy is unreachable"; exit 104; }` +
		`; if (-not (Get-Command Invoke-WebRequest -ErrorAction SilentlyContinue)) { Write-Host "Invoke-WebRequest is missing"; exit 105; }` +
		`; if (-not ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) { Write-Host "Administrator privileges required"; exit 106; }` +
		`; if (([System.IO.DriveInfo]$env:SystemDrive).AvailableFreeSpace -lt 50MB) { Write-Host "Insufficient disk space"; exit 107; }` +
		`; if ($([System.Environment]::OSVersion.Version.Major) -lt 10) { Write-Host "Unsupported Windows version"; exit 108; }` +
		`; if ((Get-CimInstance -ClassName Win32_ComputerSystem).PartOfDomain) { Write-Host "Machine is joined to a domain"; exit 109; }` +
		`; `
}

// windowsInstallerFetch returns the expected PowerShell that fetches the auth
// package installer script from the proxy.
func windowsInstallerFetch(scriptURL, proxyCheck string) string {
	return `$req = @{ Uri = '` + scriptURL + `'; UseBasicParsing = $true }` + proxyCheck + `; iex (Invoke-WebRequest @req).Content`
}

func TestInstallerScript(t *testing.T) {
	basicParams := func() *types.InstallerParams {
		return &types.InstallerParams{
			PublicProxyAddr: "proxy.example.com:443",
			ScriptName:      "scriptName",
			JoinToken:       "my-token",
		}
	}
	for _, tt := range []struct {
		name                   string
		req                    func() *types.InstallerParams
		withOptions            []scriptOption
		errCheck               require.ErrorAssertionFunc
		expectedScript         string
		expectedScriptContains string
	}{
		{
			name:     "basic",
			req:      basicParams,
			errCheck: require.NoError,
			expectedScript: installerScriptChecksFor("proxy.example.com:443") +
				`bash -c "set -o pipefail; curl --silent --show-error --location https://proxy.example.com:443/v1/webapi/scripts/installer/scriptName | bash -s my-token"`,
		},
		{
			name:                   "with nonce to ensure script is unique",
			req:                    basicParams,
			withOptions:            []scriptOption{withNonceComment()},
			errCheck:               require.NoError,
			expectedScriptContains: `bash -c "set -o pipefail; curl --silent --show-error --location https://proxy.example.com:443/v1/webapi/scripts/installer/scriptName | bash -s my-token" #`,
		},
		{
			name: "with azure clientid",
			req: func() *types.InstallerParams {
				req := basicParams()
				req.Azure = &types.AzureInstallerParams{
					ClientID: "my-id",
				}
				return req
			},
			withOptions:            []scriptOption{withNonceComment()},
			errCheck:               require.NoError,
			expectedScriptContains: `bash -c "set -o pipefail; curl --silent --show-error --location https://proxy.example.com:443/v1/webapi/scripts/installer/scriptName?azure-client-id=my-id | bash -s my-token" #`,
		},
		{
			name: "with suffix installation",
			req: func() *types.InstallerParams {
				req := basicParams()
				req.Suffix = "suffix"
				return req
			},
			errCheck: require.NoError,
			expectedScript: `export TELEPORT_INSTALL_SUFFIX=suffix; ` +
				installerScriptChecksFor("proxy.example.com:443") +
				`bash -c "set -o pipefail; curl --silent --show-error --location https://proxy.example.com:443/v1/webapi/scripts/installer/scriptName | bash -s my-token"`,
		},
		{
			name: "with update group",
			req: func() *types.InstallerParams {
				req := basicParams()
				req.UpdateGroup = "updateGroup"
				return req
			},
			errCheck: require.NoError,
			expectedScript: `export TELEPORT_UPDATE_GROUP=updateGroup; ` +
				installerScriptChecksFor("proxy.example.com:443") +
				`bash -c "set -o pipefail; curl --silent --show-error --location https://proxy.example.com:443/v1/webapi/scripts/installer/scriptName | bash -s my-token"`,
		},
		{
			name: "with install suffix and update group",
			req: func() *types.InstallerParams {
				req := basicParams()
				req.Suffix = "suffix"
				req.UpdateGroup = "updateGroup"
				return req
			},
			errCheck: require.NoError,
			expectedScript: `export TELEPORT_INSTALL_SUFFIX=suffix TELEPORT_UPDATE_GROUP=updateGroup; ` +
				installerScriptChecksFor("proxy.example.com:443") +
				`bash -c "set -o pipefail; curl --silent --show-error --location https://proxy.example.com:443/v1/webapi/scripts/installer/scriptName | bash -s my-token"`,
		},
		{
			name: "missing public proxy address but getter was set up",
			req: func() *types.InstallerParams {
				req := basicParams()
				req.PublicProxyAddr = ""
				return req
			},
			withOptions: []scriptOption{
				withProxyAddrGetter(func(ctx context.Context) (string, error) {
					return "proxy2.example.com", nil
				}),
			},
			errCheck: require.NoError,
			expectedScript: installerScriptChecksFor("proxy2.example.com") +
				`bash -c "set -o pipefail; curl --silent --show-error --location https://proxy2.example.com/v1/webapi/scripts/installer/scriptName | bash -s my-token"`,
		},
		{
			name: "proxy addr is missing but getter returns an error",
			req: func() *types.InstallerParams {
				req := basicParams()
				req.PublicProxyAddr = ""
				return req
			},
			withOptions: []scriptOption{
				withProxyAddrGetter(func(ctx context.Context) (string, error) {
					return "", trace.NotFound("proxy service is not yet running")
				}),
			},
			errCheck: require.Error,
		},
		{
			name: "proxy addr is missing and getter is missing",
			req: func() *types.InstallerParams {
				req := basicParams()
				req.PublicProxyAddr = ""
				return req
			},
			errCheck: require.Error,
		},
		{
			name: "with join token that needs escaping",
			req: func() *types.InstallerParams {
				req := basicParams()
				req.JoinToken = "token$(sh)"
				return req
			},
			errCheck: require.NoError,
			expectedScript: installerScriptChecksFor("proxy.example.com:443") +
				`bash -c "set -o pipefail; curl --silent --show-error --location https://proxy.example.com:443/v1/webapi/scripts/installer/scriptName | bash -s token\$\(sh\)"`,
		},
		{
			name: "with script name that needs escaping",
			req: func() *types.InstallerParams {
				req := basicParams()
				req.ScriptName = "scriptName$(sh)"
				return req
			},
			errCheck: require.NoError,
			expectedScript: installerScriptChecksFor("proxy.example.com:443") +
				`bash -c "set -o pipefail; curl --silent --show-error --location https://proxy.example.com:443/v1/webapi/scripts/installer/scriptName%5C$%5C%28sh%5C%29 | bash -s my-token"`,
		},
		{
			name: "with HTTP Proxy settings set",
			req: func() *types.InstallerParams {
				req := basicParams()
				req.HTTPProxySettings = &types.HTTPProxySettings{
					HTTPSProxy: "http://local-squid:3128",
					HTTPProxy:  "http://local-squid:3128",
					NoProxy:    "http://intranet.local",
				}
				return req
			},
			errCheck: require.NoError,
			expectedScript: `export HTTP_PROXY=http://local-squid:3128 HTTPS_PROXY=http://local-squid:3128 NO_PROXY=http://intranet.local; ` +
				installerScriptChecksFor("proxy.example.com:443") +
				`bash -c "set -o pipefail; curl --silent --show-error --location https://proxy.example.com:443/v1/webapi/scripts/installer/scriptName | bash -s my-token"`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			script, err := installerScript(t.Context(), tt.req(), tt.withOptions...)
			tt.errCheck(t, err)

			if tt.expectedScriptContains != "" {
				require.Contains(t, script, tt.expectedScriptContains)
			}

			if tt.expectedScript != "" {
				require.Equal(t, tt.expectedScript, script)
			}
		})
	}
}

func TestInstallerScriptWindowsDesktop(t *testing.T) {
	basicParams := func() *types.InstallerParams {
		return &types.InstallerParams{
			PublicProxyAddr: "proxy.example.com:443",
			ScriptName:      "scriptName",
		}
	}
	for _, tt := range []struct {
		name                   string
		req                    func() *types.InstallerParams
		withOptions            []scriptOption
		errCheck               require.ErrorAssertionFunc
		expectedScript         string
		expectedScriptContains string
	}{
		{
			name:     "basic",
			req:      basicParams,
			errCheck: require.NoError,
			expectedScript: windowsInstallerSetup +
				windowsInstallerChecksFor("proxy.example.com:443", "") +
				windowsInstallerFetch("https://proxy.example.com:443/v1/webapi/scripts/installer/scriptName", ""),
		},
		{
			name:                   "with nonce to ensure script is unique",
			req:                    basicParams,
			withOptions:            []scriptOption{withNonceComment()},
			errCheck:               require.NoError,
			expectedScriptContains: windowsInstallerFetch("https://proxy.example.com:443/v1/webapi/scripts/installer/scriptName", "") + " #",
		},
		{
			name: "with HTTP proxy settings set",
			req: func() *types.InstallerParams {
				req := basicParams()
				req.HTTPProxySettings = &types.HTTPProxySettings{
					HTTPSProxy: "http://local-squid:3128",
					HTTPProxy:  "http://local-squid:3128",
					NoProxy:    "http://intranet.local",
				}
				return req
			},
			errCheck: require.NoError,
			expectedScript: windowsInstallerSetup +
				`$env:HTTP_PROXY = 'http://local-squid:3128'; $env:HTTPS_PROXY = 'http://local-squid:3128'; $env:NO_PROXY = 'http://intranet.local'; ` +
				windowsInstallerChecksFor("proxy.example.com:443", windowsInstallerProxyCheck) +
				windowsInstallerFetch("https://proxy.example.com:443/v1/webapi/scripts/installer/scriptName", windowsInstallerProxyCheck),
		},
		{
			name: "missing public proxy address but getter was set up",
			req: func() *types.InstallerParams {
				req := basicParams()
				req.PublicProxyAddr = ""
				return req
			},
			withOptions: []scriptOption{
				withProxyAddrGetter(func(ctx context.Context) (string, error) {
					return "proxy2.example.com", nil
				}),
			},
			errCheck: require.NoError,
			expectedScript: windowsInstallerSetup +
				windowsInstallerChecksFor("proxy2.example.com", "") +
				windowsInstallerFetch("https://proxy2.example.com/v1/webapi/scripts/installer/scriptName", ""),
		},
		{
			name: "proxy addr is missing but getter returns an error",
			req: func() *types.InstallerParams {
				req := basicParams()
				req.PublicProxyAddr = ""
				return req
			},
			withOptions: []scriptOption{
				withProxyAddrGetter(func(ctx context.Context) (string, error) {
					return "", trace.NotFound("proxy service is not yet running")
				}),
			},
			errCheck: require.Error,
		},
		{
			name: "proxy addr is missing and getter is missing",
			req: func() *types.InstallerParams {
				req := basicParams()
				req.PublicProxyAddr = ""
				return req
			},
			errCheck: require.Error,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			script, err := installerScriptWindowsDesktop(t.Context(), tt.req(), tt.withOptions...)
			tt.errCheck(t, err)

			if tt.expectedScriptContains != "" {
				require.Contains(t, script, tt.expectedScriptContains)
			}

			if tt.expectedScript != "" {
				require.Equal(t, tt.expectedScript, script)
			}
		})
	}
}
