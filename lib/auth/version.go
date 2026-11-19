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
	"errors"
	"log/slog"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	backendinfov1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/backendinfo/v1"
	"github.com/gravitational/teleport/api/types/backendinfo"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

const (
	// versionUpgradeCheckMaxWriteRetry is the number of retries for conditional updates of the BackendInfo resource.
	versionUpgradeCheckMaxWriteRetry = 5
)

// validateAndUpdateTeleportVersion validates that the major version persistent in the backend
// meets our upgrade compatibility guide.
func validateAndUpdateTeleportVersion(
	ctx context.Context,
	backendStorage services.BackendInfoService,
	currentVersion semver.Version,
) error {
	var lastErr error
	for range versionUpgradeCheckMaxWriteRetry {
		var createNewResource bool
		backendInfo, err := backendStorage.GetBackendInfo(ctx)
		if trace.IsNotFound(err) {
			createNewResource = true
			backendInfo, err = backendinfo.NewBackendInfo(backendinfov1.BackendInfoSpec_builder{TeleportVersion: currentVersion.String()}.Build())
			if err != nil {
				return trace.Wrap(err)
			}
		} else if err != nil {
			return trace.Wrap(err)
		}

		lastKnownVersion, err := semver.NewVersion(backendInfo.GetSpec().GetTeleportVersion())
		if err != nil {
			return trace.Wrap(err, "failed to parse teleport version: %+q", backendInfo.GetSpec().GetTeleportVersion())
		}
		// The last known version is already updated to the current one.
		// Skip any further checks and resource updates.
		if !createNewResource && lastKnownVersion.Equal(currentVersion) {
			return nil
		}
		if currentVersion.Major-lastKnownVersion.Major > 1 {
			return trace.BadParameter("Unsupported upgrade path detected: from %v to %v. "+
				"Teleport supports direct upgrades to the next major version only.\n Please upgrade "+
				"your cluster to version %d.x.x first. See compatibility guarantees for details: "+
				"https://goteleport.com/docs/upgrading/overview/#component-compatibility.",
				lastKnownVersion, currentVersion.String(), lastKnownVersion.Major+1)
		}
		if lastKnownVersion.Major > currentVersion.Major {
			return trace.BadParameter("Unsupported downgrade path detected: from %v to %v. "+
				"Teleport doesn't support major version downgrades. Downgrading may lead to an inconsistent cluster state.\n  "+
				"Use at your own risk: to allow downgrading, set the environment variable `TELEPORT_UNSTABLE_SKIP_VERSION_UPGRADE_CHECK=yes`.\n  "+
				"See our upgrading guide for details: "+
				"https://goteleport.com/docs/upgrading/overview/#component-compatibility.",
				lastKnownVersion, currentVersion.String())
		}

		backendInfo.GetSpec().SetTeleportVersion(currentVersion.String())

		if createNewResource {
			_, err = backendStorage.CreateBackendInfo(ctx, backendInfo)
			if trace.IsAlreadyExists(err) {
				lastErr = trace.Wrap(err)
				slog.WarnContext(ctx, "Failed to create BackendInfo resource", "error", lastErr)
				continue
			} else if err != nil {
				return trace.Wrap(err)
			}
		} else {
			_, err = backendStorage.UpdateBackendInfo(ctx, backendInfo)
			if errors.Is(err, backend.ErrIncorrectRevision) || trace.IsNotFound(err) {
				lastErr = trace.Wrap(err)
				slog.WarnContext(ctx, "Failed to update BackendInfo resource", "error", lastErr)
				continue
			} else if err != nil {
				return trace.Wrap(err)
			}
		}

		return nil
	}
	return trace.Wrap(lastErr, "failed to persist Teleport version after %d attempts", versionUpgradeCheckMaxWriteRetry)
}

// upsertTeleportVersion overrides major version persistent in the backend.
func upsertTeleportVersion(
	ctx context.Context,
	backendStorage services.BackendInfoService,
	currentVersion semver.Version,
) error {
	backendInfo, err := backendinfo.NewBackendInfo(backendinfov1.BackendInfoSpec_builder{
		TeleportVersion: currentVersion.String(),
	}.Build())
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err = backendStorage.UpsertBackendInfo(ctx, backendInfo); err != nil {
		return trace.Wrap(err)
	}
	slog.WarnContext(ctx, "Version check skipped, Teleport might perform unsupported backend version transitions",
		"upgrade_version", currentVersion.String())

	return nil
}
