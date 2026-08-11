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
	"github.com/gravitational/teleport/lib/scopes"
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
		// Scoped bots are identified by their scope-qualified name; unscoped
		// bots by their bare name.
		name := b.GetMetadata().GetName()
		if scope := b.GetScope(); scope != "" {
			name = scopes.QualifiedName{Scope: scope, Name: name}.String()
		}
		t.AddRow([]string{
			name,
			strings.Join(b.GetSpec().GetRoles(), ", "),
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
		bot, err := c.GetBot(ctx, machineidv1pb.GetBotRequest_builder{
			BotName: ref.Name,
		}.Build())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &botCollection{bots: []*machineidv1pb.Bot{bot}}, nil
	}

	bots, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, limit int, token string) ([]*machineidv1pb.Bot, string, error) {
		resp, err := c.ListBots(ctx, machineidv1pb.ListBotsRequest_builder{
			PageSize:  int32(limit),
			PageToken: token,
		}.Build())

		return resp.GetBots(), resp.GetNextPageToken(), trace.Wrap(err)
	}))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &botCollection{bots: bots}, nil
}

// botScopedHandler returns a [ScopedHandler] for bots that are registered with
// a scope. Bots support both classic (unscoped) and scope-qualified access, so
// this is registered alongside the classic handler in ScopedHandlers().
// Create is absent because the classic handler takes precedence for 'tctl
// create' and already handles a scope on the bot resource.
func botScopedHandler() ScopedHandler {
	return ScopedHandler{
		getHandler:    getBotScoped,
		deleteHandler: deleteBotScoped,
		mfaRequired:   true,
		description:   "Represents the identity of a machine or workload within Teleport.",
	}
}

func getBotScoped(
	ctx context.Context,
	client *authclient.Client,
	subKind string,
	sqn *scopes.QualifiedName,
	opts GetOpts,
) (Collection, error) {
	if subKind != "" {
		return nil, rejectSubKind(types.KindBot, subKind)
	}
	if sqn == nil {
		// List-all is normally served by the classic handler, which bot is also
		// registered in; fall back to it for safety.
		return getBot(ctx, client, services.Ref{Kind: types.KindBot}, opts)
	}

	bot, err := client.BotServiceClient().GetBot(ctx, machineidv1pb.GetBotRequest_builder{
		BotName: sqn.Name,
		Scope:   sqn.Scope,
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &botCollection{bots: []*machineidv1pb.Bot{bot}}, nil
}

func deleteBotScoped(
	ctx context.Context,
	client *authclient.Client,
	subKind string,
	sqn scopes.QualifiedName,
) error {
	if subKind != "" {
		return rejectSubKind(types.KindBot, subKind)
	}

	_, err := client.BotServiceClient().DeleteBot(ctx, machineidv1pb.DeleteBotRequest_builder{
		BotName: sqn.Name,
		Scope:   sqn.Scope,
	}.Build())
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Bot %q has been deleted\n", sqn.String())
	return nil
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
	// String() renders the bare name when the scope is empty.
	displayName := scopes.QualifiedName{Scope: bot.GetScope(), Name: bot.GetMetadata().GetName()}.String()
	if opts.Force {
		_, err := client.BotServiceClient().UpsertBot(ctx, machineidv1pb.UpsertBotRequest_builder{
			Bot: bot,
		}.Build())
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("Bot %q has been created\n", displayName)
		return nil
	}

	_, err := client.BotServiceClient().CreateBot(ctx, machineidv1pb.CreateBotRequest_builder{
		Bot: bot,
	}.Build())
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Bot %q has been created\n", displayName)
	return nil
}

func deleteBot(
	ctx context.Context,
	client *authclient.Client,
	ref services.Ref,
) error {
	_, err := client.BotServiceClient().DeleteBot(ctx, machineidv1pb.DeleteBotRequest_builder{
		BotName: ref.Name,
	}.Build())
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Bot %q has been deleted\n", ref.Name)
	return nil
}
