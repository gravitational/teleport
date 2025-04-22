/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package teleportassets

import (
	"fmt"

	"github.com/coreos/go-semver/semver"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/modules"
)

const (
	// TeleportReleaseCDN is the Teleport CDN URL for release builds.
	// This can be used to download the Teleport binary for release builds.
	TeleportReleaseCDN = "https://cdn.teleport.dev"
	// teleportPreReleaseCDN is the Teleport CDN URL for pre-release builds.
	// This can be used to download the Teleport binary for pre-release builds.
	teleportPreReleaseCDN = "https://cdn.cloud.gravitational.io"
)

// CDNBaseURL returns the URL of the CDN that can be used to download Teleport
// binary assets.
func CDNBaseURL() string {
	return cdnBaseURL(*teleport.SemVersion)
}

// cdnBaseURL returns the base URL of the CDN that can be used to download
// Teleport binary assets.
func cdnBaseURL(version semver.Version) string {
	if version.PreRelease != "" {
		return teleportPreReleaseCDN
	}
	return TeleportReleaseCDN
}

// CDNBaseURLForVersion returns the CDN base URL for a given artifact version.
// This function ensures that a Teleport production build cannot download from
// the pre-release CDN while Teleport pre-release builds can download both form
// the production and pre-release CDN.
func CDNBaseURLForVersion(artifactVersion *semver.Version) string {
	return cdnBaseURLForVersion(artifactVersion, teleport.SemVersion)
}

func cdnBaseURLForVersion(artifactVersion, teleportVersion *semver.Version) string {
	if teleportVersion.PreRelease != "" && artifactVersion.PreRelease != "" {
		return teleportPreReleaseCDN
	}
	return TeleportReleaseCDN
}

const (
	// teleportReleaseECR is the official release repo for Teleport images.
	teleportReleaseECR = "public.ecr.aws/gravitational"
	// teleportReleaseECR is the pre-release repo for Teleport images.
	teleportPreReleaseECR = "public.ecr.aws/gravitational-staging"
	// distrolessTeleportOSSImage is the distroless image of the OSS version of Teleport
	distrolessTeleportOSSImage = "teleport-distroless"
	// distrolessTeleportEntImage is the distroless image of the Enterprise version of Teleport
	distrolessTeleportEntImage = "teleport-ent-distroless"
)

// DistrolessImage returns the distroless teleport image repo.
func DistrolessImage(version semver.Version) string {
	repo := distrolessImageRepo(version)
	name := distrolessImageName(modules.GetModules().BuildType())
	return fmt.Sprintf("%s/%s:%s", repo, name, version)
}

func distrolessImageRepo(version semver.Version) string {
	if version.PreRelease != "" {
		return teleportPreReleaseECR
	}
	return teleportReleaseECR
}

func distrolessImageName(buildType string) string {
	if buildType == modules.BuildEnterprise {
		return distrolessTeleportEntImage
	}
	return distrolessTeleportOSSImage
}
