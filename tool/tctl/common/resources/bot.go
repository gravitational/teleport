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

package resources

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"

	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type botCollection struct {
	bots []*machineidv1pb.Bot
}

func (c *botCollection) Resources() []types.Resource {
	resources := make([]types.Resource, len(c.bots))
	for i, b := range c.bots {
		resources[i] = types.ProtoResource153ToLegacy(b)
	}
	return resources
}

func (c *botCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Roles"})
	for _, b := range c.bots {
		t.AddRow([]string{
			b.Metadata.Name,
			strings.Join(b.Spec.Roles, ", "),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func botHandler() Handler {
	return Handler{
		getHandler:    getBot,
		deleteHandler: deleteBot,
		createHandler: createBot,
		singleton:     false,
		mfaRequired:   true,
		description:   "Represents the identity of a machine or workload within Teleport.",
	}
}

func getBot(
	ctx context.Context,
	client *authclient.Client,
	ref services.Ref,
	opts GetOpts,
) (Collection, error) {
	c := client.BotServiceClient()
	if ref.Name != "" {
		bot, err := c.GetBot(ctx, &machineidv1pb.GetBotRequest{
			BotName: ref.Name,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &botCollection{bots: []*machineidv1pb.Bot{bot}}, nil
	}

	bots, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, limit int, token string) ([]*machineidv1pb.Bot, string, error) {
		resp, err := c.ListBots(ctx, &machineidv1pb.ListBotsRequest{
			PageSize:  int32(limit),
			PageToken: token,
		})

		return resp.GetBots(), resp.GetNextPageToken(), trace.Wrap(err)
	}))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &botCollection{bots: bots}, nil
}

func createBot(
	ctx context.Context,
	client *authclient.Client,
	raw services.UnknownResource,
	opts CreateOpts,
) error {
	bot := &machineidv1pb.Bot{}
	if err := (protojson.UnmarshalOptions{}).Unmarshal(raw.Raw, bot); err != nil {
		return trace.Wrap(err)
	}
	if opts.Force {
		_, err := client.BotServiceClient().UpsertBot(ctx, &machineidv1pb.UpsertBotRequest{
			Bot: bot,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("Bot %q has been created\n", bot.Metadata.Name)
		return nil
	}

	_, err := client.BotServiceClient().CreateBot(ctx, &machineidv1pb.CreateBotRequest{
		Bot: bot,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Bot %q has been created\n", bot.Metadata.Name)
	return nil
}

func deleteBot(
	ctx context.Context,
	client *authclient.Client,
	ref services.Ref,
) error {
	_, err := client.BotServiceClient().DeleteBot(ctx, &machineidv1pb.DeleteBotRequest{
		BotName: ref.Name,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Bot %q has been deleted\n", ref.Name)
	return nil
}
