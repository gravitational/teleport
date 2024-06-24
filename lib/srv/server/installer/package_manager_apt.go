// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package installer

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	productionAPTPublicKeyEndpoint = "https://apt.releases.teleport.dev/gpg"
	aptRepoEndpoint                = "https://apt.releases.teleport.dev/"

	aptTeleportSourceListFileRelative = "/etc/apt/sources.list.d/teleport.list"
	aptTeleportPublicKeyFileRelative  = "/usr/share/keyrings/teleport-archive-keyring.asc"
)

type packageManagerAPT struct {
	*packageManagerAPTConfig

	// legacy indicates that the old method of adding repos must be used.
	// This is used in Xenial (16.04) and Trusty (14.04) Ubuntu releases.
	legacy bool

	httpClient *http.Client
}

type packageManagerAPTConfig struct {
	logger               *slog.Logger
	aptPublicKeyEndpoint string
	fsRootPrefix         string
	bins                 binariesLocation
}

func (p *packageManagerAPTConfig) checkAndSetDefaults() error {
	if p == nil {
		return trace.BadParameter("config is required")
	}

	if p.aptPublicKeyEndpoint == "" {
		p.aptPublicKeyEndpoint = productionAPTPublicKeyEndpoint
	}

	p.bins.checkAndSetDefaults()

	if p.fsRootPrefix == "" {
		p.fsRootPrefix = "/"
	}

	if p.logger == nil {
		p.logger = slog.Default()
	}

	return nil
}

// newPackageManagerAPT creates a new packageManagerAPT.
func newPackageManagerAPT(cfg *packageManagerAPTConfig) (*packageManagerAPT, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	httpClient, err := defaults.HTTPClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &packageManagerAPT{packageManagerAPTConfig: cfg, httpClient: httpClient}, nil
}

// newPackageManagerAPTLegacy creates a new packageManagerAPT for legacy ubuntu versions (Xenial and Trusty).
func newPackageManagerAPTLegacy(cfg *packageManagerAPTConfig) (*packageManagerAPT, error) {
	pm, err := newPackageManagerAPT(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pm.legacy = true
	pm.logger = pm.logger.With("legacy", "true")
	return pm, nil
}

// AddTeleportRepository adds the Teleport repository to the current system.
func (pm *packageManagerAPT) AddTeleportRepository(ctx context.Context, linuxInfo *linuxDistroInfo, repoChannel string) error {
	pm.logger.InfoContext(ctx, "Fetching Teleport repository key", "endpoint", pm.aptPublicKeyEndpoint)

	resp, err := pm.httpClient.Get(pm.aptPublicKeyEndpoint)
	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.Body.Close()
	publicKey, err := utils.ReadAtMost(resp.Body, teleport.MaxHTTPResponseSize)
	if err != nil {
		return trace.Wrap(err)
	}

	aptTeleportSourceListFile := path.Join(pm.fsRootPrefix, aptTeleportSourceListFileRelative)
	aptTeleportPublicKeyFile := path.Join(pm.fsRootPrefix, aptTeleportPublicKeyFileRelative)
	// Format for teleport repo entry should look like this:
	// deb [signed-by=/usr/share/keyrings/teleport-archive-keyring.asc]  https://apt.releases.teleport.dev/${ID?} ${VERSION_CODENAME?} {{ .RepoChannel }}"
	teleportRepoMetadata := fmt.Sprintf("deb [signed-by=%s] %s%s %s %s", aptTeleportPublicKeyFile, aptRepoEndpoint, linuxInfo.ID, linuxInfo.VersionCodename, repoChannel)

	switch {
	case pm.legacy:
		pm.logger.InfoContext(ctx, "Trusting Teleport repository key", "command", "apt-key add -")
		aptKeyAddCMD := exec.CommandContext(ctx, pm.bins.aptKey, "add", "-")
		aptKeyAddCMD.Stdin = bytes.NewReader(publicKey)
		aptKeyAddCMDOutput, err := aptKeyAddCMD.CombinedOutput()
		if err != nil {
			return trace.Wrap(err, string(aptKeyAddCMDOutput))
		}
		teleportRepoMetadata = fmt.Sprintf("deb %s %s %s", aptRepoEndpoint, linuxInfo.VersionCodename, repoChannel)

	default:
		pm.logger.InfoContext(ctx, "Writing Teleport repository key", "destination", aptTeleportPublicKeyFile)
		if err := os.WriteFile(aptTeleportPublicKeyFile, publicKey, filePermsRepository); err != nil {
			return trace.Wrap(err)
		}
	}

	pm.logger.InfoContext(ctx, "Adding repository metadata", "apt_source_file", aptTeleportSourceListFile, "metadata", teleportRepoMetadata)
	if err := os.WriteFile(aptTeleportSourceListFile, []byte(teleportRepoMetadata), filePermsRepository); err != nil {
		return trace.Wrap(err)
	}

	pm.logger.InfoContext(ctx, "Updating apt sources", "command", "apt-get update")
	updateReposCMD := exec.CommandContext(ctx, pm.bins.aptGet, "update")
	updateReposCMDOutput, err := updateReposCMD.CombinedOutput()
	if err != nil {
		return trace.Wrap(err, string(updateReposCMDOutput))
	}
	return nil
}

// InstallPackages installs one or multiple packages into the current system.
func (pm *packageManagerAPT) InstallPackages(ctx context.Context, packageList []packageVersion) error {
	if len(packageList) == 0 {
		return nil
	}

	installArgs := make([]string, 0, len(packageList)+2)
	installArgs = append(installArgs, "install", "-y")

	for _, pv := range packageList {
		if pv.Version != "" {
			installArgs = append(installArgs, pv.Name+"="+pv.Version)
			continue
		}
		installArgs = append(installArgs, pv.Name)
	}

	pm.logger.InfoContext(ctx, "Installing", "command", "apt-get "+strings.Join(installArgs, " "))

	installPackagesCMD := exec.CommandContext(ctx, pm.bins.aptGet, installArgs...)
	installPackagesCMDOutput, err := installPackagesCMD.CombinedOutput()
	if err != nil {
		return trace.Wrap(err, string(installPackagesCMDOutput))
	}
	return nil
}
