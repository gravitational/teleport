/*
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

package teleport

import (
	"github.com/coreos/go-semver/semver"

	"github.com/gravitational/teleport/api"
)

// Version is the version of Teleport in use.
const Version = api.Version

// SemVer returns the version of Teleport in use as a [semver.Version].
func SemVer() *semver.Version {
	return api.SemVer()
}

// MinClientSemVer returns the minimum client version required by the server.
// The [semver.Version.PreRelease] is set to "aa" so that the minimum client
// version comes before <version>-alpha so that alpha, beta, rc, and dev builds
// are permitted.
func MinClientSemVer() *semver.Version {
	return &semver.Version{Major: api.VersionMajor - 1, PreRelease: "aa"}
}

// Gitref is set to the output of "git describe" during the build process.
var Gitref string
