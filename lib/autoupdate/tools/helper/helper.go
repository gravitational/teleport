/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package helper

import (
	"context"
	"errors"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/lib/autoupdate/tools"
	"github.com/gravitational/teleport/lib/client"
)

func ManagedUpdateLocal(ctx context.Context, p *profile.Profile, reExecArgs []string) error {
	updater, err := tools.InitUpdater()
	if errors.Is(err, tools.ErrDisabled) {
		slog.WarnContext(ctx, "Client tools update is disabled", "error", err)
		return nil
	} else if err != nil {
		return trace.Wrap(err)
	}

	resp, err := updater.CheckLocal()
	if err != nil {
		return trace.Wrap(err)
	}

	if !resp.IsLocal && p != nil && p.ManagedUpdates != nil && p.ManagedUpdates.Disabled {
		return nil
	}

	if resp.ReExec {
		err := tools.UpdateAndReExec(ctx, updater, resp.Version, reExecArgs)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func ManagedUpdateRemote(ctx context.Context, tc *client.TeleportClient, reExecArgs []string) error {
	updater, err := tools.InitUpdater()
	if errors.Is(err, tools.ErrDisabled) {
		slog.WarnContext(ctx, "Client tools update is disabled", "error", err)
		return nil
	} else if err != nil {
		return trace.Wrap(err)
	}

	resp, err := updater.CheckRemote(ctx, tc.WebProxyAddr, tc.InsecureSkipVerify)
	if err != nil {
		return trace.Wrap(err)
	}

	if tc.ManagedUpdates == nil {
		tc.ManagedUpdates = &client.ManagedUpdates{Disabled: resp.Disabled}
	}

	if (tc.ManagedUpdates != nil && !tc.ManagedUpdates.Disabled) && resp.ReExec {
		err := tools.UpdateAndReExec(ctx, updater, resp.Version, reExecArgs)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}
