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
	"net/url"
	"path"
	"strings"

	"github.com/google/safetext/shsprintf"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
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

func installerScript(ctx context.Context, params *types.InstallerParams, opts ...scriptOption) (string, error) {
	scriptOptions := &scriptOptions{}
	for _, opt := range opts {
		scriptOptions = opt(scriptOptions)
	}

	if params == nil {
		return "", trace.BadParameter("installation parameters must not be nil")
	}

	proxyAddr, err := proxyAddress(ctx, params.PublicProxyAddr, scriptOptions.proxyAddr)
	if err != nil {
		return "", trace.Wrap(err)
	}

	scriptURLQuery := url.Values{}
	if params.Azure != nil && params.Azure.ClientID != "" {
		scriptURLQuery.Set("azure-client-id", shsprintf.EscapeDefaultContext(params.Azure.ClientID))
	}

	scriptURL := url.URL{
		Scheme:   "https",
		Host:     proxyAddr,
		Path:     path.Join("v1", "webapi", "scripts", "installer", shsprintf.EscapeDefaultContext(params.ScriptName)),
		RawQuery: scriptURLQuery.Encode(),
	}

	installationScript := fmt.Sprintf(`bash -c "set -o pipefail; curl --silent --show-error --location %s | bash -s %s"`,
		scriptURL.String(),
		shsprintf.EscapeDefaultContext(params.JoinToken),
	)

	envVars := envVarsFromInstallerParams(params)
	if len(envVars) > 0 {
		installationScript = fmt.Sprintf("export %s; %s", strings.Join(envVars, " "), installationScript)
	}

	if scriptOptions.addNonceComment {
		bytes := make([]byte, 8)
		rand.Read(bytes)

		installationScript = installationScript + " # " + hex.EncodeToString(bytes)
	}

	return installationScript, nil
}
