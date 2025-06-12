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

package resource

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"

	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

var bot = resource{
	getHandler:    getBot,
	createHandler: createBot,
	deleteHandler: deleteBot,
}

func createBot(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	bot := &machineidv1pb.Bot{}
	if err := (protojson.UnmarshalOptions{}).Unmarshal(raw.Raw, bot); err != nil {
		return trace.Wrap(err)
	}
	if opts.force {
		_, err := client.BotServiceClient().UpsertBot(ctx, &machineidv1pb.UpsertBotRequest{
			Bot: bot,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("bot %q has been created\n", bot.Metadata.Name)
		return nil
	}

	_, err := client.BotServiceClient().CreateBot(ctx, &machineidv1pb.CreateBotRequest{
		Bot: bot,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("bot %q has been created\n", bot.Metadata.Name)
	return nil
}

func getBot(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	remote := client.BotServiceClient()
	if ref.Name != "" {
		bot, err := remote.GetBot(ctx, &machineidv1pb.GetBotRequest{
			BotName: ref.Name,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return collections.NewBotCollection([]*machineidv1pb.Bot{bot}), nil
	}

	req := &machineidv1pb.ListBotsRequest{}
	var bots []*machineidv1pb.Bot
	for {
		resp, err := remote.ListBots(ctx, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		bots = append(bots, resp.Bots...)

		if resp.NextPageToken == "" {
			break
		}
		req.PageToken = resp.NextPageToken
	}
	return collections.NewBotCollection(bots), nil
}

func deleteBot(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if _, err := client.BotServiceClient().DeleteBot(ctx, &machineidv1pb.DeleteBotRequest{BotName: ref.Name}); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Bot %q has been deleted\n", ref.Name)
	return nil
}

var botInstance = resource{
	getHandler: getBotInstance,
}

func getBotInstance(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	if ref.Name != "" && ref.SubKind != "" {
		// Gets a specific bot instance, e.g. bot_instance/<bot name>/<instance id>
		bi, err := client.BotInstanceServiceClient().GetBotInstance(ctx, &machineidv1pb.GetBotInstanceRequest{
			BotName:    ref.SubKind,
			InstanceId: ref.Name,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return collections.NewBotInstanceCollection([]*machineidv1pb.BotInstance{bi}), nil
	}

	var instances []*machineidv1pb.BotInstance
	startKey := ""

	for {
		resp, err := client.BotInstanceServiceClient().ListBotInstances(ctx, &machineidv1pb.ListBotInstancesRequest{
			PageSize:  100,
			PageToken: startKey,

			// Note: empty filter lists all bot instances
			FilterBotName: ref.Name,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		instances = append(instances, resp.BotInstances...)

		if resp.NextPageToken == "" {
			break
		}

		startKey = resp.NextPageToken
	}

	return collections.NewBotInstanceCollection(instances), nil
}
