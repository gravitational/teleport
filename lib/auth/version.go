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
	"log/slog"
	"os"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	authinfov1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/authinfo/v1"
	"github.com/gravitational/teleport/api/types/authinfo"
	"github.com/gravitational/teleport/lib/services"
)

const (
	// skipVersionUpgradeCheckEnv is environment variable key for disabling the check
	// major version upgrade check.
	skipVersionUpgradeCheckEnv = "TELEPORT_UNSTABLE_SKIP_VERSION_UPGRADE_CHECK"
	// versionUpgradeCheckMaxWriteRetry is the number of retries for conditional updates of the AuthInfo resource.
	versionUpgradeCheckMaxWriteRetry = 5
)

// validateAndUpdateTeleportVersion validates that the major version persistent in the backend
// meets our upgrade compatibility guide.
func validateAndUpdateTeleportVersion(
	ctx context.Context,
	procStorage VersionStorage,
	backendStorage services.AuthInfoService,
	currentVersion semver.Version,
) (err error) {
	if skip := os.Getenv(skipVersionUpgradeCheckEnv); skip != "" {
		return nil
	}

	for range versionUpgradeCheckMaxWriteRetry {
		authInfo, err := backendStorage.GetAuthInfo(ctx)
		if trace.IsNotFound(err) {
			// TODO(vapopov): DELETE IN v19.0.0 – the last known version should already be migrated to backend storage.
			// Fallback to local process storage for backward compatibility with previous versions.
			lastKnownVersion, err := procStorage.GetTeleportVersion(ctx)
			if trace.IsNotFound(err) {
				authInfo, err = authinfo.NewAuthInfo(&authinfov1.AuthInfoSpec{TeleportVersion: currentVersion.String()})
				if err != nil {
					return trace.Wrap(err)
				}
				if _, err := backendStorage.CreateAuthInfo(ctx, authInfo); err != nil {
					err = trace.Wrap(err)
					slog.WarnContext(ctx, "Failed to create AuthInfo resource",
						"error", err)
					continue
				}
				return nil
			} else if err != nil {
				return trace.Wrap(err)
			}

			authInfo, err = authinfo.NewAuthInfo(&authinfov1.AuthInfoSpec{TeleportVersion: lastKnownVersion.String()})
			if err != nil {
				return trace.Wrap(err)
			}
			if _, err := backendStorage.CreateAuthInfo(ctx, authInfo); err != nil {
				err = trace.Wrap(err)
				slog.WarnContext(ctx, "Failed to create AuthInfo resource",
					"error", err)
				continue
			}
		} else if err != nil {
			return trace.Wrap(err)
		}

		lastKnownVersion, err := semver.NewVersion(authInfo.GetSpec().GetTeleportVersion())
		if err != nil {
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

		authInfo.GetSpec().TeleportVersion = currentVersion.String()
		if _, err := backendStorage.UpdateAuthInfo(ctx, authInfo); err != nil {
			err = trace.Wrap(err)
			slog.WarnContext(ctx, "Failed to update AuthInfo resource",
				"error", err)
			continue
		}
	}

	return
}
