package web

import (
	"fmt"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/utils/teleportassets"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"net/http"
	"os"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/web/scripts"
)

func (h *Handler) installScriptHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) (any, error) {
	// TODO: cache function
	const group, uuid = "", ""

	version, err := h.autoUpdateAgentVersion(r.Context(), group, uuid)
	if err != nil {
		h.logger.WarnContext(r.Context(), "Failed to update agent version", "error", err)
		version = teleport.Version
	}

	// if there's a rollout, we do new autoupdates

	_, rolloutErr := h.cfg.AccessPoint.GetAutoUpdateAgentRollout(r.Context())
	if rolloutErr != nil && !trace.IsNotFound(rolloutErr) {
		h.logger.WarnContext(r.Context(), "Failed to get rollout", "error", rolloutErr)
		return nil, trace.Wrap(err, "failed to check the autoupdate agent rollout state")
	}

	var autoupdateStyle scripts.AutoupdateStyle
	switch {
	case rolloutErr == nil:
		autoupdateStyle = scripts.UpdaterBinary
	case automaticUpgrades(h.clusterFeatures):
		autoupdateStyle = scripts.PackageManager
	default:
		autoupdateStyle = scripts.None
	}

	teleportFlavor := "teleport"
	switch modules.GetModules().BuildType() {
	case modules.BuildEnterprise:
		teleportFlavor = "teleport-ent"
	case modules.BuildOSS, modules.BuildCommunity:
		teleportFlavor = "teleport"
	default:
		h.logger.WarnContext(r.Context(), "Unknown built type, defaulting to the 'teleport' package.", "type", modules.GetModules().BuildType())
		teleportFlavor = "teleport"
	}

	cdnBaseURL, err := getCDNBaseURL()
	if err != nil {
		h.logger.WarnContext(r.Context(), "Failed to get CDN base URL", "error", err)
		return nil, trace.Wrap(err)
	}

	opts := scripts.InstallScriptOptions{
		AutoupdateStyle: autoupdateStyle,
		TeleportVersion: "v" + version,
		// Note: this override is required to configure the license on AGPL builds.
		// We cannot install random binaries if the user doesn't
		CDNBaseURL:     cdnBaseURL,
		ProdyAddr:      h.PublicProxyAddr(),
		TeleportFlavor: teleportFlavor,
		FIPS:           modules.IsBoringBinary(),
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

// EnvVarCDNBaseURL is the environment vairable that allows users to override the Teleport base CDN url used in the installation script.
// Setting this value is required for testing (make production builds install from the dev CDN, and vice versa).
// As we (the Teleport company) don't distribute AGPL binaries, this must be set when using a Teleport OSS build.
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
