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
	_ "embed"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/google/safetext/shsprintf"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils/teleportassets"
	"github.com/gravitational/teleport/lib/web/scripts/oneoff"
)

// AutoupdateStyle represents the kind of autoupdate mechanism the script should use.
type AutoupdateStyle int

const (
	// NoAutoupdate means the installed Teleport should not autoupdate.
	NoAutoupdate AutoupdateStyle = iota
	// PackageManagerAutoupdate means the installed Teleport should update via a script triggering package manager
	// updates. The script lives in the 'teleport-ent-update' package and was our original attempt at automatic updates.
	// See RFD-109 for more details: https://github.com/gravitational/teleport/blob/master/rfd/0109-cloud-agent-upgrades.md
	PackageManagerAutoupdate
	// UpdaterBinaryAutoupdate means the installed Teleport should update via the teleport-update binary.
	// This update style does not depend on any package manager (although it has a system dependency to wake up the
	// updater).
	// See RFD-184 for more details: https://github.com/gravitational/teleport/blob/master/rfd/0184-agent-auto-updates.md
	UpdaterBinaryAutoupdate

	teleportUpdateDefaultCDN = teleportassets.TeleportReleaseCDN
)

func (style AutoupdateStyle) String() string {
	switch style {
	case PackageManagerAutoupdate:
		return "package"
	case UpdaterBinaryAutoupdate:
		return "binary"
	case NoAutoupdate:
		return "none"
	default:
		return "unknown"
	}
}

// InstallScriptOptions contains the Teleport installation options used to generate installation scripts.
type InstallScriptOptions struct {
	AutoupdateStyle AutoupdateStyle
	// TeleportVersion that should be installed. Without the leading "v".
	TeleportVersion *semver.Version
	// CDNBaseURL is the URL of the CDN hosting teleport tarballs.
	// If left empty, the 'teleport-update' installer will pick the one to use.
	// For example: "https://cdn.example.com"
	CDNBaseURL string
	// ProxyAddr is the address of the Teleport Proxy service that will be used
	// by the updater to fetch the desired version. Teleport Addrs are
	// 'hostname:port' (no scheme nor path).
	ProxyAddr string
	// TeleportFlavor is the name of the Teleport artifact fetched from the CDN.
	// Common values are "teleport" and "teleport-ent".
	TeleportFlavor string
	// FIPS represents if the installed Teleport version should use Teleport
	// binaries built for FIPS compliance.
	FIPS bool
	// Insecure disables TLS certificate verification on the teleport-update command.
	// This is meant for testing purposes.
	// This does not disable the TLS certificate verification when downloading
	// the artifacts from the CDN.
	// The agent install in insecure mode will not be able to automatically update.
	Insecure bool
}

// Check validates that the minimal options are set.
func (o *InstallScriptOptions) Check() error {
	switch o.AutoupdateStyle {
	case NoAutoupdate, PackageManagerAutoupdate:
		return nil
	case UpdaterBinaryAutoupdate:
		// We'll do the checks later.
	default:
		return trace.BadParameter("unsupported autoupdate style: %v", o.AutoupdateStyle)
	}
	if o.ProxyAddr == "" {
		return trace.BadParameter("Proxy address is required")
	}

	if o.TeleportVersion == nil {
		return trace.BadParameter("Teleport version is required")
	}

	if o.TeleportFlavor == "" {
		return trace.BadParameter("Teleport flavor is required")
	}

	if o.CDNBaseURL != "" {
		url, err := url.Parse(o.CDNBaseURL)
		if err != nil {
			return trace.Wrap(err, "failed to parse CDN base URL")
		}
		if url.Scheme != "https" {
			return trace.BadParameter("CDNBaseURL's scheme must be 'https://'")
		}
	}
	return nil
}

