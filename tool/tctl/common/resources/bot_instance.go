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

	"github.com/gravitational/trace"

	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
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
		rows = append(rows, []string{item.Spec.BotName, item.Spec.InstanceId})
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
		bi, err := c.GetBotInstance(ctx, &machineidv1pb.GetBotInstanceRequest{
			BotName:    ref.SubKind,
			InstanceId: ref.Name,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &botInstanceCollection{items: []*machineidv1pb.BotInstance{bi}}, nil
	}

	instances, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, limit int, pageToken string) ([]*machineidv1pb.BotInstance, string, error) {
		// TODO(nicholasmarais1158) Use ListBotInstancesV2 instead.
		//nolint:staticcheck // SA1019
		resp, err := c.ListBotInstances(ctx, &machineidv1pb.ListBotInstancesRequest{
			PageSize:  int32(limit),
			PageToken: pageToken,

			// Note: empty filter lists all bot instances
			FilterBotName: ref.Name,
		})

		return resp.GetBotInstances(), resp.GetNextPageToken(), trace.Wrap(err)
	}))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &botInstanceCollection{items: instances}, nil
}
