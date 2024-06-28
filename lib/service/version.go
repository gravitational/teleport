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
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// majorVersionConstraint is the major version constraint when previous major version must be
	// present in the storage, if not - we must refuse to start.
	// TODO(vapopov): DELETE IN 18.0.0
	majorVersionConstraint = 18
)

// validateAndUpdateTeleportVersion validates that the major version persistent in the backend
// meets our upgrade compatibility guide.
func (process *TeleportProcess) validateAndUpdateTeleportVersion(currentVersion string) error {
	currentMajor, err := utils.MajorVersion(currentVersion)
	if err != nil {
		return trace.Wrap(err)
	}

	localVersion, err := process.storage.GetTeleportVersion(process.GracefulExitContext())
	if trace.IsNotFound(err) {
		// We have to ensure that the process database is newly created before
		//applying the majorVersionConstraint check.
		_, err := process.storage.GetState(process.GracefulExitContext(), types.RoleAdmin)
		if !trace.IsNotFound(err) && err != nil {
			return trace.Wrap(err)
		}
		if currentMajor >= majorVersionConstraint && !trace.IsNotFound(err) {
			return trace.BadParameter("Unsupported upgrade path detected: to %v. "+
				"Teleport supports direct upgrades to the next major version only.\n "+
				"For instance, if you have version 15.x.x, you must upgrade to version 16.x.x first. "+
				"See compatibility guarantees for details: "+
				"https://goteleport.com/docs/upgrading/overview/#component-compatibility.",
				currentVersion)
		}
	} else if err != nil {
		return trace.Wrap(err)
	} else {
		localMajor, err := utils.MajorVersion(localVersion)
		if err != nil {
			return trace.Wrap(err)
		}
		if currentMajor-localMajor > 1 {
			return trace.BadParameter("Unsupported upgrade path detected: from %v to %v. "+
				"Teleport supports direct upgrades to the next major version only.\n Please upgrade "+
				"your cluster to version %d.x.x first. See compatibility guarantees for details: "+
				"https://goteleport.com/docs/upgrading/overview/#component-compatibility.",
				localVersion, currentVersion, localMajor+1)
		}
	}

	if err := process.storage.WriteTeleportVersion(currentVersion); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
