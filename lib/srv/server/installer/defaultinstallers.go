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

package installer

import (
	"fmt"
	"os"
	"strings"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/installers"
	"github.com/gravitational/teleport/lib/web/scripts/oneoff"
)

const (
	scriptShebangAndSetOptions = `#!/usr/bin/env sh
set -eu`
	execGenericInstallScript = `
INSTALL_SCRIPT_URL="https://{{.PublicProxyAddr}}/scripts/install.sh"

echo "Offloading the installation part to the generic Teleport install script hosted at: $INSTALL_SCRIPT_URL"

TEMP_INSTALLER_SCRIPT="$(mktemp)"
curl -sSf "$INSTALL_SCRIPT_URL" -o "$TEMP_INSTALLER_SCRIPT"

chmod +x "$TEMP_INSTALLER_SCRIPT"

sudo -E "$TEMP_INSTALLER_SCRIPT" || (echo "The install script ($TEMP_INSTALLER_SCRIPT) returned a non-zero exit code" && exit 1)
rm "$TEMP_INSTALLER_SCRIPT"`
)

const (
	windowsScriptSetup = `$ErrorActionPreference = 'Stop'
$ProgressPreference    = 'SilentlyContinue'
$ProxyAddr = '{{.PublicProxyAddr}}'`

	windowsGetVersion = `Write-Host "Querying $ProxyAddr for cluster version..."
$find = Invoke-RestMethod -Uri "https://$ProxyAddr/webapi/find" -UseBasicParsing
$Version = "v$($find.server_version)"`

	// TODO(danielashare): Remove --ssl-no-revoke
	windowsInstallScript = `Write-Host "Using Windows Authentication Package version: $Version"

$InstallerName = "teleport-windows-auth-setup-$Version-amd64.exe"
$WorkDir       = Join-Path $env:TEMP 'teleport-wap-install'
New-Item -ItemType Directory -Force -Path $WorkDir | Out-Null

$InstallerPath = Join-Path $WorkDir $InstallerName
$CertPath      = Join-Path $WorkDir 'teleport.cer'

Write-Host "Downloading WAP installer ($InstallerName)..."
& curl.exe -fSLo $InstallerPath "https://cdn.teleport.dev/$InstallerName"
if ($LASTEXITCODE -ne 0) { throw "WAP download failed (curl exit $LASTEXITCODE)" }

Write-Host "Fetching cluster CA from $ProxyAddr..."
& curl.exe --ssl-no-revoke -fSLo $CertPath "https://$ProxyAddr/webapi/auth/export?type=windows"
if ($LASTEXITCODE -ne 0) { throw "CA download failed (curl exit $LASTEXITCODE)" }

Write-Host "Running WAP installer..."
& $InstallerPath install --cert=$CertPath -r
if ($LASTEXITCODE -ne 0) { throw "WAP installer failed (exit $LASTEXITCODE)" }

Write-Host "Teleport WAP installed successfully."`
)

// LegacyDefaultInstaller represents the default installer script provided by teleport.
var (
	// LegacyDefaultInstaller uses oneoff.sh to download the Teleport tarball and run `teleport install`.
	// The Teleport install command handles both Teleport installation and agent configuration.
	LegacyDefaultInstaller = oneoffScriptToDefaultInstaller()

	// NewDefaultInstaller installs Teleport by calling the standard "/scripts/install.sh" route on the proxy.
	// After successfully installing Teleport, it will invoke the same `teleport install`
	// command as the LegacyDefaultInstaller which will only take care of configuring Teleport.
	NewDefaultInstaller = types.MustNewInstallerV1(
		installers.InstallerScriptName,
		strings.Join(
			[]string{scriptShebangAndSetOptions, execGenericInstallScript, configureTeleport},
			"\n\n",
		),
	)
	// Note: if we add namespacing support, we will need to edit this script to use the namespaced full path
	configureTeleport = `
echo "Configuring the Teleport agent"

set +x
TELEPORT_BINARY=/usr/local/bin/teleport
[ -z "${TELEPORT_INSTALL_SUFFIX:-}" ] || TELEPORT_BINARY=/opt/teleport/${TELEPORT_INSTALL_SUFFIX}/bin/teleport

sudo -E "$TELEPORT_BINARY" ` + strings.Join(argsList, " ") + " $@"

	argsList = []string{
		"install", "autodiscover-node",
		"--public-proxy-addr={{.PublicProxyAddr}}",
		"--teleport-package={{.TeleportPackage}}",
		"--repo-channel={{.RepoChannel}}",
		"--auto-upgrade={{.AutomaticUpgrades}}",
		"--azure-client-id={{.AzureClientID}}",
	}

	DefaultWindowsInstaller = types.MustNewInstallerV1(
		installers.InstallerScriptNameWindows,
		buildWindowsInstallScript(),
	)
)

func oneoffScriptToDefaultInstaller() *types.InstallerV1 {
	script, err := oneoff.BuildScript(oneoff.OneOffScriptParams{
		EntrypointArgs:        strings.Join(argsList, " "),
		SuccessMessage:        "Teleport is installed and running.",
		TeleportCommandPrefix: oneoff.PrefixSUDO,
	})
	if err != nil {
		panic(err)
	}

	return types.MustNewInstallerV1(installers.InstallerScriptName, script)
}

func buildWindowsInstallScript() string {
	getVersion := windowsGetVersion
	if v := os.Getenv("TELEPORT_DEBUG_AP_VERSION"); v != "" {
		getVersion = fmt.Sprintf(`$Version = '%s'`, v)
	}

	return strings.Join([]string{windowsScriptSetup, getVersion, windowsInstallScript}, "\n\n")
}