// oneOffParams returns the oneoff.OneOffScriptParams that will install Teleport
// using the oneoff.sh script to download and execute 'teleport-update'.
func (o *InstallScriptOptions) oneOffParams() (params oneoff.OneOffScriptParams) {
	args := []string{"enable", "--proxy", shsprintf.EscapeDefaultContext(o.ProxyAddr)}
	// Pass the base-url override if the base url is set and is not the default one.
	if o.CDNBaseURL != "" && o.CDNBaseURL != teleportUpdateDefaultCDN {
		args = append(args, "--base-url", shsprintf.EscapeDefaultContext(o.CDNBaseURL))
	}

	successMessage := "Teleport successfully installed."
	if o.Insecure {
		args = append(args, "--insecure")
		successMessage += " --insecure was used during installation, automatic updates will not work unless the Proxy Service presents a certificate trusted by the system."
	}

	return oneoff.OneOffScriptParams{
		Entrypoint:      "teleport-update",
		EntrypointArgs:  strings.Join(args, " "),
		CDNBaseURL:      o.CDNBaseURL,
		TeleportVersion: "v" + o.TeleportVersion.String(),
		TeleportFlavor:  o.TeleportFlavor,
		SuccessMessage:  successMessage,
		TeleportFIPS:    o.FIPS,
	}
}

// GetInstallScript returns a Teleport installation script.
// This script only installs Teleport, it does not start the agent, join it, nor configure its services.
// See the InstallNodeBashScript if you need a more complete setup.
func GetInstallScript(ctx context.Context, opts InstallScriptOptions) (string, error) {
	switch opts.AutoupdateStyle {
	case NoAutoupdate, PackageManagerAutoupdate:
		return getLegacyInstallScript(ctx, opts)
	case UpdaterBinaryAutoupdate:
		return getUpdaterInstallScript(ctx, opts)
	default:
		return "", trace.BadParameter("unsupported autoupdate style: %v", opts.AutoupdateStyle)
	}
}

//go:embed install/install.sh
var legacyInstallScript string

var (
	versionVar = regexp.MustCompile(`(?m)^TELEPORT_VERSION=""$`)
	suffixVar  = regexp.MustCompile(`(?m)^TELEPORT_SUFFIX=""$`)
	editionVar = regexp.MustCompile(`(?m)^TELEPORT_EDITION=""$`)
)

// getLegacyInstallScript returns the installation script that we have been serving at
// "https://cdn.teleport.dev/install.sh". This script installs teleport via package manager
// or by unpacking the tarball. Its usage should be phased out in favor of the updater-based
// installation script served by getUpdaterInstallScript.
func getLegacyInstallScript(ctx context.Context, opts InstallScriptOptions) (string, error) {
	tunedScript := versionVar.ReplaceAllString(legacyInstallScript, fmt.Sprintf(`TELEPORT_VERSION="%s"`, opts.TeleportVersion))
	if opts.TeleportFlavor == types.PackageNameEnt {
		tunedScript = suffixVar.ReplaceAllString(tunedScript, `TELEPORT_SUFFIX="-ent"`)
	}

	var edition string
	if opts.AutoupdateStyle == PackageManagerAutoupdate {
		edition = "cloud"
	} else if opts.TeleportFlavor == types.PackageNameEnt {
		edition = "enterprise"
	} else {
		edition = "oss"
	}
	tunedScript = editionVar.ReplaceAllString(tunedScript, fmt.Sprintf(`TELEPORT_EDITION="%s"`, edition))

	return tunedScript, nil
}

// getUpdaterInstallScript returns an installation script that downloads teleport-update
// and uses it to install a self-updating version of Teleport.
// This installation script is based on the oneoff.sh script and will become the standard
// way of installing Teleport.
func getUpdaterInstallScript(ctx context.Context, opts InstallScriptOptions) (string, error) {
	if err := opts.Check(); err != nil {
		return "", trace.Wrap(err, "invalid install script parameters")
	}

	scriptParams := opts.oneOffParams()

	return oneoff.BuildScript(scriptParams)
}
