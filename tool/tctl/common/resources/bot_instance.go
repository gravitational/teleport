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
// along with this program.  If not, see <http://www.gnu.org/licenses/>

package resources

import (
	"context"
	"io"
	"strings"

	"github.com/gravitational/trace"

	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
)

type botInstanceCollection struct {
	items []*machineidv1pb.BotInstance
}

func (c *botInstanceCollection) Resources() []types.Resource {
	r := make([]types.Resource, 0, len(c.items))
	for _, resource := range c.items {
		r = append(r, types.ProtoResource153ToLegacy(resource))
	}
	return r
}

func (c *botInstanceCollection) WriteText(w io.Writer, verbose bool) error {
	headers := []string{"Bot Name", "Instance ID"}

	// TODO: consider adding additional (possibly verbose) fields showing
	// last heartbeat, last auth, etc.
	var rows [][]string
	for _, item := range c.items {
		// Instances of scoped bots are identified by the bot's scope-qualified
		// name; instances of unscoped bots by the bot's bare name.
		botName := item.GetSpec().GetBotName()
		if scope := item.GetScope(); scope != "" {
			botName = scopes.QualifiedName{Scope: scope, Name: botName}.String()
		}
		rows = append(rows, []string{botName, item.GetSpec().GetInstanceId()})
	}

	t := asciitable.MakeTable(headers, rows...)

	// stable sort by name.
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func botInstanceHandler() Handler {
	return Handler{
		getHandler:  getBotInstance,
		singleton:   false,
		description: "A single instance of the tbot agent running in Teleport.",
	}
}

func getBotInstance(
	ctx context.Context,
	client *authclient.Client,
	ref services.Ref,
	opts GetOpts,
) (Collection, error) {
	c := client.BotInstanceServiceClient()
	if ref.Name != "" && ref.SubKind != "" {
		// Gets a specific bot instance, e.g. bot_instance/<bot name>/<instance id>
		bi, err := c.GetBotInstance(ctx, machineidv1pb.GetBotInstanceRequest_builder{
			BotName:    ref.SubKind,
			InstanceId: ref.Name,
		}.Build())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &botInstanceCollection{items: []*machineidv1pb.BotInstance{bi}}, nil
	}

	instances, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, limit int, pageToken string) ([]*machineidv1pb.BotInstance, string, error) {
		// TODO(nicholasmarais1158) Use ListBotInstancesV2 instead.
		//nolint:staticcheck // SA1019
		resp, err := c.ListBotInstances(ctx, machineidv1pb.ListBotInstancesRequest_builder{
			PageSize:  int32(limit),
			PageToken: pageToken,

			// Note: empty filter lists all bot instances
			FilterBotName: ref.Name,
		}.Build())

		return resp.GetBotInstances(), resp.GetNextPageToken(), trace.Wrap(err)
	}))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &botInstanceCollection{items: instances}, nil
}

// botInstanceScopedHandler returns a [ScopedHandler] for instances of scoped
// bots. Bot instances support both classic (unscoped) and scope-qualified
// access, so this is registered alongside the classic handler in
// ScopedHandlers().
func botInstanceScopedHandler() ScopedHandler {
	return ScopedHandler{
		getHandler:  getBotInstanceScoped,
		description: "A single instance of the tbot agent running in Teleport.",
	}
}

func getBotInstanceScoped(
	ctx context.Context,
	client *authclient.Client,
	subKind string,
	sqn *scopes.QualifiedName,
	opts GetOpts,
) (Collection, error) {
	if subKind != "" {
		// The generic rejectSubKind hint would suggest '<scope>::<subKind>',
		// which is not how bot instances are addressed, so use a
		// bot_instance-specific message.
		return nil, trace.BadParameter(
			"resource type %q does not support sub-kinds (got %q)\n"+
				"hint: address an instance of a scoped bot with a scope-qualified name:\n"+
				"  tctl get %s <scope>::<bot_name>/<instance_id>",
			types.KindBotInstance, subKind, types.KindBotInstance,
		)
	}
	if sqn == nil {
		// No SQN was provided, so this is a list-all. The classic handler
		// normally serves list-all (bot_instance is registered in both maps),
		// but fall back to it here for safety.
		return getBotInstance(ctx, client, services.Ref{Kind: types.KindBotInstance}, opts)
	}

	c := client.BotInstanceServiceClient()

	// The name component is either <bot_name>/<instance_id>, addressing a
	// single instance, or a bare <bot_name>, listing the scoped bot's
	// instances. The scope travels in the request: the server resolves it
	// against scope-qualified storage, so a wrong-scope read is a not-found
	// and no client-side scope comparison is needed.
	if botName, instanceID, ok := strings.Cut(sqn.Name, "/"); ok {
		bi, err := c.GetBotInstance(ctx, machineidv1pb.GetBotInstanceRequest_builder{
			BotName:    botName,
			InstanceId: instanceID,
			BotScope:   sqn.Scope,
		}.Build())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &botInstanceCollection{items: []*machineidv1pb.BotInstance{bi}}, nil
	}

	instances, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, limit int, pageToken string) ([]*machineidv1pb.BotInstance, string, error) {
		resp, err := c.ListBotInstancesV2(ctx, machineidv1pb.ListBotInstancesV2Request_builder{
			PageSize:  int32(limit),
			PageToken: pageToken,
			Filter: machineidv1pb.ListBotInstancesV2Request_Filters_builder{
				BotName:  sqn.Name,
				BotScope: sqn.Scope,
			}.Build(),
		}.Build())

		return resp.GetBotInstances(), resp.GetNextPageToken(), trace.Wrap(err)
	}))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &botInstanceCollection{items: instances}, nil
}
