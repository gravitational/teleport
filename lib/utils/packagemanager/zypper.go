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

package packagemanager

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/linux"
)

// Zypper is a wrapper for apt package manager.
// This package manager is used in OpenSUSE/SLES and distros based on this distribution.
type Zypper struct {
	*ZypperConfig
}

// ZypperConfig contains the configurable fields for setting up the Zypper package manager.
type ZypperConfig struct {
	logger       *slog.Logger
	bins         BinariesLocation
	fsRootPrefix string
}

// CheckAndSetDefaults checks and sets default config values.
func (p *ZypperConfig) CheckAndSetDefaults() error {
	if p == nil {
		return trace.BadParameter("config is required")
	}

	p.bins.CheckAndSetDefaults()

	if p.fsRootPrefix == "" {
		p.fsRootPrefix = "/"
	}

	if p.logger == nil {
		p.logger = slog.Default()
	}

	return nil
}

// NewZypper creates a new Zypper package manager.
func NewZypper(cfg *ZypperConfig) (*Zypper, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Zypper{ZypperConfig: cfg}, nil
}

// AddTeleportRepository adds the Teleport repository to the current system.
func (pm *Zypper) AddTeleportRepository(ctx context.Context, linuxInfo *linux.OSRelease, repoChannel string, productionRepo bool) error {
	zypperRepoEndpoint, zypperRepoKeyEndpoint, err := repositoryEndpoint(productionRepo, zypper)
	if err != nil {
		return trace.Wrap(err)
	}

	// Teleport repo only targets the major version of the target distros.
	versionID := strings.Split(linuxInfo.VersionID, ".")[0]

	if linuxInfo.ID == "opensuse-tumbleweed" {
		versionID = "15" // tumbleweed uses dated VERSION_IDs like 20230702
	}

	pm.logger.InfoContext(ctx, "Trusting Teleport repository key", "command", "rpm --import "+zypperRepoKeyEndpoint)
	importPublicKeyCMD := exec.CommandContext(ctx, pm.bins.Rpm, "--import", zypperRepoKeyEndpoint)
	importPublicKeyCMDOutput, err := importPublicKeyCMD.CombinedOutput()
	if err != nil {
		return trace.Wrap(err, string(importPublicKeyCMDOutput))
	}

	// Repo location looks like this:
	// https://yum.releases.teleport.dev/$ID/$VERSION_ID/Teleport/%{_arch}/{{ .RepoChannel }}/teleport.repo
	repoLocation := fmt.Sprintf("%s%s/%s/Teleport/%%{_arch}/%s/teleport.repo", zypperRepoEndpoint, linuxInfo.ID, versionID, repoChannel)
	pm.logger.InfoContext(ctx, "Building rpm metadata for Teleport repo", "command", "rpm --eval "+repoLocation)
	rpmEvalTeleportRepoCMD := exec.CommandContext(ctx, pm.bins.Rpm, "--eval", repoLocation)
	rpmEvalTeleportRepoCMDOutput, err := rpmEvalTeleportRepoCMD.CombinedOutput()
	if err != nil {
		return trace.Wrap(err, string(rpmEvalTeleportRepoCMDOutput))
	}

	// output from the command above might have a `\n` at the end.
	repoURL := strings.TrimSpace(string(rpmEvalTeleportRepoCMDOutput))
	pm.logger.InfoContext(ctx, "Adding repository metadata", "command", "zypper --non-interactive addrepo "+repoURL)
	zypperAddRepoCMD := exec.CommandContext(ctx, pm.bins.Zypper, "--non-interactive", "addrepo", repoURL)
	zypperAddRepoCMDOutput, err := zypperAddRepoCMD.CombinedOutput()
	if err != nil {
		return trace.Wrap(err, string(zypperAddRepoCMDOutput))
	}

	pm.logger.InfoContext(ctx, "Refresh public keys", "command", "zypper --gpg-auto-import-keys refresh")
	zypperRefreshKeysCMD := exec.CommandContext(ctx, pm.bins.Zypper, "--gpg-auto-import-keys", "refresh")
	zypperRefreshKeysCMDOutput, err := zypperRefreshKeysCMD.CombinedOutput()
	if err != nil {
		return trace.Wrap(err, string(zypperRefreshKeysCMDOutput))
	}

	return nil
}

// InstallPackages installs one or multiple packages into the current system.
func (pm *Zypper) InstallPackages(ctx context.Context, packageList []PackageVersion) error {
	if len(packageList) == 0 {
		return nil
	}

	installArgs := make([]string, 0, len(packageList)+3)
	installArgs = append(installArgs, "--non-interactive", "install", "-y")

	for _, pv := range packageList {
		if pv.Version != "" {
			installArgs = append(installArgs, pv.Name+"-"+pv.Version)
			continue
		}
		installArgs = append(installArgs, pv.Name)
	}

	pm.logger.InfoContext(ctx, "Installing", "command", "zypper "+strings.Join(installArgs, " "))
	installPackagesCMD := exec.CommandContext(ctx, pm.bins.Zypper, installArgs...)
	installPackagesCMDOutput, err := installPackagesCMD.CombinedOutput()
	if err != nil {
		return trace.Wrap(err, string(installPackagesCMDOutput))
	}
	return nil
}
