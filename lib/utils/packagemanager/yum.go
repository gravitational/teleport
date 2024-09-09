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
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/linux"
)

const yumRepoEndpoint = "https://yum.releases.teleport.dev/"

var (
	// yumDistroMap maps distro IDs that teleport doesn't officially support but are known to work.
	// The key is the not-officially-supported distro ID and the value is the most similar distro.
	yumDistroMap = map[string]string{
		"rocky":     "rhel",
		"almalinux": "rhel",
	}
)

// YUM is a wrapper for yum package manager.
// This package manager is used in RedHat/AmazonLinux/Fedora/CentOS and othe distros.
type YUM struct {
	*YUMConfig
}

// YUMConfig contains the configurable fields for setting up the YUM package manager.
type YUMConfig struct {
	logger       *slog.Logger
	bins         BinariesLocation
	fsRootPrefix string
}

// CheckAndSetDefaults checks and sets default config values.
func (p *YUMConfig) CheckAndSetDefaults() error {
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

// NewYUM creates a new YUM package manager.
func NewYUM(cfg *YUMConfig) (*YUM, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &YUM{YUMConfig: cfg}, nil
}

// AddTeleportRepository adds the Teleport repository to the current system.
func (pm *YUM) AddTeleportRepository(ctx context.Context, linuxInfo *linux.OSRelease, repoChannel string) error {
	distroID := cmp.Or(yumDistroMap[linuxInfo.ID], linuxInfo.ID)

	// Teleport repo only targets the major version of the target distros.
	versionID := strings.Split(linuxInfo.VersionID, ".")[0]

	pm.logger.InfoContext(ctx, "Installing yum-utils", "command", "yum install -y yum-utils")
	installYumUtilsCMD := exec.CommandContext(ctx, pm.bins.Yum, "install", "-y", "yum-utils")
	installYumUtilsCMDOutput, err := installYumUtilsCMD.CombinedOutput()
	if err != nil {
		return trace.Wrap(err, string(installYumUtilsCMDOutput))
	}

	// Repo location looks like this:
	// https://yum.releases.teleport.dev/$ID/$VERSION_ID/Teleport/%{_arch}/{{ .RepoChannel }}/teleport.repo
	repoLocation := fmt.Sprintf("%s%s/%s/Teleport/%%{_arch}/%s/teleport.repo", yumRepoEndpoint, distroID, versionID, repoChannel)
	pm.logger.InfoContext(ctx, "Building rpm metadata for Teleport repo", "command", "rpm --eval "+repoLocation)
	rpmEvalTeleportRepoCMD := exec.CommandContext(ctx, pm.bins.Rpm, "--eval", repoLocation)
	rpmEvalTeleportRepoCMDOutput, err := rpmEvalTeleportRepoCMD.CombinedOutput()
	if err != nil {
		return trace.Wrap(err, string(rpmEvalTeleportRepoCMDOutput))
	}

	// output from the command above might have a `\n` at the end.
	repoURL := strings.TrimSpace(string(rpmEvalTeleportRepoCMDOutput))

	pm.logger.InfoContext(ctx, "Adding repository metadata", "command", "yum-config-manager --add-repo "+repoURL)
	yumAddRepoCMD := exec.CommandContext(ctx, pm.bins.YumConfigManager, "--add-repo", repoURL)
	yumAddRepoCMDOutput, err := yumAddRepoCMD.CombinedOutput()
	if err != nil {
		return trace.Wrap(err, string(yumAddRepoCMDOutput))
	}

	return nil
}

// InstallPackages installs one or multiple packages into the current system.
func (pm *YUM) InstallPackages(ctx context.Context, packageList []PackageVersion) error {
	if len(packageList) == 0 {
		return nil
	}

	installArgs := make([]string, 0, len(packageList)+2)
	installArgs = append(installArgs, "install", "-y")

	for _, pv := range packageList {
		if pv.Version != "" {
			installArgs = append(installArgs, pv.Name+"-"+pv.Version)
			continue
		}
		installArgs = append(installArgs, pv.Name)
	}

	pm.logger.InfoContext(ctx, "Installing", "command", "yum "+strings.Join(installArgs, " "))

	installPackagesCMD := exec.CommandContext(ctx, pm.bins.Yum, installArgs...)
	installPackagesCMDOutput, err := installPackagesCMD.CombinedOutput()
	if err != nil {
		return trace.Wrap(err, string(installPackagesCMDOutput))
	}
	return nil
}
