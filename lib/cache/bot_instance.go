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
	"slices"
	"strings"

	"github.com/gravitational/teleport/api/defaults"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"
)

type botInstanceIndex string

const (
	botInstanceNameIndex botInstanceIndex = "name"
)

func keyForNameIndex(botInstance *machineidv1.BotInstance) string {
	return makeNameIndexKey(
		botInstance.GetSpec().GetBotName(),
		botInstance.GetMetadata().GetName(),
	)
}

func makeNameIndexKey(botName string, instanceID string) string {
	return botName + "/" + instanceID
}

func newBotInstanceCollection(upstream services.BotInstance, w types.WatchKind) (*collection[*machineidv1.BotInstance, botInstanceIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter upstream (BotInstance)")
	}

	return &collection[*machineidv1.BotInstance, botInstanceIndex]{
		store: newStore(
			proto.CloneOf[*machineidv1.BotInstance],
			map[botInstanceIndex]func(*machineidv1.BotInstance) string{
				// Index on a combination of bot name and instance name
				botInstanceNameIndex: keyForNameIndex,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*machineidv1.BotInstance, error) {
			var resources []*machineidv1.BotInstance
			var nextToken string
			for {
				var err error
				var out []*machineidv1.BotInstance
				out, nextToken, err = upstream.ListBotInstances(ctx, "", 0, nextToken, "")
				if err != nil {
					return nil, trace.Wrap(err)
				}

				resources = append(resources, out...)

				if nextToken == "" {
					break
				}
			}
			return resources, nil
		},
		watch: w,
	}, nil
}

// GetBotInstance returns the specified BotInstance resource.
func (c *Cache) GetBotInstance(ctx context.Context, botName, instanceID string) (*machineidv1.BotInstance, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetBotInstance")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.botInstances)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		out, err := c.Config.BotInstanceService.GetBotInstance(ctx, botName, instanceID)
		return out, trace.Wrap(err)
	}

	key := makeNameIndexKey(botName, instanceID)

	out, err := rg.store.get(botInstanceNameIndex, key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return proto.CloneOf(out), nil
}

// ListBotInstances returns a page of BotInstance resources.
func (c *Cache) ListBotInstances(ctx context.Context, botName string, pageSize int, lastToken string, search string) ([]*machineidv1.BotInstance, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListBotInstances")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.botInstances)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		out, next, err := c.Config.BotInstanceService.ListBotInstances(ctx, botName, pageSize, lastToken, search)
		return out, next, trace.Wrap(err)
	}

	if pageSize <= 0 {
		pageSize = defaults.DefaultChunkSize
	}

	var out []*machineidv1.BotInstance
	for b := range rg.store.resources(botInstanceNameIndex, lastToken, "") {
		if len(out) == pageSize {
			return out, keyForNameIndex(b), nil
		}

		if matchBotInstance(b, botName, search) {
			out = append(out, proto.CloneOf(b))
		}
	}

	return out, "", nil
}

func matchBotInstance(b *machineidv1.BotInstance, botName string, search string) bool {
	// If updating this, ensure it's consistent with the upstream search logic in `lib/services/local/bot_instance.go`.

	if botName != "" && b.Spec.BotName != botName {
		return false
	}

	if search == "" {
		return true
	}

	latestHeartbeats := b.GetStatus().GetLatestHeartbeats()
	heartbeat := b.Status.InitialHeartbeat // Use initial heartbeat as a fallback
	if len(latestHeartbeats) > 0 {
		heartbeat = latestHeartbeats[len(latestHeartbeats)-1]
	}

	values := []string{
		b.Spec.BotName,
		b.Spec.InstanceId,
	}

	if heartbeat != nil {
		values = append(values, heartbeat.Hostname, heartbeat.JoinMethod, heartbeat.Version, "v"+heartbeat.Version)
	}

	return slices.ContainsFunc(values, func(val string) bool {
		return strings.Contains(strings.ToLower(val), strings.ToLower(search))
	})
}
