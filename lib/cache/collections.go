// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cache

import (
	"context"

	"github.com/gravitational/trace"

	autoupdatev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
)

// collectionHandler is used by the [Cache] to seed the initial
// data and process events for a particular resource.
type collectionHandler interface {
	// fetch fetches resources and returns a function which will apply said resources to the cache.
	// fetch *must* not mutate cache state outside of the apply function.
	// The provided cacheOK flag indicates whether this collection will be included in the cache generation that is
	// being prepared. If cacheOK is false, fetch shouldn't fetch any resources, but the apply function that it
	// returns must still delete resources from the backend.
	fetch(ctx context.Context, cacheOK bool) (apply func(ctx context.Context) error, err error)
	// onDelete will delete a single target resource from the cache. For
	// singletons, this is usually an alias to clear.
	onDelete(t types.Resource) error
	// onPut will update a single target resource from the cache
	onPut(t types.Resource) error
	// watchKind returns a watch
	// required for this collection
	watchKind() types.WatchKind
}

// collections is the group of resource [collection]s
// that the [Cache] supports.
type collections struct {
	byKind map[resourceKind]collectionHandler

	botInstances      *collection[*machineidv1.BotInstance, botInstanceIndex]
	remoteClusters    *collection[types.RemoteCluster, remoteClusterIndex]
	plugins           *collection[types.Plugin, pluginIndex]
	autoUpdateReports *collection[*autoupdatev1.AutoUpdateAgentReport, autoUpdateAgentReportIndex]
}

// isKnownUncollectedKind is true if a resource kind is not stored in
// the cache itself but it's only configured in the cache so that the
// resources events can be processed by downstream watchers.
func isKnownUncollectedKind(kind string) bool {
	switch kind {
	case types.KindAccessRequest, types.KindHeadlessAuthentication:
		return true
	default:
		return false
	}
}

// setupCollections ensures that the appropriate [collection] is
// initialized for all provided [types.WatcKind]s. An error is
// returned if a [types.WatchKind] has no associated [collection].
func setupCollections(c Config, legacyCollections map[resourceKind]legacyCollection) (*collections, error) {
	out := &collections{
		byKind: make(map[resourceKind]collectionHandler, 1),
	}

	for _, watch := range c.Watches {
		if isKnownUncollectedKind(watch.Kind) {
			continue
		}

		resourceKind := resourceKindFromWatchKind(watch)
		switch watch.Kind {
		case types.KindBotInstance:
			collect, err := newBotInstanceCollection(c.BotInstanceService, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.botInstances = collect
			out.byKind[resourceKind] = out.botInstances
		case types.KindAutoUpdateAgentReport:
			collect, err := newAutoUpdateAgentReportCollection(c.AutoUpdateService, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.autoUpdateReports = collect
			out.byKind[resourceKind] = out.autoUpdateReports
		case types.KindRemoteCluster:
			collect, err := newRemoteClusterCollection(c.Trust, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out.remoteClusters = collect
			out.byKind[resourceKind] = out.remoteClusters
		case types.KindPlugin:
			collect, err := newPluginsCollection(c.Plugin, watch)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			out.plugins = collect
			out.byKind[resourceKind] = out.plugins
		default:
			_, legacyOk := legacyCollections[resourceKind]
			if _, ok := out.byKind[resourceKind]; !ok && !legacyOk {
				return nil, trace.BadParameter("resource %q is not supported", watch.Kind)
			}
		}
	}

	return out, nil
}
