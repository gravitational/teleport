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
	"encoding/base32"
	"fmt"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport/api/defaults"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/auth/machineid/machineidv1/expression"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/typical"
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
				botInstanceNameIndex: keyForBotInstanceNameIndex,
				// Index on a combination of most recent heartbeat time and instance name
				botInstanceActiveAtIndex: keyForBotInstanceActiveAtIndex,
				// Index on a combination of most recent heartbeat version and instance name
				botInstanceVersionIndex: keyForBotInstanceVersionIndex,
				// Index on a combination of most recent heartbeat hostname and instance name
				botInstanceHostnameIndex: keyForBotInstanceHostnameIndex,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*machineidv1.BotInstance, error) {
			out, err := stream.Collect(clientutils.Resources(ctx,
				func(ctx context.Context, limit int, start string) ([]*machineidv1.BotInstance, string, error) {
					return upstream.ListBotInstances(ctx, limit, start, nil)
				},
			))
			return out, trace.Wrap(err)
		},
		watch: w,
	}, nil
}

// GetBotInstance returns the specified BotInstance resource. A bot is
// identified by (botScope, botName): a lookup with the wrong scope misses,
// just as it would against the backend's scope-namespaced key ranges.
func (c *Cache) GetBotInstance(ctx context.Context, botScope, botName, instanceID string) (*machineidv1.BotInstance, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetBotInstance")
	defer span.End()

	getter := genericGetter[*machineidv1.BotInstance, botInstanceIndex]{
		cache:      c,
		collection: c.collections.botInstances,
		index:      botInstanceNameIndex,
		upstreamGet: func(ctx context.Context, _ string) (*machineidv1.BotInstance, error) {
			return c.Config.BotInstanceService.GetBotInstance(ctx, botScope, botName, instanceID)
		},
	}

	out, err := getter.get(ctx, makeBotInstanceNameIndexKey(botScope, botName, instanceID))
	return out, trace.Wrap(err)
}

// ListBotInstances returns a page of BotInstance resources.
// request *services.ListBotInstancesRequestOptions
func (c *Cache) ListBotInstances(ctx context.Context, pageSize int, lastToken string, options *services.ListBotInstancesRequestOptions) ([]*machineidv1.BotInstance, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListBotInstances")
	defer span.End()

	index := botInstanceNameIndex
	keyFn := keyForBotInstanceNameIndex
	isDesc := options.GetSortDesc()
	switch options.GetSortField() {
	case "bot_name":
		index = botInstanceNameIndex
		keyFn = keyForBotInstanceNameIndex
	case "active_at_latest":
		index = botInstanceActiveAtIndex
		keyFn = keyForBotInstanceActiveAtIndex
	case "version_latest":
		index = botInstanceVersionIndex
		keyFn = keyForBotInstanceVersionIndex
	case "host_name_latest":
		index = botInstanceHostnameIndex
		keyFn = keyForBotInstanceHostnameIndex
	case "":
		// default ordering as defined above
	default:
		return nil, "", trace.BadParameter("unsupported sort %q but expected bot_name, active_at_latest, version_latest or host_name_latest", options.GetSortField())
	}

	var exp typical.Expression[*expression.Environment, bool]
	if options.GetFilterQuery() != "" {
		parser, err := expression.NewBotInstanceExpressionParser()
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		exp, err = parser.Parse(options.GetFilterQuery())
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
	}

	lister := genericLister[*machineidv1.BotInstance, botInstanceIndex]{
		cache:           c,
		collection:      c.collections.botInstances,
		index:           index,
		isDesc:          isDesc,
		defaultPageSize: defaults.DefaultChunkSize,
		upstreamList: func(ctx context.Context, limit int, start string) ([]*machineidv1.BotInstance, string, error) {
			return c.Config.BotInstanceService.ListBotInstances(ctx, limit, start, options)
		},
		filter: func(b *machineidv1.BotInstance) bool {
			// A bot is identified by (scope, name), so any by-bot or by-scope
			// filter constrains the scope: name without scope means the
			// unscoped bot. Only the unfiltered listing spans all scopes.
			// This mirrors the backend's range routing in
			// local.BotInstanceService.ListBotInstances.
			if options.GetFilterBotName() != "" || options.GetFilterBotScope() != "" {
				if b.GetScope() != options.GetFilterBotScope() {
					return false
				}
			}
			if !services.MatchBotInstance(b, options.GetFilterBotName(), options.GetFilterSearchTerm(), exp) {
				return false
			}
			if fn := options.GetFilterFn(); fn != nil {
				return fn(b)
			}
			return true
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

func keyForBotInstanceNameIndex(botInstance *machineidv1.BotInstance) string {
	return makeBotInstanceNameIndexKey(
		botInstance.GetScope(),
		botInstance.GetSpec().GetBotName(),
		botInstance.GetMetadata().GetName(),
	)
}

// makeBotInstanceNameIndexKey builds the primary index key for a bot
// instance: <encoded scope>/<bot name>/<instance id>. The scope leads,
// mirroring the backend's scope-namespaced key layout: a lookup with the
// wrong scope misses structurally, and each scope's instances form a
// contiguous, bot-name-ordered block. Note that scoped blocks sort before
// the unscoped ("+") block here; the index order is cache-internal (page
// tokens cannot be resumed against the backend lister).
func makeBotInstanceNameIndexKey(botScope, botName, instanceID string) string {
	encodedScope, err := scopes.EncodeForKey(botScope)
	if err != nil {
		// An invalid scope cannot be encoded at lookup time either; falling
		// back to the raw scope keeps put/delete key derivation consistent
		// for such a resource rather than corrupting the store.
		encodedScope = botScope
	}
	return encodedScope + "/" + botName + "/" + instanceID
}

func keyForBotInstanceActiveAtIndex(botInstance *machineidv1.BotInstance) string {
	heartbeat := services.GetBotInstanceLatestHeartbeat(botInstance)
	recordedAt := heartbeat.GetRecordedAt().AsTime()
	return recordedAt.Format(time.RFC3339) + "/" + botInstance.GetMetadata().GetName()
}

// keyForBotInstanceVersionIndex produces a zero-padded version string for sorting. Pre-
// releases are sorted naively - 1.0.0-rc is correctly less than 1.0.0, but
// 1.0.0-rc.2 is more than 1.0.0-rc.11
func keyForBotInstanceVersionIndex(botInstance *machineidv1.BotInstance) string {
	version := "000000.000000.000000"
	heartbeat := services.GetBotInstanceLatestHeartbeat(botInstance)
	if heartbeat == nil {
		return version + "-~/" + botInstance.GetMetadata().GetName()
	}

	sv, err := semver.NewVersion(heartbeat.GetVersion())
	if err != nil {
		return version + "-~/" + botInstance.GetMetadata().GetName()
	}

	version = fmt.Sprintf("%06d.%06d.%06d", sv.Major, sv.Minor, sv.Patch)
	if sv.PreRelease != "" {
		version = version + "-" + string(sv.PreRelease)
	} else {
		version = version + "-~"
	}
	return version + "/" + botInstance.GetMetadata().GetName()
}

func keyForBotInstanceHostnameIndex(botInstance *machineidv1.BotInstance) string {
	hostname := "~"
	heartbeat := services.GetBotInstanceLatestHeartbeat(botInstance)
	if heartbeat != nil {
		hostname = heartbeat.GetHostname()
	}
	hostname = hostname + "/" + botInstance.GetMetadata().GetName()
	return base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(hostname))
}
