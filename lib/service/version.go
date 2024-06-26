/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package service

import (
	"errors"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

const (
	// majorVersionConstraint is the major version constraint when previous major version must be
	// present in the storage, if not - we must refuse to start.
	majorVersionConstraint = 18
)

var (
	// errMajorVersionUpgrade default error for restricting upgrade major version.
	errMajorVersionUpgrade = errors.New(`Teleport supports upgrade only one major version behind. ` +
		`See: https://goteleport.com/docs/upgrading/overview/#component-compatibility`)
)

// validateAndUpdateTeleportVersion validates that the major version persistent in the backend
// meets our upgrade compatibility guide.
func (process *TeleportProcess) validateAndUpdateTeleportVersion(currentVersion string) error {
	currentMajor, err := getMajorVersion(currentVersion)
	if err != nil {
		return trace.Wrap(err)
	}

	st, err := process.storage.GetState(process.GracefulExitContext(), types.RoleInstance)
	if trace.IsNotFound(err) {
		// If the Instance state has not been found, it means this is the first launch,
		// and we don't need to check the version since this is the initial launch.
		return nil
	} else if err != nil {
		return trace.Wrap(err)
	}

	if st.Spec.LocalVersion == "" {
		if currentMajor >= majorVersionConstraint {
			return trace.Wrap(errMajorVersionUpgrade, "Teleport is going to update to version %s, but before must be upgraded to %d.x.x first",
				currentVersion, currentMajor-1)
		}

		st.Spec.LocalVersion = currentVersion
		if err := process.storage.WriteState(types.RoleInstance, *st); err != nil {
			return trace.Wrap(err)
		}
		return nil
	}

	localMajor, err := getMajorVersion(st.Spec.LocalVersion)
	if err != nil {
		return trace.Wrap(err)
	}
	if currentMajor-localMajor > 1 {
		return trace.Wrap(errMajorVersionUpgrade, "Teleport version %s is going to upgrade to %s, but first you need to upgrade to version %d.x.x at least",
			st.Spec.LocalVersion, currentVersion, localMajor+1)
	}

	st.Spec.LocalVersion = currentVersion
	if err := process.storage.WriteState(types.RoleInstance, *st); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// getMajorVersion parses string to fetch major version number.
func getMajorVersion(version string) (int64, error) {
	v, err := semver.NewVersion(version)
	if err != nil {
		return 0, trace.Wrap(err, "cannot parse version: %q", version)
	}
	return v.Major, nil
}
