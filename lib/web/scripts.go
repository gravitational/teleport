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

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/utils/teleportassets"
	"github.com/gravitational/teleport/lib/web/scripts"
)

// installScriptHandle handles calls for "/script/install.sh" and responds with a bash script installing Teleport
// by downloading and running `teleport-update`. This installation script does not start the agent, join it,
// or configure its services. This is handled by the "/scripts/:token/install-*.sh" scripts.
func (h *Handler) installScriptHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) (any, error) {
	// TODO(hugoShaka): cache function
	opts, err := h.installScriptOptions(r.Context())
	if err != nil {
		return nil, trace.Wrap(err, "Failed to build install script options")
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
	const group, uuid = "", ""

	version, err := h.autoUpdateAgentVersion(ctx, group, uuid)
	if err != nil {
		h.logger.WarnContext(ctx, "Failed to update agent version", "error", err)
		version = teleport.Version
	}

	// if there's a rollout, we do new autoupdates
	_, rolloutErr := h.cfg.AccessPoint.GetAutoUpdateAgentRollout(ctx)
	if rolloutErr != nil && !trace.IsNotFound(rolloutErr) {
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

	cdnBaseURL, err := getCDNBaseURL()
	if err != nil {
		h.logger.WarnContext(ctx, "Failed to get CDN base URL", "error", err)
		return scripts.InstallScriptOptions{}, trace.Wrap(err)
	}

	return scripts.InstallScriptOptions{
		AutoupdateStyle: autoupdateStyle,
		TeleportVersion: version,
		// Note: this override is required to configure the license on AGPL builds.
		// We cannot install random binaries if the user doesn't
		CDNBaseURL:     cdnBaseURL,
		ProxyAddr:      h.PublicProxyAddr(),
		TeleportFlavor: teleportFlavor,
		FIPS:           modules.IsBoringBinary(),
	}, nil

}

// EnvVarCDNBaseURL is the environment variable that allows users to override the Teleport base CDN url used in the installation script.
// Setting this value is required for testing (make production builds install from the dev CDN, and vice versa).
// As we (the Teleport company) don't distribute AGPL binaries, this must be set when using a Teleport OSS build.
// Example values:
// - "https://cdn.teleport.dev" (prod)
// - "https://cdn.cloud.gravitational.io" (dev builds/staging)
const EnvVarCDNBaseURL = "TELEPORT_CDN_BASE_URL"

func getCDNBaseURL() (string, error) {
	// If the user explicitly overrides the CDN base URL, we use it.
	if override := os.Getenv(EnvVarCDNBaseURL); override != "" {
		return override, nil
	}

	// If this is an AGPL build, we don't want to automatically install binaries distributed under a more restrictive
	// license so we error and ask the user set the CDN URL, either to:
	// - the official Teleport CDN if they agree with the community license and meet its requirements
	// - a custom CDN where they can store their own AGPL binaries
	if modules.GetModules().BuildType() == modules.BuildOSS {
		return "", trace.BadParameter(
			"This proxy licensed under AGPL but CDN binaries are licensed under the more restrictive Community license. "+
				"You can set TELEPORT_CDN_BASE_URL to a custom CDN, or to %q if you are OK with using the Community license.",
			teleportassets.CDNBaseURL())
	}

	return teleportassets.CDNBaseURL(), nil
}
