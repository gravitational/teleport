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
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"maps"
	"net/url"
	"path"
	"slices"
	"strings"

	"github.com/google/safetext/shsprintf"
	"github.com/gravitational/trace"
	"golang.org/x/net/http/httpproxy"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv/server/installstatus"
)

// envVarsFromInstallerParams converts InstallerParams into a list of environment variables, in the form of KEY=value.
// The output can be used as is without further escaping in scripts.
func envVarsFromInstallerParams(params *types.InstallerParams) []string {
	var envVars []string

	// InstallSuffix and UpdateGroup only contains alphanumeric characters and hyphens.
	// Escape them anyway as another layer of safety.
	if params.Suffix != "" {
		safeInstallSuffix := shsprintf.EscapeDefaultContext(params.Suffix)
		envVars = append(envVars, "TELEPORT_INSTALL_SUFFIX="+safeInstallSuffix)
	}
	if params.UpdateGroup != "" {
		safeUpdateGroup := shsprintf.EscapeDefaultContext(params.UpdateGroup)
		envVars = append(envVars, "TELEPORT_UPDATE_GROUP="+safeUpdateGroup)
	}

	if params.HTTPProxySettings != nil {
		if params.HTTPProxySettings.HTTPProxy != "" {
			safeHTTPProxy := shsprintf.EscapeDefaultContext(params.HTTPProxySettings.HTTPProxy)
			envVars = append(envVars, "HTTP_PROXY="+safeHTTPProxy)
		}
		if params.HTTPProxySettings.HTTPSProxy != "" {
			safeHTTPSProxy := shsprintf.EscapeDefaultContext(params.HTTPProxySettings.HTTPSProxy)
			envVars = append(envVars, "HTTPS_PROXY="+safeHTTPSProxy)
		}
		if params.HTTPProxySettings.NoProxy != "" {
			safeNoProxy := shsprintf.EscapeDefaultContext(params.HTTPProxySettings.NoProxy)
			envVars = append(envVars, "NO_PROXY="+safeNoProxy)
		}
	}

	return envVars
}

