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

package auth

import (
	"context"
	"os"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
)

const (
	// majorVersionConstraint is the major version constraint when previous major version must be
	// present in the storage, if not - we must refuse to start.
	// TODO(vapopov): DELETE IN 18.0.0
	majorVersionConstraint = 18

	// skipVersionUpgradeCheckEnv is environment variable key for disabling the check
	// major version upgrade check.
	skipVersionUpgradeCheckEnv = "TELEPORT_UNSTABLE_SKIP_VERSION_UPGRADE_CHECK"
)

// validateAndUpdateTeleportVersion validates that the major version persistent in the backend
// meets our upgrade compatibility guide.
func validateAndUpdateTeleportVersion(
	ctx context.Context,
	storage VersionStorage,
	currentVersion *semver.Version,
	firstTimeStart bool,
) error {
	if skip := os.Getenv(skipVersionUpgradeCheckEnv); skip != "" {
		return nil
	}

	lastKnownVersion, err := storage.GetTeleportVersion(ctx)
	if trace.IsNotFound(err) {
		// When this is not the first start, we have to ensure that previous versions,
		// introduced before this check, were also verified. Therefore, not having a version
		// in the database means the last known version is <v17.
		if currentVersion.Major >= majorVersionConstraint && !firstTimeStart {
			return trace.BadParameter("Unsupported upgrade path detected: to %v. "+
				"Teleport supports direct upgrades to the next major version only.\n "+
				"For instance, if you have version 15.x.x, you must upgrade to version 16.x.x first. "+
				"See compatibility guarantees for details: "+
				"https://goteleport.com/docs/upgrading/overview/#component-compatibility.",
				currentVersion.String())
		}
		if err := storage.WriteTeleportVersion(ctx, currentVersion); err != nil {
			return trace.Wrap(err)
		}
		return nil
	} else if err != nil {
		return trace.Wrap(err)
	}

	if currentVersion.Major-lastKnownVersion.Major > 1 {
		return trace.BadParameter("Unsupported upgrade path detected: from %v to %v. "+
			"Teleport supports direct upgrades to the next major version only.\n Please upgrade "+
			"your cluster to version %d.x.x first. See compatibility guarantees for details: "+
			"https://goteleport.com/docs/upgrading/overview/#component-compatibility.",
			lastKnownVersion, currentVersion.String(), lastKnownVersion.Major+1)
	}
	if lastKnownVersion.Major-currentVersion.Major > 1 {
		return trace.BadParameter("Unsupported downgrade path detected: from %v to %v. "+
			"Teleport doesn't support major version downgrade.\n Please downgrade "+
			"your cluster to version %d.x.x first. See compatibility guarantees for details: "+
			"https://goteleport.com/docs/upgrading/overview/#component-compatibility.",
			lastKnownVersion, currentVersion.String(), lastKnownVersion.Major-1)
	}
	if err := storage.WriteTeleportVersion(ctx, currentVersion); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
