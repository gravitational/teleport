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
	"fmt"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport/api/defaults"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type botInstanceIndex string

const (
	botInstanceNameIndex     botInstanceIndex = "name"
	botInstanceActiveAtIndex botInstanceIndex = "active_at_latest"
	botInstanceVersionIndex  botInstanceIndex = "version_latest"
	botInstanceHostnameIndex botInstanceIndex = "host_name_latest"
)

func newBotInstanceCollection(upstream services.BotInstance, w types.WatchKind) (*collection[*machineidv1.BotInstance, botInstanceIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter upstream (BotInstance)")
	}

	return &collection[*machineidv1.BotInstance, botInstanceIndex]{
		store: newStore(
			types.KindBotInstance,
			proto.CloneOf[*machineidv1.BotInstance],
			map[botInstanceIndex]func(*machineidv1.BotInstance) string{
				// Index on a combination of bot name and instance name
				botInstanceNameIndex: keyForNameIndex,
				// Index on a combination of most recent heartbeat time and instance name
				botInstanceActiveAtIndex: keyForActiveAtIndex,
				// Index on a combination of most recent heartbeat version and instance name
				botInstanceVersionIndex: keyForVersionIndex,
				// Index on a combination of most recent heartbeat hostname and instance name
				botInstanceHostnameIndex: keyForHostnameIndex,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*machineidv1.BotInstance, error) {
			out, err := stream.Collect(clientutils.Resources(ctx,
				func(ctx context.Context, limit int, start string) ([]*machineidv1.BotInstance, string, error) {
					return upstream.ListBotInstances(ctx, "", limit, start, "", nil)
				},
			))
			return out, trace.Wrap(err)
		},
		watch: w,
	}, nil
}

// GetBotInstance returns the specified BotInstance resource.
func (c *Cache) GetBotInstance(ctx context.Context, botName, instanceID string) (*machineidv1.BotInstance, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetBotInstance")
	defer span.End()

	getter := genericGetter[*machineidv1.BotInstance, botInstanceIndex]{
		cache:      c,
		collection: c.collections.botInstances,
		index:      botInstanceNameIndex,
		upstreamGet: func(ctx context.Context, _ string) (*machineidv1.BotInstance, error) {
			return c.Config.BotInstanceService.GetBotInstance(ctx, botName, instanceID)
		},
	}

	out, err := getter.get(ctx, makeNameIndexKey(botName, instanceID))
	return out, trace.Wrap(err)
}

// ListBotInstances returns a page of BotInstance resources.
func (c *Cache) ListBotInstances(ctx context.Context, botName string, pageSize int, lastToken string, search string, sort *types.SortBy) ([]*machineidv1.BotInstance, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListBotInstances")
	defer span.End()

	index := botInstanceNameIndex
	keyFn := keyForNameIndex
	isDesc := options.GetSortDesc()
	switch options.GetSortField() {
	case "bot_name":
		index = botInstanceNameIndex
		keyFn = keyForNameIndex
	case "active_at_latest":
		index = botInstanceActiveAtIndex
		keyFn = keyForActiveAtIndex
	case "version_latest":
		index = botInstanceVersionIndex
		keyFn = keyForVersionIndex
	case "host_name_latest":
		index = botInstanceHostnameIndex
		keyFn = keyForHostnameIndex
	case "":
		// default ordering as defined above
	default:
		return nil, "", trace.BadParameter("unsupported sort %q but expected bot_name or active_at_latest", options.GetSortField())
	}

	lister := genericLister[*machineidv1.BotInstance, botInstanceIndex]{
		cache:           c,
		collection:      c.collections.botInstances,
		index:           index,
		isDesc:          isDesc,
		defaultPageSize: defaults.DefaultChunkSize,
		upstreamList: func(ctx context.Context, limit int, start string) ([]*machineidv1.BotInstance, string, error) {
			return c.Config.BotInstanceService.ListBotInstances(ctx, botName, limit, start, search, sort)
		},
		filter: func(b *machineidv1.BotInstance) bool {
			return services.MatchBotInstance(b, botName, search)
		},
		nextToken: func(b *machineidv1.BotInstance) string {
			return keyFn(b)
		},
	}
	out, next, err := lister.list(ctx,
		pageSize,
		lastToken,
	)
	return out, next, trace.Wrap(err)
}

func keyForNameIndex(botInstance *machineidv1.BotInstance) string {
	return makeNameIndexKey(
		botInstance.GetSpec().GetBotName(),
		botInstance.GetMetadata().GetName(),
	)
}

func makeNameIndexKey(botName string, instanceID string) string {
	return botName + "/" + instanceID
}

func keyForActiveAtIndex(botInstance *machineidv1.BotInstance) string {
	heartbeat := services.PickBotInstanceRecentHeartbeat(botInstance)
	recordedAt := heartbeat.GetRecordedAt().AsTime()
	return recordedAt.Format(time.RFC3339) + "/" + botInstance.GetMetadata().GetName()
}

func keyForVersionIndex(botInstance *machineidv1.BotInstance) string {
	version := "000000.000000.000000"
	heartbeat := services.PickBotInstanceRecentHeartbeat(botInstance)
	if heartbeat == nil {
		return version + "/" + botInstance.GetMetadata().GetName()
	}

	sv, err := semver.NewVersion(heartbeat.GetVersion())
	if err != nil {
		return version + "/" + botInstance.GetMetadata().GetName()
	}

	version = fmt.Sprintf("%06d.%06d.%06d", sv.Major, sv.Minor, sv.Patch)
	if sv.PreRelease != "" {
		version = version + "-" + string(sv.PreRelease)
	}
	if sv.Metadata != "" {
		version = version + "+" + sv.Metadata
	}
	return version + "/" + botInstance.GetMetadata().GetName()
}

func keyForHostnameIndex(botInstance *machineidv1.BotInstance) string {
	hostname := "~"
	heartbeat := services.PickBotInstanceRecentHeartbeat(botInstance)
	if heartbeat != nil {
		hostname = heartbeat.GetHostname()
	}
	return hostname + "/" + botInstance.GetMetadata().GetName()
}
