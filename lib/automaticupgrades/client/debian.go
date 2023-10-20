/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"

	"github.com/gravitational/trace"
)

// updateDebian updates the Teleport package on a debian system
func (updater *Updater) updateDebian(ctx context.Context) error {
	teleportPackage, err := installedAptPackage(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// The release codename is required to properly verify the teleport apt repository
	codename, err := getReleaseCodename()
	if err != nil {
		return trace.Wrap(err, "failed to get release codename")
	}

	// Expecting the teleport apt repository to be properly configured
	if err := verifyTeleportAptSourcesList(codename, updater.releaseChannel(teleportPackage)); err != nil {
		return trace.Wrap(err, "failed to verify teleport apt repository")
	}

	// These options ensure only the teleport apt repository is updated
	cmdUpdate := exec.CommandContext(ctx, "apt-get", "update",
		"-o", "Dir::Etc::SourceList=sources.list.d/teleport.list",
		"-o", "Dir::Etc::SourceParts=-",
		"-o", "APT::Get::List-Cleanup=0",
	)
	cmdUpdate.Stdout = updater.Stdout
	cmdUpdate.Stderr = updater.Stderr
	if err := cmdUpdate.Run(); err != nil {
		return trace.Wrap(err)
	}

	cmdInstall := exec.CommandContext(ctx, "apt-get", "install",
		"--yes",
		"--allow-downgrades",
		fmt.Sprintf("%s=%s", teleportPackage, updater.TeleportVersion))
	cmdInstall.Stdout = updater.Stdout
	cmdInstall.Stderr = updater.Stderr
	if err := cmdInstall.Run(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// getReleaseCodename returns the OS release codename
func getReleaseCodename() (string, error) {
	b, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "", trace.Wrap(err)
	}
	re, err := regexp.Compile(`VERSION_CODENAME=([a-z]*)`)
	if err != nil {
		return "", trace.Wrap(err)
	}
	result := re.FindStringSubmatch(string(b))
	if len(result) != 2 {
		return "", trace.NotFound("VERSION_CODENAME not found")
	}
	return result[1], nil
}

// verifyTeleportAptSourcesList verifies the Teleport apt repository is correctly configured
// Ex: `deb https://apt.releases.teleport.dev/ubuntu jammy stable/cloud`
func verifyTeleportAptSourcesList(codename, channel string) error {
	b, err := os.ReadFile(TeleportAptSourcesList)
	if err != nil {
		return trace.Wrap(err)
	}
	re, err := regexp.Compile(fmt.Sprintf("(?m)%s %s %s$", TeleportAptRepositoryURL, codename, channel))
	if err != nil {
		return trace.Wrap(err)
	}
	if !re.Match(b) {
		return trace.Errorf("unexpected contents in %s", TeleportAptSourcesList)
	}
	return nil
}

// installedAptPackage returns the teleport package installed via apt
func installedAptPackage(ctx context.Context) (string, error) {
	cmdStatus := exec.CommandContext(ctx, "dpkg", "--status", TeleportOSSPackage)
	if err := cmdStatus.Run(); err == nil {
		return TeleportOSSPackage, nil
	}

	cmdStatus = exec.CommandContext(ctx, "dpkg", "--status", TeleportEnterprisePackage)
	if err := cmdStatus.Run(); err == nil {
		return TeleportEnterprisePackage, nil
	}

	return "", trace.Errorf("unable to verify teleport package status; please ensure teleport is installed via the package manager")
}
