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
	"fmt"
	"slices"

	"github.com/gravitational/trace"
)

// PackageVersion contains the package name and its version.
// Version can be empty.
type PackageVersion struct {
	Name    string
	Version string
}

const (
	apt    = "apt"
	yum    = "yum"
	zypper = "zypper"
)

var (
	supportedPackageManagers = []string{apt, yum, zypper}
)

// repositoryEndpoint returns the Teleport repository and public key endpoints for the package manager.
// The URL has a trailing slash.
// An error is returned when the package manager is not supported.
func repositoryEndpoint(productionRepo bool, packageManager string) (string, string, error) {
	if !slices.Contains(supportedPackageManagers, packageManager) {
		return "", "", trace.BadParameter("invalid package manager, only %v are supported", supportedPackageManagers)
	}

	releasesPath := "releases"
	if !productionRepo {
		releasesPath = "releases.development"
	}

	repositoryEndpoint := fmt.Sprintf("https://%s.%s.teleport.dev/", packageManager, releasesPath)

	return repositoryEndpoint, repositoryEndpoint + "gpg", nil
}
