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
	"slices"
	"strings"
	"time"

	"github.com/gravitational/teleport/api/defaults"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"
)

type botInstanceIndex string

const (
	botInstanceNameIndex   botInstanceIndex = "name"
	botInstanceRecentIndex botInstanceIndex = "recent"
)

// TODO temporary, remove.
func timer(name string) func() {
	start := time.Now()
	return func() {
		fmt.Printf("%s took %v\n", name, time.Since(start))
	}
}

func calcKeyForNameIndex(botInstance *machineidv1.BotInstance) string {
	return fmt.Sprintf("%s/%s", botInstance.GetSpec().BotName, botInstance.GetMetadata().Name)
}

func calcKeyForRecentIndex(botInstance *machineidv1.BotInstance) string {
	// TODO Work out a value for instances that don't have a heartbeat
	// recordedAt := "~" // Tilde for ascending sort
	recordedAt := " " // Space for descending sort

	if len(botInstance.GetStatus().GetLatestHeartbeats()) > 0 {
		recordedAt = botInstance.GetStatus().GetLatestHeartbeats()[0].GetRecordedAt().AsTime().Format(time.RFC3339)
	}

	if botInstance.GetStatus().GetInitialHeartbeat().GetRecordedAt() != nil {
		recordedAt = botInstance.GetStatus().GetInitialHeartbeat().GetRecordedAt().AsTime().Format(time.RFC3339)
	}

	return fmt.Sprintf("%s/%s", recordedAt, botInstance.GetMetadata().Name)
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
				botInstanceNameIndex: calcKeyForNameIndex,
				// Index on the most recent heartbeat time
				botInstanceRecentIndex: calcKeyForRecentIndex,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*machineidv1.BotInstance, error) {
			fmt.Println("newBotInstanceCollection > fetcher")
			defer timer("newBotInstanceCollection > fetcher > finished:")()

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
		// TODO: is this needed?
		// headerTransform: func(hdr *types.ResourceHeader) *machineidv1.BotInstance {},
		watch: w,
	}, nil
}

// GetBotInstance returns the specified BotInstance resource.
func (c *Cache) GetBotInstance(ctx context.Context, botName, instanceID string) (*machineidv1.BotInstance, error) {
	fmt.Println("GetBotInstance")

	ctx, span := c.Tracer.Start(ctx, "cache/GetBotInstance")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.botInstances)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		fmt.Println("GetBotInstance > Cache not ready")
		out, err := c.Config.BotInstanceService.GetBotInstance(ctx, botName, instanceID)
		return out, trace.Wrap(err)
	}

	// TODO find a way to avoid hardcoding the key format
	key := fmt.Sprintf("%s/%s", botName, instanceID)

	out, err := rg.store.get(botInstanceNameIndex, key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return proto.CloneOf(out), nil
}

// ListBotInstances returns a page of BotInstance resources.
func (c *Cache) ListBotInstances(ctx context.Context, botName string, pageSize int, lastToken string, search string) ([]*machineidv1.BotInstance, string, error) {
	fmt.Println("ListBotInstances")

	ctx, span := c.Tracer.Start(ctx, "cache/ListBotInstances")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.botInstances)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		fmt.Println("ListBotInstances > Cache not ready")
		out, next, err := c.Config.BotInstanceService.ListBotInstances(ctx, botName, pageSize, lastToken, search)
		return out, next, trace.Wrap(err)
	}

	if pageSize <= 0 {
		pageSize = defaults.DefaultChunkSize
	}

	var out []*machineidv1.BotInstance
	nextToken := ""
	for b := range rg.store.cache.Descend(botInstanceRecentIndex, lastToken, "") {
		if len(out) == pageSize {
			nextToken = calcKeyForRecentIndex(b)
			break
		}

		if botName != "" && b.Spec.BotName != botName {
			continue
		}

		if search != "" {
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

			match := slices.ContainsFunc(values, func(val string) bool {
				return strings.Contains(strings.ToLower(val), strings.ToLower(search))
			})

			if !match {
				continue
			}
		}

		out = append(out, proto.CloneOf(b))
	}

	return out, nextToken, nil
}

// TODO How does the cache know about mutations to the underlying data? Does it re-compute index?

// // CreateBotInstance adds a new BotInstance resource.
// func (c *Cache) CreateBotInstance(ctx context.Context, botInstance *machineidv1.BotInstance) (*machineidv1.BotInstance, error) {
// 	return nil, nil
// }

// // DeleteBotInstance hard deletes the specified BotInstance resource.
// func (c *Cache) DeleteBotInstance(ctx context.Context, botName, instanceID string) error {
// 	return nil
// }

// // PatchBotInstance fetches an existing bot instance by bot name and ID,
// // then calls `updateFn` to apply any changes before persisting the
// // resource.
// func (c *Cache) PatchBotInstance(
// 	ctx context.Context,
// 	botName, instanceID string,
// 	updateFn func(*machineidv1.BotInstance) (*machineidv1.BotInstance, error),
// ) (*machineidv1.BotInstance, error) {
// 	return nil, nil
// }
