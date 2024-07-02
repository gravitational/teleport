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
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/gravitational/trace"
)

const (
	zypperPublicKeyEndpoint = "https://zypper.releases.teleport.dev/gpg"
	zypperRepoEndpoint      = "https://zypper.releases.teleport.dev/"
)

type packageManagerZypper struct {
	*packageManagerZypperConfig
}

type packageManagerZypperConfig struct {
	logger       *slog.Logger
	bins         binariesLocation
	fsRootPrefix string
}

func (p *packageManagerZypperConfig) checkAndSetDefaults() error {
	if p == nil {
		return trace.BadParameter("config is required")
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

// newPackageManagerZypper creates a new packageManagerZypper.
func newPackageManagerZypper(cfg *packageManagerZypperConfig) (*packageManagerZypper, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &packageManagerZypper{packageManagerZypperConfig: cfg}, nil
}

// AddTeleportRepository adds the Teleport repository to the current system.
func (pm *packageManagerZypper) AddTeleportRepository(ctx context.Context, linuxInfo *linuxDistroInfo, repoChannel string) error {
	// Teleport repo only targets the major version of the target distros.
	versionID := strings.Split(linuxInfo.VersionID, ".")[0]

	if linuxInfo.ID == "opensuse-tumbleweed" {
		versionID = "15" // tumbleweed uses dated VERSION_IDs like 20230702
	}

	pm.logger.InfoContext(ctx, "Trusting Teleport repository key", "command", "rpm --import "+zypperPublicKeyEndpoint)
	importPublicKeyCMD := exec.CommandContext(ctx, pm.bins.rpm, "--import", zypperPublicKeyEndpoint)
	importPublicKeyCMDOutput, err := importPublicKeyCMD.CombinedOutput()
	if err != nil {
		return trace.Wrap(err, string(importPublicKeyCMDOutput))
	}

	// Repo location looks like this:
	// https://yum.releases.teleport.dev/$ID/$VERSION_ID/Teleport/%{_arch}/{{ .RepoChannel }}/teleport.repo
	repoLocation := fmt.Sprintf("%s%s/%s/Teleport/%%{_arch}/%s/teleport.repo", zypperRepoEndpoint, linuxInfo.ID, versionID, repoChannel)
	pm.logger.InfoContext(ctx, "Building rpm metadata for Teleport repo", "command", "rpm --eval "+repoLocation)
	rpmEvalTeleportRepoCMD := exec.CommandContext(ctx, pm.bins.rpm, "--eval", repoLocation)
	rpmEvalTeleportRepoCMDOutput, err := rpmEvalTeleportRepoCMD.CombinedOutput()
	if err != nil {
		return trace.Wrap(err, string(rpmEvalTeleportRepoCMDOutput))
	}

	// output from the command above might have a `\n` at the end.
	repoURL := strings.TrimSpace(string(rpmEvalTeleportRepoCMDOutput))
	pm.logger.InfoContext(ctx, "Adding repository metadata", "command", "zypper --non-interactive addrepo "+repoURL)
	zypperAddRepoCMD := exec.CommandContext(ctx, pm.bins.zypper, "--non-interactive", "addrepo", repoURL)
	zypperAddRepoCMDOutput, err := zypperAddRepoCMD.CombinedOutput()
	if err != nil {
		return trace.Wrap(err, string(zypperAddRepoCMDOutput))
	}

	pm.logger.InfoContext(ctx, "Refresh public keys", "command", "zypper --gpg-auto-import-keys refresh")
	zypperRefreshKeysCMD := exec.CommandContext(ctx, pm.bins.zypper, "--gpg-auto-import-keys", "refresh")
	zypperRefreshKeysCMDOutput, err := zypperRefreshKeysCMD.CombinedOutput()
	if err != nil {
		return trace.Wrap(err, string(zypperRefreshKeysCMDOutput))
	}

	return nil
}

// InstallPackages installs one or multiple packages into the current system.
func (pm *packageManagerZypper) InstallPackages(ctx context.Context, packageList []packageVersion) error {
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
	installPackagesCMD := exec.CommandContext(ctx, pm.bins.zypper, installArgs...)
	installPackagesCMDOutput, err := installPackagesCMD.CombinedOutput()
	if err != nil {
		return trace.Wrap(err, string(installPackagesCMDOutput))
	}
	return nil
}
