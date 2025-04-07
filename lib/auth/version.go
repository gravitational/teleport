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

	"github.com/gravitational/teleport/lib/services"
)

const (
	// skipVersionUpgradeCheckEnv is environment variable key for disabling the check
	// major version upgrade check.
	skipVersionUpgradeCheckEnv = "TELEPORT_UNSTABLE_SKIP_VERSION_UPGRADE_CHECK"
)

// validateAndUpdateTeleportVersion validates that the major version persistent in the backend
// meets our upgrade compatibility guide.
func validateAndUpdateTeleportVersion(
	ctx context.Context,
	procStorage VersionStorage,
	backendStorage services.AuthInfoService,
	currentVersion *semver.Version,
) error {
	if skip := os.Getenv(skipVersionUpgradeCheckEnv); skip != "" {
		return nil
	}

	lastKnownVersion, err := backendStorage.GetTeleportVersion(ctx)
	if trace.IsNotFound(err) {
		// TODO(vapopov): DELETE IN v19.0.0 last known version must be already migrated to backed storage.
		// Fallback to local process storage for backward compatibility with previous versions.
		lastKnownVersion, err = procStorage.GetTeleportVersion(ctx)
		if trace.IsNotFound(err) {
			if err := backendStorage.WriteTeleportVersion(ctx, currentVersion); err != nil {
				return trace.Wrap(err)
			}
			return nil
		} else if err != nil {
			return trace.Wrap(err)
		}

		// Preserve last known version from process storage to backed storage.
		if err := backendStorage.WriteTeleportVersion(ctx, lastKnownVersion); err != nil {
			return trace.Wrap(err)
		}
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

	if err := backendStorage.WriteTeleportVersion(ctx, currentVersion); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
