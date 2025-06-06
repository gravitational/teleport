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

func (rc *ResourceCommand) createBot(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	bot := &machineidv1pb.Bot{}
	if err := (protojson.UnmarshalOptions{}).Unmarshal(raw.Raw, bot); err != nil {
		return trace.Wrap(err)
	}
	if rc.IsForced() {
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

func (rc *ResourceCommand) getBot(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	remote := client.BotServiceClient()
	if rc.ref.Name != "" {
		bot, err := remote.GetBot(ctx, &machineidv1pb.GetBotRequest{
			BotName: rc.ref.Name,
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

func (rc *ResourceCommand) deleteBot(ctx context.Context, client *authclient.Client) error {
	if _, err := client.BotServiceClient().DeleteBot(ctx, &machineidv1pb.DeleteBotRequest{BotName: rc.ref.Name}); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Bot %q has been deleted\n", rc.ref.Name)
	return nil
}

func (rc *ResourceCommand) getBotInstance(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name != "" && rc.ref.SubKind != "" {
		// Gets a specific bot instance, e.g. bot_instance/<bot name>/<instance id>
		bi, err := client.BotInstanceServiceClient().GetBotInstance(ctx, &machineidv1pb.GetBotInstanceRequest{
			BotName:    rc.ref.SubKind,
			InstanceId: rc.ref.Name,
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
			FilterBotName: rc.ref.Name,
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