// escapePowerShellSingleQuoted wraps s in a PowerShell single-quoted string
// literal. Inside single quotes PowerShell treats every character literally
// except a single quote, which is escaped by doubling it. This prevents
// interpolation and command injection regardless of the input.
func escapePowerShellSingleQuoted(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// envVarsFromInstallerParamsWindows converts InstallerParams into a list of
// PowerShell statements that set environment variables for the current process,
// in the form $env:KEY = 'value'. Child processes (i.e. the downloaded
// installer) inherit these. The output can be used as is without further
// escaping in scripts.
func envVarsFromInstallerParamsWindows(params *types.InstallerParams) []string {
	var envVars []string

	setEnv := func(key, value string) {
		envVars = append(envVars, fmt.Sprintf("$env:%s = %s", key, escapePowerShellSingleQuoted(value)))
	}

	if params.HTTPProxySettings != nil {
		if params.HTTPProxySettings.HTTPProxy != "" {
			setEnv("HTTP_PROXY", params.HTTPProxySettings.HTTPProxy)
		}
		if params.HTTPProxySettings.HTTPSProxy != "" {
			setEnv("HTTPS_PROXY", params.HTTPProxySettings.HTTPSProxy)
		}
		if params.HTTPProxySettings.NoProxy != "" {
			setEnv("NO_PROXY", params.HTTPProxySettings.NoProxy)
		}
	}

	return envVars
}

type scriptOption func(*scriptOptions) *scriptOptions
type scriptOptions struct {
	addNonceComment bool
	proxyAddr       proxyGetter
}

// Azure treats scripts with the same content as the same invocation and
// won't run them more than once. This is fine when the installer script
// succeeds, but it makes troubleshooting much harder when it fails. To
// work around this, we generate a random string and append it as a comment
// to the script, forcing Azure to see each invocation as unique.
func withNonceComment() scriptOption {
	return func(opts *scriptOptions) *scriptOptions {
		opts.addNonceComment = true
		return opts
	}
}

type proxyGetter func(context.Context) (string, error)

func withProxyAddrGetter(f proxyGetter) scriptOption {
	return func(opts *scriptOptions) *scriptOptions {
		opts.proxyAddr = f
		return opts
	}
}

func proxyAddress(ctx context.Context, proxyAddr string, getter proxyGetter) (string, error) {
	if proxyAddr != "" {
		return proxyAddr, nil
	}

	if getter != nil {
		return getter(ctx)
	}

	return "", trace.BadParameter("proxy address is missing from the matcher and there is no available Proxy Service yet")
}

// resolveInstallerScript parses options, validates params, resolves the proxy
// address, and builds the installer script URL. These steps are shared by all
// platform-specific installer script builders.
func resolveInstallerScript(ctx context.Context, params *types.InstallerParams, opts ...scriptOption) (proxyAddr, scriptURL string, o *scriptOptions, err error) {
	scriptOptions := &scriptOptions{}
	for _, opt := range opts {
		scriptOptions = opt(scriptOptions)
	}

	if params == nil {
		return "", "", nil, trace.BadParameter("installation parameters must not be nil")
	}

	proxyAddr, err = proxyAddress(ctx, params.PublicProxyAddr, scriptOptions.proxyAddr)
	if err != nil {
		return "", "", nil, trace.Wrap(err)
	}

	scriptURLQuery := url.Values{}
	if params.Azure != nil && params.Azure.ClientID != "" {
		scriptURLQuery.Set("azure-client-id", shsprintf.EscapeDefaultContext(params.Azure.ClientID))
	}

	u := url.URL{
		Scheme:   "https",
		Host:     proxyAddr,
		Path:     path.Join("v1", "webapi", "scripts", "installer", shsprintf.EscapeDefaultContext(params.ScriptName)),
		RawQuery: scriptURLQuery.Encode(),
	}

	return proxyAddr, u.String(), scriptOptions, nil
}

func installerScript(ctx context.Context, params *types.InstallerParams, opts ...scriptOption) (string, error) {
	proxyAddr, scriptURL, scriptOptions, err := resolveInstallerScript(ctx, params, opts...)
	if err != nil {
		return "", trace.Wrap(err)
	}

	var installationScript string

	// Export env vars before pre flight checks so that proxy network check can use http proxy settings if they are provided in the installer params.
	envVars := envVarsFromInstallerParams(params)
	if len(envVars) > 0 {
		installationScript += fmt.Sprintf("export %s; ", strings.Join(envVars, " "))
	}

	installationScript += preFlightChecksScript(preFlightInstallerChecks(proxyAddr))

	installationScript += fmt.Sprintf(`bash -c "set -o pipefail; curl --silent --show-error --location %s | bash -s %s"`,
		scriptURL,
		shsprintf.EscapeDefaultContext(params.JoinToken),
	)

	if scriptOptions.addNonceComment {
		bytes := make([]byte, 8)
		rand.Read(bytes)

		installationScript += " # " + hex.EncodeToString(bytes)
	}

	return installationScript, nil
}

// preFlightChecksScript returns a shell script fragment that performs pre-installation checks.
// Each check exits with a specific non-zero code so the Discovery Service can identify the failure.
func preFlightChecksScript(checks map[installstatus.ExitCode]string) string {
	exitCodes := slices.Collect(maps.Keys(checks))
	slices.Sort(exitCodes)

	var checkScriptFragments []string
	for _, exitCode := range exitCodes {
		checkScriptFragments = append(checkScriptFragments, checks[exitCode])
	}

	return strings.Join(checkScriptFragments, "; ") + "; "
}

func preFlightInstallerChecks(proxyAddr string) map[installstatus.ExitCode]string {
	proxyFindURL := url.URL{
		Scheme: "https",
		Host:   proxyAddr,
		Path:   path.Join("webapi", "find"),
	}

	orExitWithMessageScriptSnippet := func(exitCode installstatus.ExitCode, message string) string {
		return fmt.Sprintf(`|| { echo "%s"; exit %d; }`, message, exitCode)
	}

	return map[installstatus.ExitCode]string{
		// Basic command checks for bash, sudo and curl.
		installstatus.BashNotFound: fmt.Sprintf(`command -v bash > /dev/null 2>&1 %s`, orExitWithMessageScriptSnippet(installstatus.BashNotFound, "bash is missing")),
		installstatus.SudoNotFound: fmt.Sprintf(`command -v sudo > /dev/null 2>&1 %s`, orExitWithMessageScriptSnippet(installstatus.SudoNotFound, "sudo is missing")),
		installstatus.CurlNotFound: fmt.Sprintf(`command -v curl > /dev/null 2>&1 %s`, orExitWithMessageScriptSnippet(installstatus.CurlNotFound, "curl is missing")),

		// check if there's enough disk space for the installation
		// df -Pm outputs disk usage in megabytes; awk selects the data row (NR==2) and
		// exits non-zero if the available column ($4) is below the required threshold.
		// It tries to check /opt/teleport first if it exists, then /opt, and finally / as a last resort.
		installstatus.InsufficientDiskSpace: fmt.Sprintf(`df -Pm $(p=/opt/teleport; until [ -e "$p" ]; do p=$(dirname "$p"); done; echo "$p") | awk 'NR==2{exit($4<%d)}' %s`,
			installstatus.InstallerMinFreeDiskMB,
			orExitWithMessageScriptSnippet(installstatus.InsufficientDiskSpace, "insufficient disk space"),
		),

		// check if network connection to the proxy is available
		installstatus.ProxyPingError: fmt.Sprintf(`curl --silent --max-time 10 --output /dev/null %s %s`,
			shsprintf.EscapeDefaultContext(proxyFindURL.String()),
			orExitWithMessageScriptSnippet(installstatus.ProxyPingError, "proxy is unreachable"),
		),
	}
}

func installerScriptWindowsDesktop(ctx context.Context, params *types.InstallerParams, opts ...scriptOption) (string, error) {
	proxyAddr, scriptURL, scriptOptions, err := resolveInstallerScript(ctx, params, opts...)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// curl.exe (which reads HTTP(S)_PROXY / NO_PROXY natively) is absent on
	// Windows before version 1803 and on Windows Server 2016, both of which we
	// support, so the scripts use Invoke-WebRequest. Invoke-WebRequest doesn't
	// read those env vars, so compute here whether a proxy applies to the proxy
	// endpoint. proxyCheck is a PowerShell fragment reused for the pre-flight
	// proxy check and the install script below (both target the proxy over
	// HTTPS). It reads the value from $env:HTTPS_PROXY at runtime and is empty
	// when no proxy applies.
	var proxyCheck string
	if params.HTTPProxySettings != nil {
		cfg := httpproxy.Config{
			HTTPProxy:  params.HTTPProxySettings.HTTPProxy,
			HTTPSProxy: params.HTTPProxySettings.HTTPSProxy,
			NoProxy:    params.HTTPProxySettings.NoProxy,
		}
		target := &url.URL{Scheme: "https", Host: proxyAddr}
		if proxyURL, err := cfg.ProxyFunc()(target); err == nil && proxyURL != nil {
			proxyCheck = `; if ($env:HTTPS_PROXY) { $req.Proxy = $env:HTTPS_PROXY }`
		}
	}

	var installationScript string

	// Abort on any error and enable TLS 1.2 before any HTTPS request. Windows
	// PowerShell 5.1 does not always negotiate it by default. This must precede
	// the pre-flight proxy check, which also makes an HTTPS request.
	installationScript += `$ErrorActionPreference = 'Stop'; [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12; `

	// Export env vars before pre-flight checks so that proxy network check can
	// use http proxy settings if they are provided in the installer params.
	envVars := envVarsFromInstallerParamsWindows(params)
	if len(envVars) > 0 {
		installationScript += strings.Join(envVars, "; ") + "; "
	}

	installationScript += preFlightChecksScript(preFlightInstallerChecksWindows(proxyAddr, proxyCheck))

	// Fetch the installer script from the proxy and run it in the current session
	// using iex so it inherits the env vars set above. Set UseBasicParsing to
	// avoid using IE engine for parsing.
	installationScript += fmt.Sprintf(`$req = @{ Uri = %s; UseBasicParsing = $true }%s; iex (Invoke-WebRequest @req).Content`,
		escapePowerShellSingleQuoted(scriptURL),
		proxyCheck,
	)

	if scriptOptions.addNonceComment {
		bytes := make([]byte, 8)
		rand.Read(bytes)

		installationScript += " # " + hex.EncodeToString(bytes)
	}

	return installationScript, nil
}

// preFlightInstallerChecksWindows returns the Windows pre-flight checks.
// proxyCheck is a PowerShell fragment (computed by the caller) that routes the
// proxy check through HTTPS_PROXY when applicable. It is empty when no proxy
// applies.
func preFlightInstallerChecksWindows(proxyAddr, proxyCheck string) map[installstatus.ExitCode]string {
	proxyFindURL := url.URL{
		Scheme: "https",
		Host:   proxyAddr,
		Path:   path.Join("webapi", "find"),
	}

	exitWithMessage := func(exitCode installstatus.ExitCode, message string) string {
		return fmt.Sprintf(`{ Write-Host "%s"; exit %d; }`, message, exitCode)
	}

	return map[installstatus.ExitCode]string{
		// Check for Invoke-WebRequest
		installstatus.InvokeWebRequestNotFound: fmt.Sprintf(`if (-not (Get-Command Invoke-WebRequest -ErrorAction SilentlyContinue)) %s`,
			exitWithMessage(installstatus.InvokeWebRequestNotFound, "Invoke-WebRequest is missing")),

		// Check that we have the correct permissions
		installstatus.AdministratorPrivilegesRequired: fmt.Sprintf(`if (-not ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) %s`,
			exitWithMessage(installstatus.AdministratorPrivilegesRequired, "Administrator privileges required")),

		// Check if there's enough disk space on the system drive for the installation
		installstatus.WindowsInsufficientDiskSpace: fmt.Sprintf(`if (([System.IO.DriveInfo]$env:SystemDrive).AvailableFreeSpace -lt %dMB) %s`,
			installstatus.WindowsDesktopInstallerMinFreeDiskMB,
			exitWithMessage(installstatus.WindowsInsufficientDiskSpace, "Insufficient disk space")),

		// Check if network connection to the proxy is available. Route through
		// HTTPS_PROXY when it applies and isn't excluded by NO_PROXY.
		installstatus.ProxyPingError: fmt.Sprintf(`$req = @{ Uri = %s; UseBasicParsing = $true; TimeoutSec = 10 }%s; try { Invoke-WebRequest @req | Out-Null } catch %s`,
			escapePowerShellSingleQuoted(proxyFindURL.String()),
			proxyCheck,
			exitWithMessage(installstatus.ProxyPingError, "Proxy is unreachable")),

		// Check if the system is running a supported version of Windows (Windows Server 2016 or later, Windows 10 or later)
		// See https://learn.microsoft.com/en-us/windows/win32/sysinfo/operating-system-version
		installstatus.UnsupportedWindowsVersion: fmt.Sprintf(`if ($([System.Environment]::OSVersion.Version.Major) -lt 10) %s`,
			exitWithMessage(installstatus.UnsupportedWindowsVersion, "Unsupported Windows version")),

		// Check that the machine is not joined to an Active Directory domain. The
		// authentication package is only supported on non-domain-joined hosts.
		installstatus.WindowsMachineDomainJoined: fmt.Sprintf(`if ((Get-CimInstance -ClassName Win32_ComputerSystem).PartOfDomain) %s`,
			exitWithMessage(installstatus.WindowsMachineDomainJoined, "Machine is joined to a domain")),
	}
}
