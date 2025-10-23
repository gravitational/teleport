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
	"log/slog"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// cmcCacheTTL is the cache TTL for the clusterMaintenanceConfig resource.
	// This cache is here to protect against accidental or intentional DDoS, the TTL must be low to quickly reflect
	// cluster configuration changes.
	cmcCacheTTL = time.Minute
)

type AutoUpdateAgentRolloutGetter interface {
	GetAutoUpdateAgentRollout(ctx context.Context) (*autoupdate.AutoUpdateAgentRollout, error)
}

type ClusterMaintenanceConfigGetter interface {
	GetClusterMaintenanceConfig(ctx context.Context) (types.ClusterMaintenanceConfig, error)
}

// Config is the Resolver configuration. All fields must be set.
type Config struct {
	RolloutGetter AutoUpdateAgentRolloutGetter
	CMCGetter     ClusterMaintenanceConfigGetter
	Channels      automaticupgrades.Channels
	Log           *slog.Logger
	Clock         clockwork.Clock
	Context       context.Context
}

// Resolver resolves which version should be ran, and if update should happen now.
type Resolver struct {
	cfg Config
	// cmcCache is used to cache the cluster maintenance config from the AUth Service.
	cmcCache *utils.FnCache
}

// NewResolver validates the Config and creates a Resolver.
func NewResolver(cfg Config) (*Resolver, error) {
	if cfg.RolloutGetter == nil {
		return nil, trace.BadParameter("missing autoupdate autoupdate rollout getter")
	}
	if cfg.CMCGetter == nil {
		return nil, trace.BadParameter("missing cluster maintenance config getter")
	}
	if cfg.Channels == nil {
		return nil, trace.BadParameter("missing autoupdate Channels")
	}
	if err := cfg.Channels.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.Log == nil {
		return nil, trace.BadParameter("missing logger")
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	// We create the cache after applying the options to make sure we use the fake clock if it was passed.
	cmcCache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:         cmcCacheTTL,
		Clock:       cfg.Clock,
		Context:     cfg.Context,
		ReloadOnErr: false,
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating cluster maintenance config cache")
	}

	return &Resolver{
		cfg:      cfg,
		cmcCache: cmcCache,
	}, nil
}

// GetVersion returns the version the agent should install/update to based on
// its group and updater UUID.
// If the cluster contains an autoupdate_agent_rollout resource from RFD184 it should take precedence.
// If the resource is not there, we fall back to RFD109-style updates with channels
// and maintenance window derived from the cluster_maintenance_config resource.
func (h *Resolver) GetVersion(ctx context.Context, group, updaterUUID string) (*semver.Version, error) {
	rollout, err := h.cfg.RolloutGetter.GetAutoUpdateAgentRollout(ctx)
	if err != nil {
		// Fallback to channels if there is no autoupdate_agent_rollout.
		if trace.IsNotFound(err) || trace.IsNotImplemented(err) {
			return getVersionFromChannel(ctx, h.cfg.Channels, group)
		}
		// Something is broken, we don't want to fallback to channels, this would be harmful.
		return nil, trace.Wrap(err, "getting autoupdate_agent_rollout")
	}

	return getVersionFromRollout(rollout, group, updaterUUID)
}

// ShouldUpdate returns if the agent should update now to based on its group
// and updater UUID.
// If the cluster contains an autoupdate_agent_rollout resource from RFD184 it should take precedence.
// If the resource is not there, we fall back to RFD109-style updates with channels
// and maintenance window derived from the cluster_maintenance_config resource.
func (h *Resolver) ShouldUpdate(ctx context.Context, group, updaterUUID string, windowLookup bool) (bool, error) {
	rollout, err := h.cfg.RolloutGetter.GetAutoUpdateAgentRollout(ctx)
	if err != nil {
		// Fallback to channels if there is no autoupdate_agent_rollout.
		if trace.IsNotFound(err) || trace.IsNotImplemented(err) {
			// Updaters using the RFD184 API are not aware of maintenance windows
			// like RFD109 updaters are. To have both updaters adopt the same behavior
			// we must do the CMC window lookup for them.
			if windowLookup {
				return h.getTriggerFromWindowThenChannel(ctx, group)
			}
			return getTriggerFromChannel(ctx, h.cfg.Channels, group)
		}
		// Something is broken, we don't want to fallback to channels, this would be harmful.
		return false, trace.Wrap(err, "failed to get auto-update rollout")
	}

	return getTriggerFromRollout(rollout, group, updaterUUID)
}
