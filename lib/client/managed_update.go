/*
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

package client

import (
	"context"
	"errors"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/autoupdate/tools/helper"
)

// CheckAndUpdateRemote verifies client tools version is set for update in cluster
// configuration by making the http request to `webapi/find` endpoint. The requested
// version is compared with the current client tools version. If they differ, the version
// package is downloaded, extracted to the client tools directory, and re-executed
// with the updated version.
// If $TELEPORT_HOME/bin contains downloaded client tools, it always re-executes
// using the version from the home directory.
func (tc *TeleportClient) CheckAndUpdateRemote(ctx context.Context, reExecArgs []string) error {
	updater, err := helper.NewDefaultUpdater()
	if errors.Is(err, helper.ErrDisabled) {
		slog.WarnContext(ctx, "Client tools update is disabled")
		return nil
	} else if err != nil {
		return trace.Wrap(err)
	}

	resp, err := updater.CheckRemote(ctx, tc.WebProxyAddr, tc.InsecureSkipVerify)
	if err != nil {
		return trace.Wrap(err)
	}

	if tc.ManagedUpdates == nil {
		tc.ManagedUpdates = &ManagedUpdates{Disabled: resp.Disabled}
	}

	if (tc.ManagedUpdates != nil && !tc.ManagedUpdates.Disabled) && resp.ReExec {
		err := helper.UpdateAndReExec(ctx, updater, resp.Version, reExecArgs)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}
