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

package lookup

import (
	"context"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/utils"
)

// getVersionFromChannel gets the target version from the RFD109 channels.
func getVersionFromChannel(ctx context.Context, channels automaticupgrades.Channels, groupName string) (version *semver.Version, err error) {
	if channel, ok := channels[groupName]; ok {
		return channel.GetVersion(ctx)
	}
	return channels.DefaultVersion(ctx)
}

// getTriggerFromWindowThenChannel gets the target version from the RFD109 maintenance window and channels.
func (h *Resolver) getTriggerFromWindowThenChannel(ctx context.Context, groupName string) (bool, error) {
	// Caching the CMC for 60 seconds because this resource is cached neither by the auth nor the proxy.
	// And this function can be accessed via unauthenticated endpoints.
	cmc, err := utils.FnCacheGet(ctx, h.cmcCache, "cmc", func(ctx context.Context) (types.ClusterMaintenanceConfig, error) {
		return h.cfg.CMCGetter.GetClusterMaintenanceConfig(ctx)
	})

	// If there's no CMC or we failed to get it, we fallback directly to the channel
	if err != nil {
		if !trace.IsNotFound(err) {
			h.cfg.Log.WarnContext(ctx, "Failed to get cluster maintenance config", "error", err)
		}
		return getTriggerFromChannel(ctx, h.cfg.Channels, groupName)
	}

	// If we have a CMC, we check if the window is active, else we just check if the update is critical.
	if cmc.WithinUpgradeWindow(h.cfg.Clock.Now()) {
		return true, nil
	}
	return getTriggerFromChannel(ctx, h.cfg.Channels, groupName)
}

// getTriggerFromWindowThenChannel gets the target version from the RFD109 channels.
func getTriggerFromChannel(ctx context.Context, channels automaticupgrades.Channels, groupName string) (bool, error) {
	if channel, ok := channels[groupName]; ok {
		return channel.GetCritical(ctx)
	}
	defaultChannel, err := channels.DefaultChannel()
	if err != nil {
		return false, trace.Wrap(err, "creating new default channel")
	}
	return defaultChannel.GetCritical(ctx)
}
