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

package web

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/utils/teleportassets"
	"github.com/gravitational/teleport/lib/web/scripts"
)

const insecureParamName = "insecure"

// installScriptHandle handles calls for "/scripts/install.sh" and responds with a bash script installing Teleport
// by downloading and running `teleport-update`. This installation script does not start the agent, join it,
// or configure its services. This is handled by the "/scripts/:token/install-*.sh" scripts.
func (h *Handler) installScriptHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) (any, error) {
	// This is a hack because the router is not allowing us to register "/scripts/install.sh", so we use
	// the parameter ":token" to match the script name.
	// Currently, only "install.sh" is supported.
	if params.ByName("token") != "install.sh" {
		return nil, trace.NotFound(`Route not found, query "/scripts/install.sh" for the install-only script, or "/scripts/:token/install-node.sh" for the install + join script.`)
	}

	// TODO(hugoShaka): cache function
	opts, err := h.installScriptOptions(r.Context())
	if err != nil {
		return nil, trace.Wrap(err, "Failed to build install script options")
	}

	if insecure := r.URL.Query().Get(insecureParamName); insecure != "" {
		v, err := strconv.ParseBool(insecure)
		if err != nil {
			return nil, trace.BadParameter("failed to parse insecure flag %q: %v", insecure, err)
		}
		opts.Insecure = v
	}

	script, err := scripts.GetInstallScript(r.Context(), opts)
	if err != nil {
		h.logger.WarnContext(r.Context(), "Failed to get install script", "error", err)
		return nil, trace.Wrap(err, "getting script")
	}

	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprintln(w, script); err != nil {
		h.logger.WarnContext(r.Context(), "Failed to write install script", "error", err)
	}

	return nil, nil
}

// installScriptOptions computes the agent installation options based on the proxy configuration and the cluster status.
// This includes:
// - the type of automatic updates
// - the desired version
// - the proxy address (used for updates).
// - the Teleport artifact name and CDN
func (h *Handler) installScriptOptions(ctx context.Context) (scripts.InstallScriptOptions, error) {
	const defaultGroup, defaultUpdater = "", ""

	version, err := h.autoUpdateAgentVersion(ctx, defaultGroup, defaultUpdater)
	if err != nil {
		h.logger.WarnContext(ctx, "Failed to get intended agent version", "error", err)
		version = teleport.Version
	}

	// if there's a rollout, we do new autoupdates
	_, rolloutErr := h.cfg.AccessPoint.GetAutoUpdateAgentRollout(ctx)
	if rolloutErr != nil && !(trace.IsNotFound(rolloutErr) || trace.IsNotImplemented(rolloutErr)) {
		h.logger.WarnContext(ctx, "Failed to get rollout", "error", rolloutErr)
		return scripts.InstallScriptOptions{}, trace.Wrap(err, "failed to check the autoupdate agent rollout state")
	}

	var autoupdateStyle scripts.AutoupdateStyle
	switch {
	case rolloutErr == nil:
		autoupdateStyle = scripts.UpdaterBinaryAutoupdate
	case automaticUpgrades(h.clusterFeatures):
		autoupdateStyle = scripts.PackageManagerAutoupdate
	default:
		autoupdateStyle = scripts.NoAutoupdate
	}

	var teleportFlavor string
	switch modules.GetModules().BuildType() {
	case modules.BuildEnterprise:
		teleportFlavor = types.PackageNameEnt
	case modules.BuildOSS, modules.BuildCommunity:
		teleportFlavor = types.PackageNameOSS
	default:
		h.logger.WarnContext(ctx, "Unknown built type, defaulting to the 'teleport' package.", "type", modules.GetModules().BuildType())
		teleportFlavor = types.PackageNameOSS
	}

	cdnBaseURL, err := getCDNBaseURL(version)
	if err != nil {
		h.logger.WarnContext(ctx, "Failed to get CDN base URL", "error", err)
		return scripts.InstallScriptOptions{}, trace.Wrap(err)
	}

	return scripts.InstallScriptOptions{
		AutoupdateStyle: autoupdateStyle,
		TeleportVersion: version,
		CDNBaseURL:      cdnBaseURL,
		ProxyAddr:       h.PublicProxyAddr(),
		TeleportFlavor:  teleportFlavor,
		FIPS:            modules.IsBoringBinary(),
	}, nil

}

// EnvVarCDNBaseURL is the environment variable that allows users to override the Teleport base CDN url used in the installation script.
// Setting this value is required for testing (make production builds install from the dev CDN, and vice versa).
// As we (the Teleport company) don't distribute AGPL binaries, this must be set when using a Teleport OSS build.
// Example values:
// - "https://cdn.teleport.dev" (prod)
// - "https://cdn.cloud.gravitational.io" (dev builds/staging)
const EnvVarCDNBaseURL = "TELEPORT_CDN_BASE_URL"

func getCDNBaseURL(version string) (string, error) {
	// If the user explicitly overrides the CDN base URL, we use it.
	if override := os.Getenv(EnvVarCDNBaseURL); override != "" {
		return override, nil
	}

	v, err := semver.NewVersion(version)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// For backward compatibility we don't fail if the user is running AGPL and
	// did not specify the CDN URL. However we will fail in v18 for this as we
	// cannot automatically install binaries subject to a license the user has
	// not agreed to.

	return teleportassets.CDNBaseURLForVersion(v), nil
}
