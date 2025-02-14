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
	"strconv"
	"strings"

	"github.com/google/safetext/shsprintf"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/web/scripts/oneoff"
)

// AutoupdateStyle represents the kind of autoupdate mechanism the script should use.
type AutoupdateStyle int

const (
	// NoAutoupdate means the installed Teleport should not autoupdate.
	NoAutoupdate AutoupdateStyle = iota
	// PackageManagerAutoupdate means the installed Teleport should update via a script triggering package manager
	// updates. The script lives in the 'teleport-ent-update' package and was our original attempt at automatic updates.
	PackageManagerAutoupdate
	// UpdaterBinaryAutoupdate means the installed Teleport should update via the teleport-update binary.
	// This update style does not depend on any package manager (although it has a system dependency to wake up the
	// updater).
	UpdaterBinaryAutoupdate
)

// InstallScriptOptions contains the Teleport installation options used to generate installation scripts.
type InstallScriptOptions struct {
	AutoupdateStyle AutoupdateStyle
	// TeleportVersion that should be installed. Without the leading "v".
	TeleportVersion string
	// CDNBaseURL is the URL of the CDN hosting teleport tarballs.
	// If left empty, the 'teleport-update' installer will pick the one to use.
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

	if o.TeleportVersion == "" {
		return trace.BadParameter("Teleport version is required")
	}

	if o.TeleportFlavor == "" {
		return trace.BadParameter("Teleport flavor is required")
	}

	if o.CDNBaseURL != "" && !strings.HasPrefix(o.CDNBaseURL, "https://") {
		return trace.BadParameter("CDNBaseURL must start with 'https://'")
	}
	return nil
}

// oneOffParams returns the oneoff.OneOffScriptParams that will install Teleport
// using the oneoff.sh script to download and execute 'teleport-update'.
func (o *InstallScriptOptions) oneOffParams() (params oneoff.OneOffScriptParams) {
	// We add the leading v if it's not here
	version := o.TeleportVersion
	if o.TeleportVersion[0] != 'v' {
		version = "v" + o.TeleportVersion
	}

	args := []string{"enable", "--proxy", strconv.Quote(shsprintf.EscapeDefaultContext(o.ProxyAddr))}
	if o.CDNBaseURL != "" {
		args = append(args, "--base-url", strconv.Quote(shsprintf.EscapeDefaultContext(o.CDNBaseURL)))
	}

	return oneoff.OneOffScriptParams{
		TeleportBin:     "teleport-update",
		TeleportArgs:    strings.Join(args, " "),
		CDNBaseURL:      o.CDNBaseURL,
		TeleportVersion: version,
		TeleportFlavor:  o.TeleportFlavor,
		SuccessMessage:  "Teleport successfully installed.",
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

// getLegacyInstallScript returns the installation script that we have been serving at
// "https://cdn.teleport.dev/install.sh". This script installs teleport via package manager
// or by unpacking the tarball. Its usage should be phased out in favor of the updater-based
// installation script served by getUpdaterInstallScript.
func getLegacyInstallScript(ctx context.Context, opts InstallScriptOptions) (string, error) {
	return legacyInstallScript, nil
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
