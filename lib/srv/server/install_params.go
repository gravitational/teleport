/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package server

import (
	"context"

	"github.com/gravitational/trace"

	autoupdate "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
)

// autoUpdateRolloutGetter is an interface that allows checking whether managed updates are enabled.
type autoUpdateRolloutGetter interface {
	GetAutoUpdateAgentRollout(ctx context.Context) (*autoupdate.AutoUpdateAgentRollout, error)
}

// CheckInstallParamsManagedUpdates checks whether managed updates is required for the installer params.
// Specifically, if install_suffix or update_group is set in any of the installer params, then managed updates must be enabled.
func CheckInstallParamsManagedUpdates(ctx context.Context, params *types.InstallerParams, rolloutGetter autoUpdateRolloutGetter) error {
	if params == nil || (params.Suffix == "" && params.UpdateGroup == "") {
		return nil
	}

	_, err := rolloutGetter.GetAutoUpdateAgentRollout(ctx)
	switch {
	case err == nil:
		// Managed updates are enabled: install_suffix and update_group can be used
		return nil

	case trace.IsNotFound(err) || trace.IsNotImplemented(err):
		// Managed updates is not enabled: install_suffix and update_group cannot be used
		// If we are using an old version of teleport, it might not have the GetAutoUpdateAgentRollout method implemented.

		return trace.BadParameter("Managed updates are not enabled which is required to use install_suffix and update_group. " +
			"See https://goteleport.com/docs/upgrading/agent-managed-updates/ for more information on how to enable managed updates.")

	default:
		// An error occurred while checking the rollout state: return the error
		return trace.Wrap(err)
	}
}
